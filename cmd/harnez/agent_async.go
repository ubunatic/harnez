package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/procs"
	"ubunatic.com/harnez/internal/subagent"
)

var agentExecutable = os.Executable
var foregroundDetachGrace = time.Second

const foregroundDetachTestOverrideEnv = "HARNEZ_TEST_FOREGROUND_DETACH"

func foregroundDetachEnabled() bool {
	if os.Getenv(foregroundDetachTestOverrideEnv) == "1" {
		return true
	}
	return flag.Lookup("test.v") == nil
}

func foregroundWorkerEnv() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key != agentRoleEnv {
			env = append(env, entry)
		}
	}
	return env
}

// timeoutSetByUser reports an explicit timeout (e.g. HTO=0), which disables
// the 60s foreground detach.
func timeoutSetByUser() bool {
	return os.Getenv(execTimeoutExplicitEnv) == "1" || os.Getenv(execTimeoutShortEnv) != "" || os.Getenv(execTimeoutEnv) != ""
}

func foregroundDetachTimeout(cmd *cobra.Command) time.Duration {
	if !foregroundDetachEnabled() {
		return 0
	}
	if cmd != nil && (cmd.Flags().Changed("timeout") || cmd.InheritedFlags().Changed("timeout")) {
		return 0
	}
	if timeoutSetByUser() {
		return 0
	}
	outerTimeout, err := time.ParseDuration(os.Getenv(execTimeoutEffectiveEnv))
	if err != nil || outerTimeout <= 0 {
		return 0
	}
	deadline := min(outerTimeout, 60*time.Second)
	if deadline <= foregroundDetachGrace {
		return 0
	}
	return deadline - foregroundDetachGrace
}

// sessionBackgroundHelp tells hosts how to run start/resume so the worker is
// their own tracked job (issue 630). It mirrors what `harnez agent selftest` teaches.
const sessionBackgroundHelp = `Run workers in your session background, not the machine background.
Session background: a job owned by your chat session; it shows in your task
list and its exit wakes you. Machine background (shell &, nohup, setsid):
invisible to you, so you could only find out by polling. No external event
system will notify you; your own background tool is the only signal.

One call per worker, with HTO=0 so the turn never detaches after 60s:
  Claude Code  Bash tool, run_in_background: true, command
               "HTO=0 harnez agent start ...". Its exit sends you a
               task-notification. Claude Code stops background jobs after
               30 minutes: size each worker turn to finish before that.
  AGY          run_command with "HTO=0 harnez agent start ..." and a small
               WaitMsBeforeAsync (e.g. 500); you are notified on completion.
  Codex        exec_command with "HTO=0 harnez agent start ..."; then call
               write_stdin on the returned session_id with chars "" until
               it reports an exit code.

Then tell the user the job id and the session name. Claude Code and AGY: end
your turn or do other work until notified. Codex: keep calling write_stdin
until the exit code. Never check status or logs in a loop while the job runs.
Use "harnez agent wait <session>" only to recover a lost job.`

// detachHost names the calling agent host so reattach guidance can use that
// host's own tool vocabulary; generic wording made Codex orphan detached
// agents (issue 647).
func detachHost() string {
	if os.Getenv("CODEX_THREAD_ID") != "" || os.Getenv("CODEX_CLI") != "" {
		return "codex"
	}
	return detectAgent("", nil)
}

// reattachSteps tells the host how to wait on `harnez agent wait <name>`
// with the tools it actually has.
func reattachSteps(host, name string) []string {
	wait := "harnez agent wait " + name
	switch host {
	case "codex":
		return []string{
			"Run `" + wait + "` with exec_command.",
			"It returns a session_id while the agent is still working: call write_stdin with that session_id and chars \"\" again and again until it reports an exit code.",
			"Do NOT end your turn, report completion, or start other work that depends on this agent before that exit code arrives.",
		}
	case "agy":
		return []string{
			"Run `" + wait + "` as a background task; Antigravity will automatically notify you when the task completes.",
			"Do NOT poll. Do NOT schedule timers or cron jobs. Do NOT block on the task; continue other work until notified.",
		}
	case "claude":
		return []string{
			"Run `" + wait + "` with the Bash tool and run_in_background: true (a host background job); Claude Code will automatically notify you when it exits.",
			"Do NOT poll. Do NOT schedule timers or cron jobs.",
		}
	}
	return []string{
		"Launch `" + wait + "` as a host background job; the environment will automatically notify this session when it finishes.",
		"Do NOT poll. Do NOT schedule timers or cron jobs.",
	}
}

// waitHint is the one-line pre-turn reminder of the reattach steps. When the
// turn cannot detach (e.g. HTO=0 in a session-background job) no separate wait
// job is needed, so the hint says that instead (issue 630).
func waitHint(name string) string {
	if timeoutSetByUser() {
		return "this turn does not detach; no separate wait job is needed"
	}
	return "if this turn detaches after 60s: " + strings.Join(reattachSteps(detachHost(), name), " ")
}

func writeDetachGuidance(w io.Writer, sess *subagent.Session, jsonOut bool) {
	steps := reattachSteps(detachHost(), sess.Name)
	const msg = "Agent turn exceeded 60s and has been cleanly detached to the background. The agent is STILL RUNNING; its result is not collected yet."
	if jsonOut {
		_ = json.NewEncoder(w).Encode(struct {
			Session      *subagent.Session `json:"session"`
			Status       string            `json:"status"`
			Detached     bool              `json:"detached"`
			Message      string            `json:"message"`
			Instructions []string          `json:"instructions"`
			WaitCommand  string            `json:"wait_command"`
		}{
			Session: sess, Status: "running", Detached: true,
			Message:      msg,
			Instructions: steps,
			WaitCommand:  "harnez agent wait " + sess.Name,
		})
		return
	}
	fmt.Fprintf(w, "[session info: id=%s name=%s status=running]\n", sess.ID, sess.Name)
	fmt.Fprintln(w, msg)
	for _, s := range steps {
		fmt.Fprintln(w, s)
	}
}

func launchForegroundWorker(cmd *cobra.Command, store *subagent.FileSessionStore, sess *subagent.Session, args []string, timeout time.Duration, jsonOut bool) (bool, error) {
	exe, err := agentExecutable()
	if err != nil {
		return false, err
	}
	stdout, err := os.OpenFile(sess.StdoutLog, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return false, err
	}
	defer stdout.Close()
	stderr, err := os.OpenFile(sess.StderrLog, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return false, err
	}
	defer stderr.Close()
	worker := exec.Command(exe, args...)
	worker.Env = foregroundWorkerEnv()
	worker.Stdin = nil
	worker.Stdout, worker.Stderr = stdout, stderr
	worker.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := worker.Start(); err != nil {
		return false, err
	}
	sess.Status = "running"
	sess.ProcessPID = worker.Process.Pid
	sess.ProcessStarttime = procs.ProcessStarttime(sess.ProcessPID)
	if err := store.Save(sess); err != nil {
		_ = worker.Process.Kill()
		_ = worker.Wait()
		return true, err
	}
	done := make(chan error, 1)
	go func() { done <- worker.Wait() }()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-timer.C:
		writeDetachGuidance(cmd.OutOrStdout(), sess, jsonOut)
		return true, nil
	case waitErr := <-done:
		if waitErr != nil {
			log, _ := os.ReadFile(sess.StderrLog)
			if len(bytes.TrimSpace(log)) > 0 {
				return true, fmt.Errorf("agent turn failed: %s", bytes.TrimSpace(log))
			}
			return true, fmt.Errorf("agent turn failed: %w", waitErr)
		}
		if jsonOut {
			output, err := os.ReadFile(sess.StdoutLog)
			if err != nil {
				return true, err
			}
			_, err = cmd.OutOrStdout().Write(output)
			return true, err
		}
		output, err := os.ReadFile(sess.StdoutLog)
		if err != nil {
			return true, err
		}
		var result agentOutput
		if err := json.Unmarshal(bytes.TrimSpace(output), &result); err != nil {
			return true, fmt.Errorf("decode completed agent turn: %w", err)
		}
		return true, writeAgentOutput(cmd, false, result)
	}
}

func launchDetachedWithPreflight(cmd *cobra.Command, req startRequest, storeDir, parentID string, check func(context.Context) error) error {
	modelSpec := req.ModelSpec
	if modelSpec == "" {
		var err error
		modelSpec, err = subagent.DefaultModelSpec()
		if err != nil {
			return err
		}
	}
	model, err := subagent.ResolveModel(modelSpec)
	if err != nil {
		return err
	}
	if err := rejectExhaustedQuota(modelSpec, model, req.AllowExhaustedQuota, providerAvailability); err != nil {
		return err
	}
	if err := preflightCodex(cmd.Context(), model.Provider, nil, check); err != nil {
		return fmt.Errorf("agent start preflight: %w", err)
	}
	role, err := startRole(req.Role, req.Prompt, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		return err
	}
	if req.Name == "" {
		sessions, err := store.List("", true)
		if err != nil {
			return err
		}
		taken := make(map[string]bool, len(sessions)*2)
		for _, existing := range sessions {
			taken[existing.Name] = true
			taken[existing.ID] = true
		}
		req.Name, err = subagent.GenerateSessionName(func(candidate string) bool { return taken[candidate] })
		if err != nil {
			return err
		}
	}
	if existing, err := store.Find(req.Name); err == nil {
		return fmt.Errorf("session name %q is already in use by %s", req.Name, existing.ID)
	}
	dir, err := filepath.Abs(req.Dir)
	if err != nil {
		return err
	}
	id := uuid.NewString()
	stdoutPath := filepath.Join(storeDir, id+".stdout.log")
	stderrPath := filepath.Join(storeDir, id+".stderr.log")
	now := time.Now()
	sess := &subagent.Session{ID: id, Name: req.Name, StartPrompt: req.StoredPrompt, Provider: model.Provider, Model: model.Name, Tier: model.Tier, WorkingDir: dir, ParentSessionID: parentID, CallerPID: os.Getpid(), HarnessType: "harnez", Status: "running", Role: role, CreatedAt: now, LastActiveAt: now, StdoutLog: stdoutPath, StderrLog: stderrPath}
	if err := store.Create(sess); err != nil {
		return err
	}
	exe, err := agentExecutable()
	if err != nil {
		sess.Status = "failed"
		sess.LastError = err.Error()
		_ = store.Save(sess)
		return err
	}
	args := []string{"--store-dir", storeDir, "agent", "start", "--worker-session", id, "--json", "--model", modelSpec, "--role", role, "--name", req.Name, "--dir", dir, "--plan", map[bool]string{true: "yes", false: "no"}[req.PlanFirst]}
	if req.AllowExhaustedQuota {
		args = append(args, "--allow-exhausted-quota")
	}
	args = append(args, "--", req.Prompt)
	worker := exec.Command(exe, args...)
	worker.Env = os.Environ()
	worker.Stdin = nil
	stdout, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		sess.Status = "failed"
		sess.LastError = err.Error()
		_ = store.Save(sess)
		return err
	}
	defer stdout.Close()
	stderr, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		sess.Status = "failed"
		sess.LastError = err.Error()
		_ = store.Save(sess)
		return err
	}
	defer stderr.Close()
	worker.Stdout, worker.Stderr = stdout, stderr
	if err := worker.Start(); err != nil {
		sess.Status = "failed"
		sess.LastError = err.Error()
		_ = store.Save(sess)
		return err
	}
	go func() { _ = worker.Wait() }()
	sess.ProcessPID = worker.Process.Pid
	sess.ProcessStarttime = procs.ProcessStarttime(sess.ProcessPID)
	if err := store.Save(sess); err != nil {
		_ = worker.Process.Kill()
		_ = worker.Wait()
		return err
	}
	if req.JSON {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(sess)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Started agent %s (%s)\n", sess.Name, sess.ID)
	return nil
}

func runDetachedWorker(cmd *cobra.Command, req startRequest, storeDir string) error {
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		return err
	}
	var sess *subagent.Session
	for i := 0; i < 100; i++ {
		sess, err = store.Get(req.SessionID)
		if err != nil {
			return err
		}
		if sess.ProcessPID == os.Getpid() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if sess == nil || sess.ProcessPID != os.Getpid() {
		return fmt.Errorf("detached worker session %q was not registered", req.SessionID)
	}
	req.StoredPrompt = sess.StartPrompt
	err = runStart(cmd, agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return sess.ParentSessionID }, preflight: subagent.CheckCodexAuth, dbPath: agentUsageDBPath()}, req)
	current, getErr := store.Get(sess.ID)
	if getErr != nil {
		return getErr
	}
	current.ProcessPID = 0
	current.LastActiveAt = time.Now()
	if err != nil {
		current.Status = "failed"
		current.LastError = err.Error()
	} else {
		current.Status = "completed"
	}
	if saveErr := store.Save(current); saveErr != nil {
		return saveErr
	}
	return err
}

func runDetachedResumeWorker(cmd *cobra.Command, sessionID string, req resumeRequest, storeDir string) error {
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		return err
	}
	var sess *subagent.Session
	for i := 0; i < 100; i++ {
		sess, err = store.Get(sessionID)
		if err != nil {
			return err
		}
		if sess.ProcessPID == os.Getpid() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if sess == nil || sess.ProcessPID != os.Getpid() {
		return fmt.Errorf("detached resume session %q was not registered", sessionID)
	}
	req.SessionID = sessionID
	req.Name = sess.Name
	req.JSON = true
	deps := agentDeps{
		store:    func() (*subagent.FileSessionStore, error) { return store, nil },
		storeDir: storeDir,
		dbPath:   agentUsageDBPath(),
		parent:   func() string { return sess.ParentSessionID },
		find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
			return s.Find(id)
		},
		preflight: subagent.CheckCodexAuth,
	}
	err = runResume(cmd, deps, req)
	current, getErr := store.Get(sessionID)
	if getErr != nil {
		return getErr
	}
	current.ProcessPID = 0
	current.LastActiveAt = time.Now()
	if err != nil {
		current.Status = "failed"
		current.LastError = err.Error()
	} else {
		current.Status = "completed"
	}
	if saveErr := store.Save(current); saveErr != nil {
		return saveErr
	}
	return err
}

func waitForAgent(ctx context.Context, store *subagent.FileSessionStore, identifier string, timeout time.Duration) (*subagent.Session, error) {
	if timeout < 0 {
		return nil, fmt.Errorf("timeout must not be negative")
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		sess, err := store.Find(identifier)
		if err != nil {
			return nil, err
		}
		writers, err := sessionWriters(sess)
		if err != nil {
			return nil, fmt.Errorf("check session writer: %w", err)
		}
		wrapperExists := sess.ProcessPID > 0 && processExists(sess.ProcessPID)
		if len(writers) == 0 && !wrapperExists {
			// The worker may publish its final result while /proc is scanned.
			// Re-read before returning or replacing a running record as stale.
			current, err := store.Get(sess.ID)
			if err != nil {
				return nil, err
			}
			if current.Status != sess.Status || current.ProcessPID != sess.ProcessPID || current.ProviderPID != sess.ProviderPID {
				continue
			}
		}
		if sess.Status != "running" && len(writers) == 0 && !wrapperExists {
			return sess, nil
		}
		if sess.Status == "running" && sess.ProcessPID > 0 && len(writers) == 0 && !wrapperExists {
			sess.Status = "failed"
			sess.ProcessPID = 0
			sess.LastActiveAt = time.Now()
			sess.LastError = "detached worker exited without recording a terminal status"
			if err := store.Save(sess); err != nil {
				return nil, err
			}
			return sess, nil
		}
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return sess, nil
			}
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

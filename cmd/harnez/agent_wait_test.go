package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/subagent"
)

func TestAgentWaitTimeoutAndCompletion(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "wait-session", Name: "worker", Status: "running", HarnessType: "harnez", ProcessPID: os.Getpid()}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--store-dir", storeDir, "wait", "--timeout", "20ms", "--json", "wait-session"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"status":"running"`) {
		t.Fatalf("timeout output = %s", out.String())
	}
	sess.Status = "completed"
	sess.Response = "finished"
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	cmd = newAgentCmd()
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--store-dir", storeDir, "wait", "--timeout", "1s", "--json", "wait-session"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"response":"finished"`) {
		t.Fatalf("completion output = %s", out.String())
	}
}

func TestAgentWaitConcurrentCompletions(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"wait-one", "wait-two"} {
		if err := store.Save(&subagent.Session{ID: id, Name: id, Status: "running"}); err != nil {
			t.Fatal(err)
		}
	}
	results := make(chan *subagent.Session, 2)
	errs := make(chan error, 2)
	for _, id := range []string{"wait-one", "wait-two"} {
		go func(id string) {
			sess, err := waitForAgent(context.Background(), store, id, time.Second)
			results <- sess
			errs <- err
		}(id)
	}
	for _, id := range []string{"wait-two", "wait-one"} {
		sess, err := store.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		sess.Status = "completed"
		sess.Response = id
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	got := map[string]bool{}
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
		sess := <-results
		if sess == nil || sess.Status != "completed" || sess.Response != sess.ID {
			t.Fatalf("wait result = %#v", sess)
		}
		got[sess.ID] = true
	}
	if len(got) != 2 {
		t.Fatalf("completed sessions = %#v", got)
	}
}

func TestAgentWaitCancellation(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "cancel-wait", Name: "cancel-wait", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := waitForAgent(ctx, store, "cancel-wait", 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait error = %v, want context.Canceled", err)
	}
}

func TestAgentWaitFailedWorkerIsNotOrphaned(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worker := exec.Command(os.Args[0], "-test.run=^TestWaitWorkerCrashHelper$")
	worker.Env = append(os.Environ(), "HARNEZ_WAIT_CRASH_HELPER=1")
	if err := worker.Start(); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "crashed-worker", Name: "crashed-worker", Status: "running", ProcessPID: worker.Process.Pid}); err != nil {
		_ = worker.Process.Kill()
		t.Fatal(err)
	}
	if err := worker.Wait(); err != nil {
		t.Fatalf("crash helper: %v", err)
	}
	sess, err := waitForAgent(context.Background(), store, "crashed-worker", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Status != "failed" || sess.ProcessPID != 0 {
		t.Fatalf("crashed worker session = %#v", sess)
	}
}

func TestWaitWorkerCrashHelper(t *testing.T) {
	if os.Getenv("HARNEZ_WAIT_CRASH_HELPER") == "1" {
		os.Exit(0)
	}
}

func TestAgentDetachedSpawnAndWait(t *testing.T) {
	installUnclassifiedNeus(t)
	storeDir := t.TempDir()
	helperBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(t.TempDir(), "worker-launcher")
	script := "#!/bin/sh\nstore=''\nid=''\nwhile test $# -gt 0; do\n case \"$1\" in\n --store-dir) store=$2; shift 2 ;;\n --worker-session) id=$2; shift 2 ;;\n *) shift ;;\n esac\ndone\nHARNEZ_TEST_WORKER_STORE=$store HARNEZ_TEST_WORKER_ID=$id HARNEZ_TEST_BINARY='" + helperBinary + "' exec '" + helperBinary + "' -test.run=TestDetachedLaunchHelper\n"
	if err := os.WriteFile(launcher, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	old := agentExecutable
	agentExecutable = func() (string, error) { return launcher, nil }
	defer func() { agentExecutable = old }()
	cmd := newAgentCmd()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err = launchDetachedWithPreflight(cmd, startRequest{Name: "async-worker", Prompt: "task", StoredPrompt: "task", ModelSpec: "codex:luna:low", Dir: t.TempDir(), JSON: true}, storeDir, "", func(context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	var created subagent.Session
	if err := json.Unmarshal(out.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Status != "running" || created.ProcessPID == 0 || created.StdoutLog == "" || created.StderrLog == "" {
		t.Fatalf("created session = %#v", created)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	finished, err := waitForAgent(context.Background(), store, created.ID, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != "completed" || finished.Response != "helper done" {
		t.Fatalf("finished session = %#v", finished)
	}
	if processExists(created.ProcessPID) {
		t.Fatalf("worker process %d remains after wait returned", created.ProcessPID)
	}
}

func TestForegroundStartAndResumeDetachIntoWaitableWorkers(t *testing.T) {
	t.Setenv(foregroundDetachTestOverrideEnv, "1")
	t.Setenv(execTimeoutEffectiveEnv, "100ms")
	t.Setenv(execTimeoutExplicitEnv, "0")
	t.Setenv(execTimeoutShortEnv, "")
	t.Setenv(execTimeoutEnv, "")
	t.Setenv(agentRoleEnv, "")
	t.Setenv(agentSessionEnv, "")
	oldGrace := foregroundDetachGrace
	foregroundDetachGrace = time.Millisecond
	defer func() { foregroundDetachGrace = oldGrace }()

	helperBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	launcher := filepath.Join(t.TempDir(), "foreground-worker-launcher")
	script := "#!/bin/sh\nstore=''\nid=''\nwhile test $# -gt 0; do\n case \"$1\" in\n --store-dir) store=$2; shift 2 ;;\n --worker-session) id=$2; shift 2 ;;\n *) shift ;;\n esac\ndone\nHARNEZ_TEST_FOREGROUND_STORE=$store HARNEZ_TEST_FOREGROUND_ID=$id exec '" + helperBinary + "' -test.run=^TestForegroundWorkerHelper$\n"
	if err := os.WriteFile(launcher, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	oldExecutable := agentExecutable
	agentExecutable = func() (string, error) { return launcher, nil }
	defer func() { agentExecutable = oldExecutable }()
	oldDriver := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return &scriptDriver{} }
	defer func() { agentDriver = oldDriver }()

	for _, verb := range []string{"start", "resume"} {
		t.Run(verb, func(t *testing.T) {
			storeDir := t.TempDir()
			store, err := subagent.NewSessionStore(storeDir)
			if err != nil {
				t.Fatal(err)
			}
			name := "foreground-" + verb
			deps := agentDeps{
				store:    func() (*subagent.FileSessionStore, error) { return store, nil },
				storeDir: storeDir,
				parent:   func() string { return "" },
				find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
					return s.Find(id)
				},
			}
			cmd := newAgentCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			if verb == "start" {
				err = runStart(t.Context(), cmd, deps, startRequest{Name: name, Prompt: "task", StoredPrompt: "task", ModelSpec: "codex:luna:low", Dir: t.TempDir(), StreamMode: streamFull})
			} else {
				if err := store.Save(&subagent.Session{ID: "resume-id", Name: name, Provider: "codex", Model: "gpt-6-luna", Tier: "low", WorkingDir: t.TempDir(), Status: "completed", ContextTokens: 10}); err != nil {
					t.Fatal(err)
				}
				err = runResume(t.Context(), cmd, deps, resumeRequest{Name: name, Prompt: "continue", StreamMode: streamFull})
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, directive := range []string{"status=running", "cleanly detached", "Do NOT poll", "Do NOT schedule", "harnez agent wait " + name, "automatically notify"} {
				if !strings.Contains(out.String(), directive) {
					t.Errorf("detach output %q missing %q", out.String(), directive)
				}
			}
			sess, err := store.Find(name)
			if err != nil {
				t.Fatal(err)
			}
			if sess.Status != "running" || sess.ProcessPID <= 0 || sess.StdoutLog == "" || sess.StderrLog == "" {
				t.Fatalf("detached session = %#v", sess)
			}
			pgid, err := syscall.Getpgid(sess.ProcessPID)
			if err != nil || pgid != sess.ProcessPID {
				t.Fatalf("worker process group = %d, err=%v; want independent group %d", pgid, err, sess.ProcessPID)
			}

			waitCmd := newAgentCmd()
			var waitOut bytes.Buffer
			waitCmd.SetOut(&waitOut)
			waitCmd.SetArgs([]string{"--store-dir", storeDir, "wait", name})
			if err := waitCmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(waitOut.String(), "finished "+name) {
				t.Fatalf("wait output = %q", waitOut.String())
			}
		})
	}
}

func TestForegroundDetachDisabledInGoTestMode(t *testing.T) {
	t.Setenv(foregroundDetachTestOverrideEnv, "")
	t.Setenv(execTimeoutEffectiveEnv, "60s")
	t.Setenv(execTimeoutExplicitEnv, "0")
	t.Setenv(execTimeoutShortEnv, "")
	t.Setenv(execTimeoutEnv, "")
	if got := foregroundDetachTimeout(newAgentCmd()); got != 0 {
		t.Fatalf("foreground detach timeout = %s, want disabled in test mode", got)
	}
}

func TestForegroundWorkerEnvironmentOmitsLeafRole(t *testing.T) {
	t.Setenv(agentRoleEnv, "developer")
	for _, entry := range foregroundWorkerEnv() {
		if strings.HasPrefix(entry, agentRoleEnv+"=") {
			t.Fatalf("worker environment contains leaf role: %q", entry)
		}
	}
}

func TestWriteDetachGuidanceJSON(t *testing.T) {
	sess := &subagent.Session{ID: "session-id", Name: "calm-otter", Status: "running"}
	var out bytes.Buffer
	writeDetachGuidance(&out, sess, true)
	var got struct {
		Session      subagent.Session `json:"session"`
		Status       string           `json:"status"`
		Detached     bool             `json:"detached"`
		Message      string           `json:"message"`
		Instructions []string         `json:"instructions"`
		WaitCommand  string           `json:"wait_command"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("detach guidance is not JSON: %v (%s)", err, out.String())
	}
	if got.Session.ID != sess.ID || got.Session.Name != sess.Name || got.Status != "running" || !got.Detached {
		t.Fatalf("detach JSON = %#v", got)
	}
	if got.WaitCommand != "harnez agent wait calm-otter" || !strings.Contains(strings.Join(got.Instructions, " "), "Do NOT poll") || !strings.Contains(got.Message, "detached") {
		t.Fatalf("detach instructions = %#v", got)
	}
}

func TestAgentWorkerSessionCLIFlags(t *testing.T) {
	t.Setenv(agentRoleEnv, "")
	t.Setenv(agentSessionEnv, "")
	driver := &recordingAgentDriver{}
	oldDriver := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return driver }
	defer func() { agentDriver = oldDriver }()

	for _, tc := range []struct {
		name, verb, id, sessionName string
	}{
		{name: "start", verb: "start", id: "worker-start", sessionName: "start-worker"},
		{name: "resume", verb: "resume", id: "worker-resume", sessionName: "resume-worker"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storeDir := t.TempDir()
			store, err := subagent.NewSessionStore(storeDir)
			if err != nil {
				t.Fatal(err)
			}
			initial := &subagent.Session{ID: tc.id, Name: tc.sessionName, Provider: "claude", Model: "haiku", Tier: "low", ProviderSessionID: "provider-session", WorkingDir: t.TempDir(), Role: "developer", HarnessType: "harnez", Status: "running", ProcessPID: os.Getpid(), ContextTokens: 10}
			if tc.verb == "start" {
				initial.StartPrompt = "stored prompt"
			} else {
				initial.Status = "completed"
			}
			if err := store.Save(initial); err != nil {
				t.Fatal(err)
			}
			cmd := newAgentCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(new(bytes.Buffer))
			args := []string{"--store-dir", storeDir, tc.verb, "--worker-session", tc.id, "--json", "--name", tc.sessionName}
			if tc.verb == "start" {
				args = append(args, "--model", "claude:haiku:low", "--dir", initial.WorkingDir)
			}
			args = append(args, "--", "worker prompt")
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("execute %s --worker-session: %v", tc.verb, err)
			}
			var result agentOutput
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("%s worker output is not JSON: %v (%s)", tc.verb, err, out.String())
			}
			finished, err := store.Get(tc.id)
			if err != nil {
				t.Fatal(err)
			}
			if finished.Status != "completed" || finished.ProcessPID != 0 || result.Session == nil {
				t.Fatalf("%s worker result=%#v stored=%#v", tc.verb, result, finished)
			}
		})
	}
}

func TestForegroundWorkerHelper(t *testing.T) {
	storeDir, sessionID := os.Getenv("HARNEZ_TEST_FOREGROUND_STORE"), os.Getenv("HARNEZ_TEST_FOREGROUND_ID")
	if storeDir == "" || sessionID == "" {
		t.Skip("foreground worker helper")
	}
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	var sess *subagent.Session
	for i := 0; i < 100; i++ {
		sess, err = store.Get(sessionID)
		if err != nil {
			t.Fatal(err)
		}
		if sess.ProcessPID == os.Getpid() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if sess.ProcessPID != os.Getpid() {
		t.Fatal("parent did not register foreground worker PID")
	}
	time.Sleep(300 * time.Millisecond)
	sess.Status = "completed"
	sess.ProcessPID = 0
	sess.Response = "finished " + sess.Name
	sess.Messages = []string{sess.Response}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(agentOutput{Session: sess, Response: sess.Response, Messages: sess.Messages}); err != nil {
		t.Fatal(err)
	}
}

func TestDetachedLaunchHelper(t *testing.T) {
	storeDir, id := os.Getenv("HARNEZ_TEST_WORKER_STORE"), os.Getenv("HARNEZ_TEST_WORKER_ID")
	if storeDir == "" || id == "" {
		t.Skip("subprocess helper")
	}
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		sess, err := store.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if sess.ProcessPID == os.Getpid() {
			sess.Status = "completed"
			sess.ProcessPID = 0
			sess.Response = "helper done"
			if err := store.Save(sess); err != nil {
				t.Fatal(err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("parent did not register worker PID")
}

func TestReattachStepsUseHostVocabulary(t *testing.T) {
	for host, want := range map[string][]string{
		"codex":  {"exec_command", "write_stdin", "session_id", "Do NOT end your turn"},
		"claude": {"run_in_background: true", "Do NOT poll"},
		"agy":    {"background task", "Do NOT poll"},
		"":       {"host background job", "Do NOT poll"},
	} {
		got := strings.Join(reattachSteps(host, "calm-otter"), " ")
		for _, w := range append(want, "harnez agent wait calm-otter") {
			if !strings.Contains(got, w) {
				t.Errorf("host %q: %q missing %q", host, got, w)
			}
		}
	}
	if got := strings.Join(reattachSteps("codex", "x"), " "); strings.Contains(got, "Do NOT poll") {
		t.Errorf("codex steps forbid polling, its only wait mechanism: %q", got)
	}
}

func TestDetachHostPrefersCodexThread(t *testing.T) {
	t.Setenv("HARNEZ_AGENT", "claude")
	t.Setenv("CODEX_THREAD_ID", "t1")
	if got := detachHost(); got != "codex" {
		t.Fatalf("detachHost = %q, want codex", got)
	}
}

// TestAgentHelpHidesDetachAndNamesSessionBackground guards issue 630: hosts that
// read --detach in the help lost track of their workers.
func TestAgentHelpHidesDetachAndNamesSessionBackground(t *testing.T) {
	for _, sub := range []string{"start", "resume"} {
		t.Run(sub, func(t *testing.T) {
			c := newAgentCmd()
			var out bytes.Buffer
			c.SetOut(&out)
			c.SetErr(new(bytes.Buffer))
			c.SetArgs([]string{sub, "--help"})
			if err := c.Execute(); err != nil {
				t.Fatal(err)
			}
			help := out.String()
			for _, hidden := range []string{"--detach", "--async"} {
				if strings.Contains(help, hidden) {
					t.Errorf("%s help still lists %s:\n%s", sub, hidden, help)
				}
			}
			for _, want := range []string{"session background", "HTO=0", "run_in_background: true", "run_command", "WaitMsBeforeAsync", "exec_command", "write_stdin", "30 minutes", "Never check status or logs in a loop"} {
				if !strings.Contains(help, want) {
					t.Errorf("%s help missing %q", sub, want)
				}
			}
		})
	}
}

// TestHTOZeroDisablesForegroundDetach guards the one-call session-background
// form (issue 630): with HTO=0 the turn must never detach, and the wait hint
// must not ask for a separate wait job.
func TestHTOZeroDisablesForegroundDetach(t *testing.T) {
	t.Setenv(foregroundDetachTestOverrideEnv, "1")
	t.Setenv(execTimeoutEffectiveEnv, "60s")
	t.Setenv(execTimeoutExplicitEnv, "")
	t.Setenv(execTimeoutEnv, "")
	t.Setenv(execTimeoutShortEnv, "")
	if got := foregroundDetachTimeout(newAgentCmd()); got <= 0 {
		t.Fatalf("baseline detach timeout = %s, want > 0 without HTO", got)
	}
	if hint := waitHint("w"); !strings.Contains(hint, "harnez agent wait w") {
		t.Fatalf("baseline wait hint = %q, want reattach steps", hint)
	}
	t.Setenv(execTimeoutShortEnv, "0")
	if got := foregroundDetachTimeout(newAgentCmd()); got != 0 {
		t.Fatalf("detach timeout with HTO=0 = %s, want 0", got)
	}
	hint := waitHint("w")
	if strings.Contains(hint, "harnez agent wait") || !strings.Contains(hint, "does not detach") {
		t.Fatalf("wait hint with HTO=0 = %q, want no separate wait job", hint)
	}
}

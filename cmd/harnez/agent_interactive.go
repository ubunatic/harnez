package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/subagent"
)

type interactiveDeps struct {
	store    func() (*subagent.FileSessionStore, error)
	parent   func() string
	find     func(*cobra.Command, *subagent.FileSessionStore, string) (*subagent.Session, error)
	storeDir string
}

type interactiveStartRequest struct {
	Prompt, Name, ModelSpec, Dir, Role string
}

type interactiveResumeRequest struct{ Name, Prompt string }

func validateInteractiveFlags(cmd *cobra.Command, detached, jsonOut bool, stream string, timeout time.Duration, worker bool) error {
	if detached {
		return fmt.Errorf("--interactive cannot be combined with --detach/--async")
	}
	if timeout > 0 || cmd.Flags().Changed("timeout") || cmd.InheritedFlags().Changed("timeout") {
		return fmt.Errorf("--interactive cannot be combined with --timeout")
	}
	if jsonOut {
		return fmt.Errorf("--interactive cannot be combined with --json")
	}
	if stream != streamFull || cmd.Flags().Changed("stream") || cmd.InheritedFlags().Changed("stream") {
		return fmt.Errorf("--interactive cannot be combined with --stream")
	}
	if worker {
		return fmt.Errorf("--interactive cannot be used by a detached worker")
	}
	return nil
}

func runInteractiveStart(cmd *cobra.Command, d interactiveDeps, req interactiveStartRequest) error {
	spec := req.ModelSpec
	if spec == "" {
		var err error
		spec, err = subagent.DefaultModelSpec()
		if err != nil {
			return err
		}
	}
	m, err := subagent.ResolveModel(spec)
	if err != nil {
		return fmt.Errorf("agent start %q rejected: %w; ask for guidance rather than using a different model", spec, err)
	}
	role, err := startRole(req.Role)
	if err != nil {
		return err
	}
	canonicalDir, err := filepath.Abs(req.Dir)
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	store, err := d.store()
	if err != nil {
		return err
	}
	sessions, err := store.List("", true)
	if err != nil {
		return err
	}
	taken := make(map[string]bool, len(sessions)*2)
	for _, existing := range sessions {
		taken[existing.Name], taken[existing.ID] = true, true
	}
	if req.Name != "" && taken[req.Name] {
		return fmt.Errorf("session name %q is already in use", req.Name)
	}
	var sess *subagent.Session
	for {
		sessName := req.Name
		if sessName == "" {
			sessName, err = subagent.GenerateSessionName(func(candidate string) bool { return taken[candidate] })
			if err != nil {
				return err
			}
		}
		now := time.Now()
		sess = &subagent.Session{ID: uuid.NewString(), Name: sessName, StartPrompt: req.Prompt, Provider: m.Provider, Model: m.Name, Tier: m.Tier, WorkingDir: canonicalDir, ParentSessionID: d.parent(), CallerPID: os.Getpid(), HarnessType: "interactive", Status: "active", Role: role, CreatedAt: now, LastActiveAt: now}
		sess.ControlSocket = filepath.Join(d.storeDir, sess.ID+".sock")
		if m.Provider == "claude" {
			sess.ProviderSessionID = sess.ID
		}
		err = store.Create(sess)
		if err == nil {
			break
		}
		if req.Name != "" || !errors.Is(err, subagent.ErrSessionNameInUse) {
			return err
		}
		taken[sessName] = true
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Harnez Agent Interactive: %s (%s)\n", sess.Name, sess.ID)
	opts := interactiveOptions(cmd, store, sess, d.storeDir, req.Prompt)
	return launchInteractive(cmd, store, sess, func() error { return agentInteractiveRunner.Chat(cmd.Context(), opts) })
}

func runInteractiveResume(cmd *cobra.Command, d interactiveDeps, req interactiveResumeRequest) error {
	store, err := d.store()
	if err != nil {
		return err
	}
	sess, err := d.find(cmd, store, req.Name)
	if err != nil {
		return err
	}
	if !subagent.CanManage(d.parent(), sess) {
		return fmt.Errorf("session %q is outside caller lineage", sess.ID)
	}
	if sess.Status == "running" {
		return fmt.Errorf("session %q has a background turn running; use harnez agent wait %s before resuming interactively", sess.Name, sess.Name)
	}
	if sess.ProviderSessionID == "" {
		return fmt.Errorf("session %q cannot be resumed: %s did not expose a provider session ID", sess.Name, sess.Provider)
	}
	if sess.HarnessType == "interactive" && sess.Status == "active" {
		return fmt.Errorf("session %q is already active interactively", sess.Name)
	}
	sess.Status = "active"
	sess.ControlSocket = filepath.Join(d.storeDir, sess.ID+".sock")
	sess.LastActiveAt = time.Now()
	if err := store.Save(sess); err != nil {
		return err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Harnez Agent Resumed Interactively: %s (%s)\n", sess.Name, sess.ID)
	opts := interactiveOptions(cmd, store, sess, d.storeDir, req.Prompt)
	return launchInteractive(cmd, store, sess, func() error { return agentInteractiveRunner.Attach(cmd.Context(), opts, sess.ProviderID()) })
}

func interactiveOptions(cmd *cobra.Command, store *subagent.FileSessionStore, sess *subagent.Session, storeDir, prompt string) subagent.InteractiveOptions {
	return subagent.InteractiveOptions{Model: subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier}, Prompt: prompt, SessionID: sess.ID, Name: sess.Name, Dir: sess.WorkingDir, Stdin: cmd.InOrStdin(), Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr(), ControlSocket: filepath.Join(storeDir, sess.ID+".sock"), Started: func(pid int) error {
		sess.ProcessPID = pid
		return store.Save(sess)
	}}
}

func launchInteractive(cmd *cobra.Command, store *subagent.FileSessionStore, sess *subagent.Session, launch func() error) error {
	if err := store.Save(sess); err != nil {
		return err
	}
	err := launch()
	current, getErr := store.Get(sess.ID)
	if getErr != nil {
		return err
	}
	sess = current
	sess.LastActiveAt = time.Now()
	sess.ControlSocket = ""
	sess.ProcessPID = 0
	if sess.Status != "stopped" {
		if err != nil {
			sess.Status = "failed"
		} else {
			sess.Status = "completed"
		}
	}
	if saveErr := store.Save(sess); saveErr != nil {
		if err != nil {
			return fmt.Errorf("%v; save session state: %w", err, saveErr)
		}
		return saveErr
	}
	return err
}

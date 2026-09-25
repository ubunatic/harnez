package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/subagent"
)

var agentExecutable = os.Executable

func launchDetached(cmd *cobra.Command, req startRequest, storeDir, parentID string) error {
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
	role, err := startRole(req.Role)
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
	args := []string{"--store-dir", storeDir, "agent", "start", "--worker-session", id, "--json", "--model", modelSpec, "--role", role, "--name", req.Name, "--dir", dir, "--plan", map[bool]string{true: "yes", false: "no"}[req.PlanFirst], "--", req.Prompt}
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
	sess.ProcessPID = worker.Process.Pid
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
	err = runStart(cmd, agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return sess.ParentSessionID }}, req)
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
		if sess.Status != "running" {
			return sess, nil
		}
		if sess.ProcessPID > 0 && !processExists(sess.ProcessPID) {
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

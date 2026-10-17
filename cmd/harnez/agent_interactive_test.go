package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
)

type recordingInteractiveRunner struct {
	storeDir string
	opts     subagent.InteractiveOptions
	active   bool
	attachID string
	err      error
}

func (r *recordingInteractiveRunner) Chat(_ context.Context, opts subagent.InteractiveOptions) error {
	r.opts = opts
	store, err := subagent.NewSessionStore(r.storeDir)
	if err != nil {
		return err
	}
	sess, err := store.Find(opts.Name)
	r.active = err == nil && sess.Status == "active" && sess.HarnessType == "interactive"
	if err != nil {
		return err
	}
	return r.err
}

func TestAgentInteractiveFailureTransition(t *testing.T) {
	old := agentInteractiveRunner
	storeDir := t.TempDir()
	agentInteractiveRunner = &recordingInteractiveRunner{storeDir: storeDir, err: errors.New("provider exited")}
	defer func() { agentInteractiveRunner = old }()
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "-i", "--model", "claude:haiku", "--name", "bold-fox", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "provider exited") {
		t.Fatalf("chat error = %v", err)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sess, err := store.Find("bold-fox")
	if err != nil || sess.Status != "failed" {
		t.Fatalf("failed session = %#v, %v", sess, err)
	}
}

func TestAgentInteractiveActiveControlAndDeletion(t *testing.T) {
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver {
		t.Fatal("provider driver must not be called")
		return nil
	}
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	socket := filepath.Join(storeDir, "active.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	actions := make(chan struct {
		Action string `json:"action"`
		Prompt string `json:"prompt"`
	}, 3)
	go func() {
		for range 3 {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			var request struct {
				Action string `json:"action"`
				Prompt string `json:"prompt"`
			}
			_ = json.NewDecoder(conn).Decode(&request)
			actions <- request
			_ = json.NewEncoder(conn).Encode(struct{}{})
			_ = conn.Close()
		}
	}()
	// The socket is mocked; use an exited PID instead of a literal that could
	// belong to an unrelated real process during stop's ownership checks.
	exited := exec.Command("true")
	if err := exited.Run(); err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "active-id", ProviderSessionID: "provider-id", Name: "active", Provider: "claude", Model: "haiku", HarnessType: "interactive", Status: "active", ControlSocket: socket, ProcessPID: exited.Process.Pid, ContextTokens: 100, Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1, Rating: testRatingPtr(4)}}}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"resume", "--name", "active", "orchestrator prompt"}, {"compact", "--name", "active"}, {"stop", "--name", "active"}} {
		cmd := newAgentCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs(append(args, "--store-dir", storeDir))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	for _, want := range []struct{ action, prompt string }{{"prompt", "orchestrator prompt"}, {"compact", ""}, {"stop", ""}} {
		got := <-actions
		if got.Action != want.action || got.Prompt != want.prompt {
			t.Errorf("control = %#v, want %#v", got, want)
		}
	}
	sess, err = store.Find("active")
	if err != nil || sess.Status != "stopped" {
		t.Fatalf("stopped session = %#v, %v", sess, err)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "--name", "active", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Find("active"); err == nil {
		t.Fatal("completed/stopped interactive registry metadata was not deleted")
	}
	completed := &subagent.Session{ID: "completed-id", Name: "completed", Provider: "agy", Model: "gemini-3.7-flash", HarnessType: "interactive", Status: "completed", Turn: 1, TurnRecords: []subagent.TurnRecord{{Turn: 1, Rating: testRatingPtr(4)}}}
	if err := store.Save(completed); err != nil {
		t.Fatal(err)
	}
	cmd = newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "--name", "completed", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(completed.ID); err == nil {
		t.Fatal("completed interactive registry metadata was not deleted")
	}
}

func TestAgentInteractiveMissingProviderIDLimitsPostExitOnly(t *testing.T) {
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "codex-id", Name: "codex-chat", Provider: "codex", Model: "gpt-6-luna", HarnessType: "interactive", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "-i", "--name", "codex-chat", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "did not expose a provider session ID") {
		t.Fatalf("attach error = %v", err)
	}
}

func (r *recordingInteractiveRunner) Attach(_ context.Context, opts subagent.InteractiveOptions, providerID string) error {
	r.opts = opts
	r.attachID = providerID
	return nil
}

func TestAgentInteractiveRegistersBeforeLaunchAndTransitions(t *testing.T) {
	old := agentInteractiveRunner
	storeDir := t.TempDir()
	runner := &recordingInteractiveRunner{storeDir: storeDir}
	agentInteractiveRunner = runner
	defer func() { agentInteractiveRunner = old }()

	dir := t.TempDir()
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetIn(bytes.NewBufferString("input"))
	cmd.SetArgs([]string{"start", "-i", "--model", "claude:haiku", "--name", "calm-otter", "-d", dir, "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !runner.active {
		t.Fatal("session was not registered as active interactive before launch")
	}
	if runner.opts.Stdin == nil || runner.opts.Stdout == nil || runner.opts.Stderr == nil {
		t.Fatal("interactive terminal streams were not inherited")
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sess, err := store.Find("calm-otter")
	if err != nil {
		t.Fatal(err)
	}
	if sess.Status != "completed" || sess.ProviderSessionID != sess.ID || sess.CallerPID != os.Getpid() || sess.WorkingDir != dir {
		t.Fatalf("saved interactive session = %#v", sess)
	}

	cmd = newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "-i", "--name", "calm-otter", "--store-dir", storeDir, "--", "continue interactively"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.attachID != sess.ID {
		t.Fatalf("attach provider ID = %q, want %q", runner.attachID, sess.ID)
	}
	if runner.opts.Prompt != "continue interactively" {
		t.Fatalf("resume opening prompt = %q", runner.opts.Prompt)
	}
}

func TestAgentInteractiveResumeDefaultsToLatestInDirectory(t *testing.T) {
	old := agentInteractiveRunner
	storeDir := t.TempDir()
	runner := &recordingInteractiveRunner{storeDir: storeDir}
	agentInteractiveRunner = runner
	defer func() { agentInteractiveRunner = old }()
	dir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	for _, sess := range []*subagent.Session{
		{ID: "old", Name: "old-session", ProviderSessionID: "old-provider", Provider: "agy", Model: "gemini-3.7-flash", WorkingDir: dir, HarnessType: "interactive", Status: "completed", LastActiveAt: time.Now().Add(-time.Minute)},
		{ID: "new", Name: "new-session", ProviderSessionID: "new-provider", Provider: "agy", Model: "gemini-3.7-flash", WorkingDir: dir, HarnessType: "interactive", Status: "active", ProcessPID: 99999999, LastActiveAt: time.Now()},
	} {
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "-i", "-d", dir, "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.attachID != "new-provider" {
		t.Fatalf("attached provider ID = %q, want new-provider", runner.attachID)
	}
}

func TestInteractiveProviderIDIsPersistedDuringRun(t *testing.T) {
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "session", Name: "session", Provider: "codex", Model: "gpt-6-luna", HarnessType: "interactive", Status: "active"}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	opts, err := interactiveOptions(newAgentCmd(), store, sess, storeDir, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := opts.ProviderIDFound("codex-thread"); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Get(sess.ID)
	if err != nil || loaded.ProviderSessionID != "codex-thread" {
		t.Fatalf("stored provider session ID = %#v, %v", loaded, err)
	}
}

func TestAgentInteractivePromptAndFlagConflicts(t *testing.T) {
	old := agentInteractiveRunner
	storeDir := t.TempDir()
	runner := &recordingInteractiveRunner{storeDir: storeDir}
	agentInteractiveRunner = runner
	defer func() { agentInteractiveRunner = old }()

	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "-i", "--model", "claude:haiku", "--store-dir", storeDir, "--", "opening line"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.opts.Prompt != "opening line" {
		t.Fatalf("opening prompt = %q", runner.opts.Prompt)
	}

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"detach", []string{"start", "-i", "--detach"}, "--detach/--async"},
		{"async", []string{"start", "-i", "--async"}, "--detach/--async"},
		{"timeout", []string{"start", "-i", "--timeout", "0"}, "--timeout"},
		{"json", []string{"start", "-i", "--json"}, "--json"},
		{"stream", []string{"start", "-i", "--stream", "stats"}, "--stream"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newAgentCmd()
			c.SetOut(new(bytes.Buffer))
			c.SetErr(new(bytes.Buffer))
			c.SetArgs(append(tc.args, "--store-dir", t.TempDir()))
			if err := c.Execute(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("conflict error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestAgentResumeInteractiveRejectsRunningBackgroundSession(t *testing.T) {
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "busy-id", ProviderSessionID: "provider-id", Name: "busy-worker", Provider: "claude", Model: "haiku", WorkingDir: t.TempDir(), HarnessType: "harnez", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "-i", "--name", "busy-worker", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "harnez agent wait busy-worker") {
		t.Fatalf("busy-session error = %v", err)
	}
}

func TestAgentInteractiveGeneratesUniqueNameAndRejectsDuplicate(t *testing.T) {
	old := agentInteractiveRunner
	storeDir := t.TempDir()
	agentInteractiveRunner = &recordingInteractiveRunner{storeDir: storeDir}
	defer func() { agentInteractiveRunner = old }()

	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "-i", "--model", "claude:haiku", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sessions, err := store.List("", true)
	if err != nil || len(sessions) != 1 || sessions[0].Name == "" {
		t.Fatalf("generated session registration = %#v, %v", sessions, err)
	}
	cmd = newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "-i", "--model", "claude:haiku", "--name", sessions[0].Name, "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected duplicate name error")
	}
}

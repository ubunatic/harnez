package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
)

func TestAgentCommandSurface(t *testing.T) {
	c := newAgentCmd()
	if len(c.Commands()) == 0 {
		t.Fatal("agent command has no children")
	}
	for _, name := range []string{"start", "chat", "resume", "list", "status", "compact", "stop", "delete", "enable", "disable"} {
		found := false
		for _, child := range c.Commands() {
			if child.Name() == name {
				found = true
			}
		}
		if !found {
			t.Errorf("missing agent subcommand %q", name)
		}
	}
}

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

func TestAgentChatFailureTransition(t *testing.T) {
	old := agentInteractiveRunner
	storeDir := t.TempDir()
	agentInteractiveRunner = &recordingInteractiveRunner{storeDir: storeDir, err: errors.New("provider exited")}
	defer func() { agentInteractiveRunner = old }()
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"chat", "claude:haiku", "--name", "bold-fox", "--store-dir", storeDir})
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
	agentDriver = func(subagent.Model) subagent.Driver {
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
	sess := &subagent.Session{ID: "active-id", ProviderSessionID: "provider-id", Name: "active", Provider: "claude", Model: "haiku", HarnessType: "interactive", Status: "active", ControlSocket: socket, ProcessPID: 123}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"resume", "active", "orchestrator prompt"}, {"compact", "active"}, {"stop", "active"}} {
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
	cmd.SetArgs([]string{"delete", "active", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Find("active"); err == nil {
		t.Fatal("completed/stopped interactive registry metadata was not deleted")
	}
	completed := &subagent.Session{ID: "completed-id", Name: "completed", Provider: "agy", Model: "gemini-3.7-flash", HarnessType: "interactive", Status: "completed"}
	if err := store.Save(completed); err != nil {
		t.Fatal(err)
	}
	cmd = newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "completed", "--store-dir", storeDir})
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
	if err := store.Save(&subagent.Session{ID: "codex-id", Name: "codex-chat", Provider: "codex", Model: "gpt-5.6-luna", HarnessType: "interactive", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"chat", "attach", "codex-chat", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "does not expose a provider session ID") {
		t.Fatalf("attach error = %v", err)
	}
}

func (r *recordingInteractiveRunner) Attach(_ context.Context, opts subagent.InteractiveOptions, providerID string) error {
	r.opts = opts
	r.attachID = providerID
	return nil
}

func TestAgentChatRegistersBeforeLaunchAndTransitions(t *testing.T) {
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
	cmd.SetArgs([]string{"chat", "claude:haiku", "--name", "calm-otter", "-d", dir, "--store-dir", storeDir})
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
	cmd.SetArgs([]string{"chat", "attach", "calm-otter", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.attachID != sess.ID {
		t.Fatalf("attach provider ID = %q, want %q", runner.attachID, sess.ID)
	}
}

func TestAgentChatGeneratesUniqueNameAndRejectsDuplicate(t *testing.T) {
	old := agentInteractiveRunner
	storeDir := t.TempDir()
	agentInteractiveRunner = &recordingInteractiveRunner{storeDir: storeDir}
	defer func() { agentInteractiveRunner = old }()

	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"chat", "claude:haiku", "--store-dir", storeDir})
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
	cmd.SetArgs([]string{"chat", "claude:haiku", "--name", sessions[0].Name, "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected duplicate name error")
	}
}

func TestAgentRepoStatusAndPolicyCommands(t *testing.T) {
	dir := t.TempDir()
	store := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.local.md"), []byte("local notes\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"enable", "-d", dir, "--store-dir", store})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "AGENTS.local.md"))
	if err != nil || !bytes.Contains(content, []byte("subagent_mode: harnez")) || !bytes.Contains(content, []byte("local notes")) {
		t.Fatalf("enable did not update local overlay: %v\n%s", err, content)
	}
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "-d", dir, "--store-dir", store})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Subagent Policy State: Enabled (./AGENTS.local.md)")) || !bytes.Contains(out.Bytes(), []byte("Repository Agent Sessions:")) {
		t.Fatalf("status output = %s", out.String())
	}
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"disable", "-d", dir, "--store-dir", store})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(filepath.Join(dir, "AGENTS.local.md"))
	if err != nil || !bytes.Contains(content, []byte("subagent_mode: native")) {
		t.Fatalf("disable did not update local overlay: %v\n%s", err, content)
	}
}

func TestAgentRepoStatusUnsetJSONAndSingleSession(t *testing.T) {
	dir, storeDir := t.TempDir(), t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "inside", Name: "demo", Provider: "codex", Model: "model", Status: "completed", WorkingDir: dir, LastActiveAt: time.Now()}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "-d", dir, "--store-dir", storeDir, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Policy struct {
			Mode string `json:"mode"`
		} `json:"policy"`
		Sessions []subagent.Session `json:"sessions"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Policy.Mode != "unset" || len(got.Sessions) != 1 || got.Sessions[0].ID != "inside" {
		t.Fatalf("repo status = %#v", got)
	}
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "inside", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil || !bytes.Contains(out.Bytes(), []byte("ID: inside\nName: demo\nStatus: completed")) {
		t.Fatalf("single session status: err=%v output=%s", err, out.String())
	}
}

type recordingAgentDriver struct{ dir string }

func (d *recordingAgentDriver) Run(_ context.Context, opts subagent.RunOptions) (*subagent.TurnResult, error) {
	d.dir = opts.Dir
	return &subagent.TurnResult{SessionID: "recorded", Response: "ok"}, nil
}
func (d *recordingAgentDriver) Resume(context.Context, string, string) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{}, nil
}
func (d *recordingAgentDriver) Compact(context.Context, string) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{}, nil
}
func (d *recordingAgentDriver) Stop(context.Context, string) error   { return nil }
func (d *recordingAgentDriver) Delete(context.Context, string) error { return nil }

func TestAgentStartStoresCanonicalWorkingDir(t *testing.T) {
	old := agentDriver
	recorder := &recordingAgentDriver{}
	agentDriver = func(subagent.Model) subagent.Driver { return recorder }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "codex:luna", "prompt", "-d", ".", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(".")
	if err != nil || recorder.dir != want {
		t.Fatalf("driver working directory = %q, want %q", recorder.dir, want)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sess, err := store.Get("recorded")
	if err != nil || sess.WorkingDir != want {
		t.Fatalf("saved working directory = %q, err=%v", sess.WorkingDir, err)
	}
}

func TestAgentHaikuAliases(t *testing.T) {
	for _, spec := range []string{"claude:haiku", "claude:haiku:latest"} {
		m, err := subagent.ResolveModel(spec)
		if err != nil || m.Provider != "claude" || m.Name != "haiku" || m.Tier != "low" {
			t.Fatalf("%s resolved to %#v (%v)", spec, m, err)
		}
	}
}

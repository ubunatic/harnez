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

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/subagent"
)

func TestAgentCommandSurface(t *testing.T) {
	c := newAgentCmd()
	if len(c.Commands()) == 0 {
		t.Fatal("agent command has no children")
	}
	for _, name := range []string{"start", "models", "chat", "resume", "list", "status", "compact", "stop", "delete", "enable", "disable"} {
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

func TestAgentCompletionDescription(t *testing.T) {
	long := strings.Repeat("prompt ", 30)
	tests := []struct {
		name, prompt, want string
	}{
		{name: "missing", want: "agent prompt unavailable"},
		{name: "short", prompt: "fix the build", want: "fix the build"},
		{name: "multiline", prompt: "first line\nsecond\tline", want: "first line second line"},
		{name: "sensitive", prompt: "email me at user@example.com", want: "email me at [redacted-email]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sessionPromptDescription(tt.prompt); got != tt.want {
				t.Fatalf("description = %q, want %q", got, tt.want)
			}
		})
	}
	if got := sessionPromptDescription(long); len([]rune(got)) != 100 || !strings.HasSuffix(got, "...") {
		t.Fatalf("long description = %q, want 100 runes with ellipsis", got)
	}
}

func TestAgentReferenceCommandCompletionCoverage(t *testing.T) {
	root := newAgentCmd()
	for _, path := range [][]string{{"resume"}, {"status"}, {"compact"}, {"stop"}, {"delete"}, {"chat", "attach"}} {
		cmd, _, err := root.Find(path)
		if err != nil {
			t.Fatalf("find %v: %v", path, err)
		}
		if cmd.ValidArgsFunction == nil {
			t.Errorf("%s has no session completion", strings.Join(path, " "))
		}
	}
}

func TestAgentNameCompletion(t *testing.T) {
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "one", Name: "calm-otter", StartPrompt: "Investigate the release process"}); err != nil {
		t.Fatal(err)
	}
	completer := agentSessionCompletion(storeDir, func() string { return "" })
	completions, directive := completer(newAgentCmd(), nil, "cal")
	if directive != cobra.ShellCompDirectiveNoFileComp || len(completions) != 1 || completions[0] != "calm-otter\tInvestigate the release process" {
		t.Fatalf("completion = %#v, directive = %v", completions, directive)
	}
}

func TestAgentModelsListsKnownSpecs(t *testing.T) {
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"models"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "codex:luna:low") || !strings.Contains(out.String(), "agy:flash:low") {
		t.Fatalf("known models output = %q", out.String())
	}
}

func TestAgentStartRejectsUnknownModelWithoutCreatingSession(t *testing.T) {
	storeDir := t.TempDir()
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "codex:missing:low", "x", "--store-dir", storeDir})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "codex:missing:low") || !strings.Contains(err.Error(), "ask for guidance") {
		t.Fatalf("start error = %v, want requested spec and guidance", err)
	}
	store, storeErr := subagent.NewSessionStore(storeDir)
	if storeErr != nil {
		t.Fatal(storeErr)
	}
	sessions, listErr := store.List("", true)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(sessions) != 0 {
		t.Fatalf("sessions after rejected start = %#v, want none", sessions)
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

type recordingAgentDriver struct {
	dir     string
	stopped []string
	deleted []string
}

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
func (d *recordingAgentDriver) Stop(_ context.Context, id string) error {
	d.stopped = append(d.stopped, id)
	return nil
}
func (d *recordingAgentDriver) Delete(_ context.Context, id string) error {
	d.deleted = append(d.deleted, id)
	return nil
}

func TestAgentStopAndDeleteAllManageableSessions(t *testing.T) {
	storeDir := t.TempDir()
	oldSessionID := os.Getenv("HARNEZ_SESSION_ID")
	if err := os.Setenv("HARNEZ_SESSION_ID", "caller"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HARNEZ_SESSION_ID", oldSessionID) }()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, sess := range []*subagent.Session{
		{ID: "one", Name: "one", Provider: "codex", Model: "model", Status: "completed", HarnessType: "batch", ParentSessionID: "caller"},
		{ID: "two", Name: "two", Provider: "codex", Model: "model", Status: "completed", HarnessType: "batch", ParentSessionID: "foreign"},
	} {
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	old := agentDriver
	recorder := &recordingAgentDriver{}
	agentDriver = func(subagent.Model) subagent.Driver { return recorder }
	defer func() { agentDriver = old }()

	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"stop", "--all", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(recorder.stopped) != 1 || recorder.stopped[0] != "one" {
		t.Fatalf("stopped sessions = %#v", recorder.stopped)
	}

	cmd = newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"delete", "--all", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(recorder.deleted) != 1 || recorder.deleted[0] != "one" {
		t.Fatalf("deleted sessions = %#v", recorder.deleted)
	}
	if _, err := store.Get("one"); err == nil {
		t.Fatal("manageable session was not deleted")
	}
	if _, err := store.Get("two"); err != nil {
		t.Fatalf("foreign session was deleted: %v", err)
	}
}

func TestAgentStopAndDeleteAllWithNoSessions(t *testing.T) {
	storeDir := t.TempDir()
	for _, args := range [][]string{{"stop", "--all"}, {"delete", "--all"}} {
		cmd := newAgentCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetArgs(append(args, "--store-dir", storeDir))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
}

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

func TestAgentResumePrintsReplyNotStructDump(t *testing.T) {
	old := agentDriver
	agentDriver = func(subagent.Model) subagent.Driver { return &replyDriver{} }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "luna", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"resume", "worker", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); strings.Contains(got, "{0x") || !strings.Contains(got, "[harnez session] Resumed: sid") || !strings.Contains(got, "[agent response]\nthe reply text") {
		t.Fatalf("stdout = %q, want plain reply without struct dump", got)
	}
	if !strings.Contains(errOut.String(), "caller must wait") {
		t.Fatalf("stderr = %q, want wait notice", errOut.String())
	}
}

type replyDriver struct{ recordingAgentDriver }

func (*replyDriver) Resume(context.Context, string, string) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{Response: "the reply text"}, nil
}

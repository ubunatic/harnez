package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
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
	old := agentDriver
	agentDriver = func(subagent.Model) subagent.Driver { t.Fatal("provider driver must not be called"); return nil }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "codex:missing:low", "x", "--store-dir", storeDir})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "model is now --model") {
		t.Fatalf("start error = %v, want model flag guidance", err)
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

func TestAgentOldModelSpecGuard(t *testing.T) {
	tests := []struct {
		name, first string
		reject      bool
	}{
		{"known", "codex:luna:low", false},
		{"mistyped known provider", "codex:missing:low", true},
		{"unknown provider remains prompt", "note:fix", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := agentDriver
			d := &recordingAgentDriver{}
			agentDriver = func(subagent.Model) subagent.Driver { return d }
			defer func() { agentDriver = old }()
			cmd := newAgentCmd()
			var errOut bytes.Buffer
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(&errOut)
			args := []string{"start", tt.first, "prompt", "--store-dir", t.TempDir()}
			if tt.first == "codex:luna:low" {
				args = []string{"start", "--model", tt.first, "prompt", "--store-dir", t.TempDir()}
			}
			cmd.SetArgs(args)
			err := cmd.Execute()
			if tt.reject {
				if err == nil || !strings.Contains(err.Error(), "model is now --model") {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAgentResumeRequiresName(t *testing.T) {
	storeDir := t.TempDir()
	var errOut bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"resume", "worker", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "multiple resumable agents") && !strings.Contains(err.Error(), "no resumable agent") {
		t.Fatalf("error = %v", err)
	}
}

func TestAgentStartFileOnlyAndNameCollision(t *testing.T) {
	old := agentDriver
	d := &recordingAgentDriver{}
	agentDriver = func(subagent.Model) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	file := filepath.Join(t.TempDir(), "prompt.md")
	if err := os.WriteFile(file, []byte("full file prompt"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "--name", "named", "-f", file, "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sess, err := store.Find("named")
	if err != nil || !strings.Contains(sess.StartPrompt, file+" (16 bytes)") || strings.Contains(sess.StartPrompt, "full file prompt") {
		t.Fatalf("stored prompt = %#v, err=%v", sess, err)
	}
	cmd = newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "--name", "named", "again", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "recorded") || !strings.Contains(err.Error(), sess.WorkingDir) {
		t.Fatalf("collision error = %v", err)
	}
}

func TestAgentResumeModelConflict(t *testing.T) {
	old := agentDriver
	agentDriver = func(subagent.Model) subagent.Driver { t.Fatal("provider driver must not be called"); return nil }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "luna", Tier: "low", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "--name", "worker", "--model", "codex:luna:latest", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("conflict error = %v", err)
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
	cmd.SetArgs([]string{"chat", "--model", "claude:haiku", "--name", "bold-fox", "--store-dir", storeDir})
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
	completed := &subagent.Session{ID: "completed-id", Name: "completed", Provider: "agy", Model: "gemini-3.7-flash", HarnessType: "interactive", Status: "completed"}
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
	if err := store.Save(&subagent.Session{ID: "codex-id", Name: "codex-chat", Provider: "codex", Model: "gpt-5.6-luna", HarnessType: "interactive", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"chat", "attach", "--name", "codex-chat", "--store-dir", storeDir})
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
	cmd.SetArgs([]string{"chat", "--model", "claude:haiku", "--name", "calm-otter", "-d", dir, "--store-dir", storeDir})
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
	cmd.SetArgs([]string{"chat", "attach", "--name", "calm-otter", "--store-dir", storeDir})
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
	cmd.SetArgs([]string{"chat", "--model", "claude:haiku", "--store-dir", storeDir})
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
	cmd.SetArgs([]string{"chat", "--model", "claude:haiku", "--name", sessions[0].Name, "--store-dir", storeDir})
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
	cmd.SetArgs([]string{"status", "--name", "inside", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil || !bytes.Contains(out.Bytes(), []byte("ID: inside\nName: demo\nStatus: completed")) {
		t.Fatalf("single session status: err=%v output=%s", err, out.String())
	}
}

type recordingAgentDriver struct {
	dir     string
	stopped []string
	deleted []string
}

type resumeOutcomeDriver struct {
	recordingAgentDriver
	err      error
	resumes  int
	terminal bool
}

func (d *resumeOutcomeDriver) Resume(context.Context, string, string) (*subagent.TurnResult, error) {
	d.resumes++
	if d.err != nil {
		return nil, d.err
	}
	return &subagent.TurnResult{Response: "resumed"}, nil
}

func (d *resumeOutcomeDriver) CheckResumable(string) (bool, string) {
	if d.terminal {
		return false, "session record is terminal"
	}
	return true, ""
}

func saveResumeSession(t *testing.T, dir, id, name, provider string) *subagent.FileSessionStore {
	t.Helper()
	store, err := subagent.NewSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: id, Name: name, Provider: provider, Model: "model", Tier: "low", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestAgentListThenResumeByDisplayedIDAndName(t *testing.T) {
	for _, identifier := range []string{"sid", "worker"} {
		t.Run(identifier, func(t *testing.T) {
			old := agentDriver
			d := &resumeOutcomeDriver{}
			agentDriver = func(subagent.Model) subagent.Driver { return d }
			defer func() { agentDriver = old }()
			storeDir := t.TempDir()
			saveResumeSession(t, storeDir, "sid", "worker", "fake")
			var listed bytes.Buffer
			cmd := newAgentCmd()
			cmd.SetOut(&listed)
			cmd.SetArgs([]string{"list", "--all-sessions", "--store-dir", storeDir})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(listed.String(), "sid\tworker") {
				t.Fatalf("list = %q", listed.String())
			}
			cmd = newAgentCmd()
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs([]string{"resume", "--name", identifier, "hi", "--store-dir", storeDir})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAgentResumeFailureRecordedAndCleared(t *testing.T) {
	old := agentDriver
	d := &resumeOutcomeDriver{err: errors.New("provider failed\nwith detail")}
	agentDriver = func(subagent.Model) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store := saveResumeSession(t, storeDir, "sid", "worker", "fake")
	for want := 1; want <= 2; want++ {
		cmd := newAgentCmd()
		cmd.SetArgs([]string{"resume", "--name", "worker", "hi", "--store-dir", storeDir})
		if err := cmd.Execute(); err == nil {
			t.Fatal("resume unexpectedly succeeded")
		}
		sess, _ := store.Get("sid")
		if sess.ResumeFailures != want || sess.LastError != "provider failed" {
			t.Fatalf("session = %#v", sess)
		}
		if len([]rune(sess.LastError)) > 300 {
			t.Fatal("last error too long")
		}
	}
	d.err = nil
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "--name", "worker", "hi", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	sess, _ := store.Get("sid")
	if sess.LastError != "" || sess.ResumeFailures != 0 {
		t.Fatalf("success did not clear failure: %#v", sess)
	}
}

func TestAgentListResumeColumnStates(t *testing.T) {
	old := agentDriver
	agentDriver = func(m subagent.Model) subagent.Driver {
		d := &resumeOutcomeDriver{}
		if m.Provider == "terminal" {
			d.terminal = true
		}
		return d
	}
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	for _, sess := range []*subagent.Session{
		{ID: "ok", Name: "okay", Provider: "fake", Model: "model", Status: "completed"},
		{ID: "bad", Name: "failed", Provider: "fake", Model: "model", LastError: "provider failed", Status: "completed"},
		{ID: "term", Name: "terminal", Provider: "terminal", Model: "model", Status: "completed"},
	} {
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"list", "--all-sessions", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"okay\tfake\tcompleted\tok", "failed\tfake\tcompleted\tfailed", "terminal\tterminal\tcompleted\tterminal"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("list = %q, missing %q", out.String(), want)
		}
	}
}

func TestAgentTerminalResumeRefusal(t *testing.T) {
	old := agentDriver
	d := &resumeOutcomeDriver{terminal: true}
	agentDriver = func(subagent.Model) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store := saveResumeSession(t, storeDir, "sid", "worker", "fake")
	cmd := newAgentCmd()
	cmd.SetArgs([]string{"resume", "--name", "worker", "hi", "--store-dir", storeDir})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot be resumed") || !strings.Contains(err.Error(), "harnez agent start --name") {
		t.Fatalf("error = %v", err)
	}
	sess, _ := store.Get("sid")
	if d.resumes != 0 || sess.ResumeFailures != 0 {
		t.Fatalf("terminal attempt changed state: resumes=%d session=%#v", d.resumes, sess)
	}
}

func TestAgentStatusResumeDiagnostics(t *testing.T) {
	storeDir := t.TempDir()
	store := saveResumeSession(t, storeDir, "sid", "worker", "fake")
	sess, _ := store.Get("sid")
	sess.LastError = "provider failed"
	sess.ResumeFailures = 2
	_ = store.Save(sess)
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "--name", "worker", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Last error: provider failed") || !strings.Contains(out.String(), "Resume failures: 2") {
		t.Fatalf("status = %q", out.String())
	}
	sess.LastError = ""
	_ = store.Save(sess)
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "--name", "worker", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "Last error:") || strings.Contains(out.String(), "Resume failures:") {
		t.Fatalf("clean status = %q", out.String())
	}
}

func TestAgentResumeMissingIdentifier(t *testing.T) {
	cmd := newAgentCmd()
	cmd.SetArgs([]string{"resume", "--name", "nope", "hi", "--store-dir", t.TempDir()})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestAttributableOrderingAndFilters(t *testing.T) {
	old := agentDriver
	agentDriver = func(m subagent.Model) subagent.Driver {
		d := &resumeOutcomeDriver{}
		d.terminal = m.Provider == "terminal"
		return d
	}
	defer func() { agentDriver = old }()
	now := time.Now()
	sessions := []*subagent.Session{
		{ID: "old", Name: "old", Provider: "fake", WorkingDir: ".", Status: "completed", LastActiveAt: now.Add(-time.Hour)},
		{ID: "new", Name: "new", Provider: "fake", WorkingDir: ".", Status: "completed", LastActiveAt: now},
		{ID: "other-dir", Name: "other-dir", Provider: "fake", WorkingDir: "/else", Status: "completed", LastActiveAt: now.Add(time.Hour)},
		{ID: "other-caller", Name: "other-caller", Provider: "fake", WorkingDir: ".", ParentSessionID: "else", Status: "completed", LastActiveAt: now.Add(time.Hour)},
		{ID: "terminal", Name: "terminal", Provider: "terminal", WorkingDir: ".", Status: "completed", LastActiveAt: now.Add(time.Hour)},
	}
	for _, sess := range sessions[:2] {
		sess.ParentSessionID = "caller"
	}
	got := attributable(sessions, ".", "caller")
	if len(got) != 2 || got[0].Name != "new" || got[1].Name != "old" {
		t.Fatalf("attributable = %#v", got)
	}
}

func TestAgentResumeAttributionAndContinue(t *testing.T) {
	old := agentDriver
	d := &resumeOutcomeDriver{}
	agentDriver = func(subagent.Model) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	now := time.Now()
	for _, sess := range []*subagent.Session{{ID: "one", Name: "one", Provider: "fake", Model: "model", WorkingDir: ".", Status: "completed", LastActiveAt: now.Add(-time.Hour)}, {ID: "two", Name: "two", Provider: "fake", Model: "model", WorkingDir: ".", Status: "completed", LastActiveAt: now}} {
		_ = store.Save(sess)
	}
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "--continue", "hi", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if d.resumes != 1 {
		t.Fatalf("resumes=%d", d.resumes)
	}
	cmd = newAgentCmd()
	cmd.SetArgs([]string{"resume", "--name", "one", "--continue", "hi", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("error=%v", err)
	}
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
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "prompt", "-d", ".", "--store-dir", storeDir})
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
	cmd.SetArgs([]string{"resume", "--name", "worker", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); strings.Contains(got, "{0x") || !strings.Contains(got, "[agent messages]\n[msg 1]\nthe reply text") {
		t.Fatalf("stdout = %q, want plain reply without struct dump", got)
	}
	if e := errOut.String(); !strings.HasPrefix(e, "[session timeline]\n") || !strings.Contains(e, "caller must wait") || !strings.Contains(e, " done] ") {
		t.Fatalf("stderr = %q, want timeline with wait notice", e)
	}
}

type replyDriver struct{ recordingAgentDriver }

func (*replyDriver) Resume(context.Context, string, string) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{Response: "the reply text"}, nil
}

type ackDriver struct{ recordingAgentDriver }

func (*ackDriver) Resume(context.Context, string, string) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{Response: "real reply", Messages: []string{"Context compacted.", "real reply"}, TokensTurn: 900000, CachedTokens: 890000}, nil
}

func TestAgentResumeCompactsOnceAndSeparatesAck(t *testing.T) {
	old := agentDriver
	agentDriver = func(subagent.Model) subagent.Driver { return &ackDriver{} }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "luna", Status: "completed", TokensCumulative: 7000000, TokensSinceCompact: 150000}); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"resume", "--name", "worker", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "[agent messages]\n[msg 1]\nreal reply\n" {
		t.Fatalf("stdout = %q", got)
	}
	if e := errOut.String(); !strings.Contains(e, " compact] queued /compact") || !strings.Contains(e, "agent acknowledged: Context compacted.") {
		t.Fatalf("stderr = %q", e)
	}
	sess, err := store.Get("sid")
	if err != nil || sess.TokensSinceCompact != 10000 || sess.TokensCumulative != 7900000 {
		t.Fatalf("since=%d cumulative=%d err=%v", sess.TokensSinceCompact, sess.TokensCumulative, err)
	}
	if subagent.ShouldCompact(sess.TokensSinceCompact) {
		t.Fatal("next resume must not compact again")
	}
}

type streamDriver struct{ recordingAgentDriver }

func (*streamDriver) emit(fn subagent.EventFunc) *subagent.TurnResult {
	fn(subagent.Event{Kind: "session", Text: "thread-1", Bytes: 40})
	fn(subagent.Event{Kind: "message", Text: "on it", Bytes: 80})
	fn(subagent.Event{Kind: "activity", Text: "running sleep", Bytes: 80})
	time.Sleep(60 * time.Millisecond)
	fn(subagent.Event{Kind: "message", Text: "all done", Bytes: 80})
	return &subagent.TurnResult{SessionID: "thread-1", Response: "all done", Messages: []string{"on it", "all done"}, TokensTurn: 500}
}
func (d *streamDriver) RunStream(_ context.Context, _ subagent.RunOptions, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	return d.emit(fn), nil
}
func (d *streamDriver) ResumeStream(_ context.Context, _, _ string, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	return d.emit(fn), nil
}

func TestAgentStartStreamsLabeledBlocks(t *testing.T) {
	old, oldSched, oldRepeat := agentDriver, heartbeatSchedule, heartbeatRepeat
	agentDriver = func(subagent.Model) subagent.Driver { return &streamDriver{} }
	heartbeatSchedule, heartbeatRepeat = []time.Duration{20 * time.Millisecond}, 20*time.Millisecond
	defer func() { agentDriver, heartbeatSchedule, heartbeatRepeat = old, oldSched, oldRepeat }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "prompt", "--name", "w", "--store-dir", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	order := []string{"[session info: id=thread-1 agent=codex:gpt-5.6-luna action=start resolved=new]", "name=w", "reconnect: harnez agent resume thread-1", "[wait:", "[message: 0s]\non it", "[heartbeat: 0s, ~", "last: running sleep]", "[message: 0s]\nall done", "[done: 2 messages, last message is the reply"}
	pos := 0
	for _, want := range order {
		i := strings.Index(got[pos:], want)
		if i < 0 {
			t.Fatalf("missing %q (in order) in:\n%s", want, got)
		}
		pos += i + len(want)
	}
	if strings.Contains(got, "[agent messages]") || strings.Contains(got, "[msg ") {
		t.Fatalf("old labels present:\n%s", got)
	}
}

func TestAgentResumeStreamsCompactionAckLabel(t *testing.T) {
	old := agentDriver
	agentDriver = func(subagent.Model) subagent.Driver { return &streamDriver{} }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "luna", Status: "completed", TokensSinceCompact: 150000}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"resume", "--name", "worker", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "[session info: id=sid agent=codex:luna action=resume resolved=name]\n[wait: ") || !strings.Contains(got, "[compact: queued /compact at 150.0k new tokens") || !strings.Contains(got, "[compaction ack: 0s]\non it") || !strings.Contains(got, "[message: 0s]\nall done") {
		t.Fatalf("stdout:\n%s", got)
	}
}

func TestHeartbeatSchedule(t *testing.T) {
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 6 * time.Minute, 10 * time.Minute, 15 * time.Minute, 20 * time.Minute}
	for i, w := range want {
		if got := heartbeatAt(i); got != w {
			t.Fatalf("heartbeatAt(%d) = %s, want %s", i, got, w)
		}
	}
}

func TestAgentStartStatsModeShowsOnlyHeartbeatsAndReply(t *testing.T) {
	old, oldSched, oldRepeat := agentDriver, heartbeatSchedule, heartbeatRepeat
	agentDriver = func(subagent.Model) subagent.Driver { return &streamDriver{} }
	heartbeatSchedule, heartbeatRepeat = []time.Duration{20 * time.Millisecond}, 20*time.Millisecond
	defer func() { agentDriver, heartbeatSchedule, heartbeatRepeat = old, oldSched, oldRepeat }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "prompt", "--stream", "stats", "--store-dir", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, "on it") || strings.Contains(got, "[message:") {
		t.Fatalf("intermediate message leaked in stats mode:\n%s", got)
	}
	for _, want := range []string{"[heartbeat: ", "1 messages, 1 commands", "[reply: 0s]\nall done\n", "[done: 2 messages (only the reply is shown)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestAgentRejectsUnknownStreamMode(t *testing.T) {
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "prompt", "--stream", "loud", "--store-dir", t.TempDir()})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "invalid --stream") {
		t.Fatalf("err = %v", err)
	}
}

func TestShortDur(t *testing.T) {
	for in, want := range map[time.Duration]string{0: "0s", 30 * time.Second: "30s", time.Minute: "1m", 150 * time.Second: "2m30s", time.Hour: "1h", time.Hour + 5*time.Minute: "1h5m"} {
		if got := shortDur(in); got != want {
			t.Fatalf("shortDur(%s) = %q, want %q", in, got, want)
		}
	}
}

func TestTokenSummarySeparatesNewFromCached(t *testing.T) {
	r := &subagent.TurnResult{InputTokens: 1230000, OutputTokens: 1200, CachedTokens: 1198000, TokensTurn: 1231200}
	if got, want := tokenSummary(r), "tokens: 33.2k new (1200 out), 1.2M cached"; got != want {
		t.Fatalf("tokenSummary = %q, want %q", got, want)
	}
	for in, want := range map[int]string{999: "999", 12300: "12.3k", 1230742: "1.2M"} {
		if got := humanCount(in); got != want {
			t.Fatalf("humanCount(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestStreamingResumeKeepsStderrQuiet(t *testing.T) {
	old := agentDriver
	agentDriver = func(subagent.Model) subagent.Driver { return &streamDriver{} }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "luna", Status: "completed", TokensSinceCompact: 150000}); err != nil {
		t.Fatal(err)
	}
	var errOut bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"resume", "--name", "worker", "prompt", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty while streaming", errOut.String())
	}
}

// step is one scripted event, emitted after wait.
type step struct {
	wait time.Duration
	ev   subagent.Event
}

type scriptDriver struct {
	recordingAgentDriver
	steps   []step
	prompt  string
	sid     string   // session id returned by a turn; "thread-1" when empty
	resumed []string // provider ids passed to ResumeStream
	// environment the provider process would inherit during the turn
	envRole, envSession string
}

func (d *scriptDriver) play(fn subagent.EventFunc) *subagent.TurnResult {
	var msgs []string
	for _, s := range d.steps {
		time.Sleep(s.wait)
		fn(s.ev)
		if s.ev.Kind == "message" {
			msgs = append(msgs, s.ev.Text)
		}
	}
	sid := d.sid
	if sid == "" {
		sid = "thread-1"
	}
	return &subagent.TurnResult{SessionID: sid, Response: msgs[len(msgs)-1], Messages: msgs}
}
func (d *scriptDriver) RunStream(_ context.Context, o subagent.RunOptions, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	d.prompt = o.Prompt
	d.envRole, d.envSession = os.Getenv(agentRoleEnv), os.Getenv(agentSessionEnv)
	return d.play(fn), nil
}
func (d *scriptDriver) ResumeStream(_ context.Context, id, p string, fn subagent.EventFunc) (*subagent.TurnResult, error) {
	d.prompt = p
	d.envRole, d.envSession = os.Getenv(agentRoleEnv), os.Getenv(agentSessionEnv)
	d.resumed = append(d.resumed, id)
	return d.play(fn), nil
}

func runScripted(t *testing.T, d *scriptDriver, args ...string) string {
	t.Helper()
	old, oldTimeout := agentDriver, confirmTimeout
	agentDriver = func(subagent.Model) subagent.Driver { return d }
	confirmTimeout = 30 * time.Millisecond
	defer func() { agentDriver, confirmTimeout = old, oldTimeout }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs(append(args, "--store-dir", t.TempDir()))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func msg(text string) subagent.Event  { return subagent.Event{Kind: "message", Text: text, Bytes: 10} }
func tool(text string) subagent.Event { return subagent.Event{Kind: "activity", Text: text, Bytes: 10} }

func TestConfirmationAndPlanShowInStatsMode(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: will count files")}, {0, tool("running ls")}, {0, msg("PLAN: ls then sort")}, {0, msg("interim chatter")}, {0, msg("9 files")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "orig task", "--name", "w", "--stream", "stats")
	for _, want := range []string{"[confirmation: 0s]\nwill count files\n", "[plan: 0s]\nls then sort\n", "[reply: 0s]\n9 files\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "interim chatter") || strings.Contains(got, "violation") || strings.Contains(got, "warning") {
		t.Fatalf("unexpected output:\n%s", got)
	}
}

func TestToolBeforeConfirmationIsAViolation(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, tool("running rg foo")}, {0, msg("CONFIRM: late")}, {0, msg("done")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "task", "--name", "w")
	want := "[violation: running rg foo before any confirmation; caller may stop the agent: harnez agent stop w]"
	if !strings.Contains(got, want) || strings.Count(got, "[violation:") != 1 {
		t.Fatalf("stdout:\n%s", got)
	}
}

func TestSilentAgentGetsWarning(t *testing.T) {
	d := &scriptDriver{steps: []step{{100 * time.Millisecond, msg("late reply")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "task", "--name", "w")
	if !strings.Contains(got, "[warning: no confirmation after 0s; caller may stop the agent: harnez agent stop w]") || strings.Count(got, "[warning:") != 1 {
		t.Fatalf("stdout:\n%s", got)
	}
}

func TestPromptGetsProtocolButStoredPromptStaysOriginal(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
	storeDir := t.TempDir()
	old := agentDriver
	agentDriver = func(subagent.Model) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "--model", "codex:luna", "orig task", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(d.prompt, "Harnez dispatch protocol:") || !strings.HasSuffix(d.prompt, "\n\norig task") || !strings.Contains(d.prompt, "CONFIRM:") {
		t.Fatalf("driver prompt = %q", d.prompt)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sess, err := store.Get("thread-1")
	if err != nil || sess.StartPrompt != "orig task" {
		t.Fatalf("stored prompt = %q, err=%v", sess.StartPrompt, err)
	}
}

func TestPlanFirstAddsGateAndPrompt(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: will plan")}, {0, msg("PLAN: 1) read 2) count")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "count files", "--name", "w", "--plan", "yes")
	if !strings.Contains(d.prompt, "plan-first turn") || !strings.HasSuffix(d.prompt, "\n\ncount files") {
		t.Fatalf("driver prompt = %q", d.prompt)
	}
	for _, want := range []string{"[plan: 0s]\n1) read 2) count\n", `[gate: plan-first turn ended, nothing was executed; to proceed: harnez agent resume w "go ahead"]`, "[done: 2 messages, tokens:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "warning") {
		t.Fatalf("unexpected warning:\n%s", got)
	}
}

func TestPlanFirstWithoutPlanWarns(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("I just did it")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "task", "--name", "w", "--plan", "yes")
	if !strings.Contains(got, "[warning: plan-first turn ended without a PLAN: message; the last message was: I just did it]") {
		t.Fatalf("stdout:\n%s", got)
	}
}

func TestAgentPlanFlagValidationAndCompatibility(t *testing.T) {
	if _, err := runWithStore(t, &scriptDriver{steps: []step{{ev: msg("ok")}}}, t.TempDir(), "start", "--model", "luna", "--plan", "maybe", "task"); err == nil || err.Error() != `invalid --plan "maybe": want "yes" or "no"` {
		t.Fatalf("invalid plan error = %v", err)
	}
	if _, err := runWithStore(t, &scriptDriver{steps: []step{{ev: msg("ok")}}}, t.TempDir(), "start", "--model", "luna", "--plan-first", "task"); err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("old flag error = %v", err)
	}
}

func TestAgentPlanNoEqualsDefault(t *testing.T) {
	for _, args := range [][]string{{"task"}, {"--plan", "no", "task"}} {
		d := &scriptDriver{steps: []step{{ev: msg("ok")}}}
		if _, err := runWithStore(t, d, t.TempDir(), append([]string{"start", "--model", "luna"}, args...)...); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(d.prompt, "plan-first") {
			t.Fatalf("unexpected plan preamble: %q", d.prompt)
		}
	}
}

func TestAgentRootPlanYes(t *testing.T) {
	d := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "root"}}, {ev: msg("CONFIRM: ok")}, {ev: msg("PLAN: wait")}}}
	out, err := runWithStore(t, d, t.TempDir(), "--plan", "yes", "-p", "task")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.prompt, "plan-first turn") || !strings.Contains(out, "[gate: plan-first turn ended") {
		t.Fatalf("plan output=%q prompt=%q", out, d.prompt)
	}
}

func TestAgentHelpUsesNewForms(t *testing.T) {
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "--name") || strings.Contains(out.String(), "start codex:") || strings.Contains(out.String(), "resume <session>") {
		t.Fatalf("root help=%q", out.String())
	}
	start, _, err := newAgentCmd().Find([]string{"start"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(start.Example, "--name") || strings.Contains(start.Example, "start codex:") || strings.Contains(start.Example, "resume <session>") {
		t.Fatalf("start example=%q", start.Example)
	}
}

func TestNormalTurnHasNoPlanFirstText(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
	got := runScripted(t, d, "start", "--model", "codex:luna", "task", "--name", "w")
	if strings.Contains(d.prompt, "plan-first") || strings.Contains(got, "[gate:") {
		t.Fatalf("prompt=%q\nstdout:\n%s", d.prompt, got)
	}
}

// runWithStore runs an agent command against a preloaded store directory.
func runWithStore(t *testing.T, d subagent.Driver, storeDir string, args ...string) (string, error) {
	t.Helper()
	old := agentDriver
	agentDriver = func(subagent.Model) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs(append(args, "--store-dir", storeDir))
	err := cmd.Execute()
	return out.String(), err
}

func saveSessions(t *testing.T, storeDir string, sessions ...*subagent.Session) {
	t.Helper()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sessions {
		if s.Provider == "" {
			s.Provider, s.Model = "codex", "gpt-5.6-luna"
		}
		if s.Status == "" {
			s.Status = "completed"
		}
		if s.WorkingDir == "" {
			s.WorkingDir = "."
		}
		if err := store.Save(s); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAgentResumeWithoutNameResolution(t *testing.T) {
	now := time.Now()
	older := &subagent.Session{ID: "id-old", Name: "old", LastActiveAt: now.Add(-time.Hour)}
	newer := &subagent.Session{ID: "id-new", Name: "new", LastActiveAt: now}
	other := &subagent.Session{ID: "id-else", Name: "elsewhere", WorkingDir: "/somewhere/else", LastActiveAt: now}
	for _, tc := range []struct {
		name     string
		sessions []*subagent.Session
		args     []string
		wantID   string
		header   string
		wantErr  []string
	}{
		{"unique in dir", []*subagent.Session{older, other}, []string{"resume", "go"}, "id-old", "resolved=dir]", nil},
		{"unique with -c", []*subagent.Session{older}, []string{"resume", "-c", "go"}, "id-old", "resolved=continue]", nil},
		{"-c picks most recent", []*subagent.Session{older, newer, other}, []string{"resume", "-c", "go"}, "id-new", "resolved=continue]", nil},
		{"several without -c", []*subagent.Session{older, newer}, []string{"resume", "go"}, "", "", []string{"multiple resumable agents", "old (", "new (", "pass --name"}},
		{"none", []*subagent.Session{other}, []string{"resume", "go"}, "", "", []string{"no resumable agent in .", "harnez agent start --name"}},
		{"none with -c", nil, []string{"resume", "-c", "go"}, "", "", []string{"no resumable agent in ."}},
		{"by name", []*subagent.Session{older, newer}, []string{"resume", "--name", "old", "go"}, "id-old", "resolved=name]", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			storeDir := t.TempDir()
			saveSessions(t, storeDir, tc.sessions...)
			d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
			out, err := runWithStore(t, d, storeDir, tc.args...)
			if tc.wantErr != nil {
				for _, w := range tc.wantErr {
					if err == nil || !strings.Contains(err.Error(), w) {
						t.Fatalf("err = %v, want %q", err, w)
					}
				}
				if len(d.resumed) != 0 {
					t.Fatalf("provider resumed despite error: %v", d.resumed)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(d.resumed) != 1 || d.resumed[0] != tc.wantID || !strings.Contains(out, tc.header) {
				t.Fatalf("resumed=%v want %s; header %q missing in:\n%s", d.resumed, tc.wantID, tc.header, out)
			}
		})
	}
}

func TestAgentDestructiveVerbsRejectContinue(t *testing.T) {
	for _, verb := range []string{"stop", "delete", "compact", "status"} {
		for _, flag := range []string{"-c", "--continue"} {
			_, err := runWithStore(t, &recordingAgentDriver{}, t.TempDir(), verb, flag)
			if err == nil || !strings.Contains(err.Error(), "unknown") {
				t.Fatalf("%s %s: err = %v, want unknown flag", verb, flag, err)
			}
		}
	}
}

func TestAgentStartGeneratesMemorableUniqueNames(t *testing.T) {
	storeDir := t.TempDir()
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		sid := fmt.Sprintf("thread-%d", i)
		d := &scriptDriver{sid: sid, steps: []step{{0, subagent.Event{Kind: "session", Text: sid}}, {0, msg("CONFIRM: ok")}, {0, msg("done")}}}
		out, err := runWithStore(t, d, storeDir, "start", "task")
		if err != nil {
			t.Fatal(err)
		}
		var name string
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "name=") {
				name = strings.Fields(strings.TrimPrefix(line, "name="))[0]
			}
		}
		if name == "" || strings.HasPrefix(name, "agent-") || !strings.Contains(name, "-") || seen[name] {
			t.Fatalf("run %d: generated name %q (seen %v)\n%s", i, name, seen, out)
		}
		seen[name] = true
	}
}

func TestRunStartWithoutCobraFlags(t *testing.T) {
	storeDir := t.TempDir()
	driver := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "direct-start"}}, {ev: msg("CONFIRM: ready")}, {ev: msg("done")}}}
	old := agentDriver
	agentDriver = func(subagent.Model) subagent.Driver { return driver }
	defer func() { agentDriver = old }()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err = runStart(cmd, agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }}, startRequest{Prompt: "direct", StoredPrompt: "direct", ModelSpec: "codex:luna", Dir: ".", StreamMode: streamFull})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "name=") {
		t.Fatalf("stream header missing: %s", out.String())
	}
	sessions, err := store.List("", true)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("saved sessions = %v, err=%v", sessions, err)
	}
}

func TestRunResumeWithoutCobraFlags(t *testing.T) {
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "direct-resume", Name: "direct", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: ".", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	driver := &scriptDriver{steps: []step{{ev: msg("CONFIRM: resumed")}, {ev: msg("done")}}}
	old := agentDriver
	agentDriver = func(subagent.Model) subagent.Driver { return driver }
	defer func() { agentDriver = old }()
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }, find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
		return resolveSession(s, id, "")
	}}
	if err := runResume(cmd, deps, resumeRequest{Prompt: "continue", Name: "direct", Dir: ".", StreamMode: streamFull}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "resolved=name") {
		t.Fatalf("stream header missing: %s", out.String())
	}
	if len(driver.resumed) != 1 || driver.resumed[0] != "direct-resume" {
		t.Fatalf("resumed = %v", driver.resumed)
	}
}

func TestAgentRootPromptStartsGeneratedSession(t *testing.T) {
	d := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "root"}}, {ev: msg("CONFIRM: ok")}, {ev: msg("done")}}}
	out, err := runWithStore(t, d, t.TempDir(), "-p", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "resolved=new") {
		t.Fatalf("output = %q", out)
	}
}

func TestAgentRootPromptModelAlias(t *testing.T) {
	d := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "root"}}, {ev: msg("ok")}}}
	if _, err := runWithStore(t, d, t.TempDir(), "--model", "luna", "-p", "hello"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(d.prompt, "\n\nhello") {
		t.Fatalf("prompt = %q", d.prompt)
	}
}

func TestAgentRootNameUpsert(t *testing.T) {
	for _, exists := range []bool{false, true} {
		t.Run(fmt.Sprintf("exists=%v", exists), func(t *testing.T) {
			d := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "root"}}, {ev: msg("ok")}}}
			dir := t.TempDir()
			if exists {
				saveSessions(t, dir, &subagent.Session{ID: "named", Name: "worker", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: "."})
			}
			args := []string{"--name", "worker", "words"}
			out, err := runWithStore(t, d, dir, args...)
			if err != nil {
				t.Fatal(err)
			}
			want := "resolved=name"
			if !exists {
				want = "resolved=new"
			}
			if !strings.Contains(out, want) {
				t.Fatalf("output = %q", out)
			}
		})
	}
}

func TestAgentRootContinueStartOrResumeMostRecent(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		d := &scriptDriver{steps: []step{{ev: subagent.Event{Kind: "session", Text: "new"}}, {ev: msg("ok")}}}
		out, err := runWithStore(t, d, t.TempDir(), "-c", "-p", "hello")
		if err != nil || !strings.Contains(out, "resolved=new") {
			t.Fatalf("out=%q err=%v", out, err)
		}
	})
	t.Run("resume-most-recent", func(t *testing.T) {
		dir := t.TempDir()
		now := time.Now()
		saveSessions(t, dir, &subagent.Session{ID: "old", Name: "old", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: ".", LastActiveAt: now.Add(-time.Hour)}, &subagent.Session{ID: "new", Name: "new", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: ".", LastActiveAt: now})
		d := &scriptDriver{steps: []step{{ev: msg("ok")}}}
		out, err := runWithStore(t, d, dir, "-c", "-p", "hello")
		if err != nil || !strings.Contains(out, "resolved=continue") || len(d.resumed) != 1 || d.resumed[0] != "new" {
			t.Fatalf("out=%q resumed=%v err=%v", out, d.resumed, err)
		}
	})
}

func TestAgentRootContinueNameConflict(t *testing.T) {
	_, err := runWithStore(t, &scriptDriver{}, t.TempDir(), "-c", "--name", "worker", "-p", "hello")
	if err == nil || err.Error() != "agent: --continue cannot be combined with --name" {
		t.Fatalf("err = %v", err)
	}
}

func TestAgentRootPromptInputs(t *testing.T) {
	file := filepath.Join(t.TempDir(), "prompt.txt")
	if err := os.WriteFile(file, []byte("from file"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-f", file}, {"--", "/x"}} {
		d := &scriptDriver{steps: []step{{ev: msg("ok")}}}
		if _, err := runWithStore(t, d, t.TempDir(), args...); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(d.prompt, strings.TrimPrefix(args[len(args)-1], "/")) && args[0] == "--" {
			t.Fatalf("prompt = %q", d.prompt)
		}
	}
}

func TestAgentRootNoPromptShowsHelp(t *testing.T) {
	out, err := runWithStore(t, &scriptDriver{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Manage subagent sessions") {
		t.Fatalf("help = %q", out)
	}
}

type slashDriver struct{ compacted, stopped []string }

func (d *slashDriver) Run(context.Context, subagent.RunOptions) (*subagent.TurnResult, error) {
	return nil, errors.New("provider Run called")
}
func (d *slashDriver) Resume(context.Context, string, string) (*subagent.TurnResult, error) {
	return nil, errors.New("provider Resume called")
}
func (d *slashDriver) Compact(_ context.Context, id string) (*subagent.TurnResult, error) {
	d.compacted = append(d.compacted, id)
	return &subagent.TurnResult{}, nil
}
func (d *slashDriver) Stop(_ context.Context, id string) error {
	d.stopped = append(d.stopped, id)
	return nil
}
func (d *slashDriver) Delete(context.Context, string) error { return nil }

func TestAgentRootSlashCompactIntercepts(t *testing.T) {
	dir := t.TempDir()
	saveSessions(t, dir, &subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: "."})
	d := &slashDriver{}
	old := agentDriver
	agentDriver = func(subagent.Model) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	if _, err := runWithStore(t, d, dir, "-p", "/compact"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.compacted, []string{"sid"}) {
		t.Fatalf("compacted = %v", d.compacted)
	}
}

func TestAgentRootSlashStatusAndStop(t *testing.T) {
	for _, command := range []string{"/status", "/stop"} {
		t.Run(command, func(t *testing.T) {
			dir := t.TempDir()
			saveSessions(t, dir, &subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: "."})
			d := &slashDriver{}
			old := agentDriver
			agentDriver = func(subagent.Model) subagent.Driver { return d }
			defer func() { agentDriver = old }()
			out, err := runWithStore(t, d, dir, "-p", command)
			if err != nil {
				t.Fatal(err)
			}
			if command == "/status" && !strings.Contains(out, "Status: completed") {
				t.Fatalf("output = %q", out)
			}
			if command == "/stop" && !reflect.DeepEqual(d.stopped, []string{"sid"}) {
				t.Fatalf("stopped = %v", d.stopped)
			}
		})
	}
}

func TestAgentRootSlashErrorsAndLiteral(t *testing.T) {
	// A known command with no session resolves like `resume`; an unknown one
	// is rejected first (see TestAgentRootUnknownSlashIsRejectedBeforeSessionResolution).
	_, err := runWithStore(t, &slashDriver{}, t.TempDir(), "-p", "/status")
	if err == nil || err.Error() != "no resumable agent in .; start one with: harnez agent start --name <name> ..." {
		t.Fatalf("err = %v", err)
	}
	dir := t.TempDir()
	d := &scriptDriver{steps: []step{{ev: msg("ok")}}}
	if _, err := runWithStore(t, d, dir, "--", "/x"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.prompt, "/x") {
		t.Fatalf("prompt = %q", d.prompt)
	}
	dir = t.TempDir()
	saveSessions(t, dir, &subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "gpt-5.6-luna", Tier: "low", WorkingDir: "."})
	_, err = runWithStore(t, &slashDriver{}, dir, "-p", "/x")
	if err == nil || err.Error() != `unknown agent command "/x"; send it literally with: -- /x` {
		t.Fatalf("err = %v", err)
	}
}

func TestAgentRootUnknownSlashIsRejectedBeforeSessionResolution(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
	_, err := runWithStore(t, d, t.TempDir(), "-p", "/foo") // empty store: no session to resolve
	if err == nil || err.Error() != `unknown agent command "/foo"; send it literally with: -- /foo` {
		t.Fatalf("err = %v", err)
	}
	if len(d.resumed) != 0 {
		t.Fatalf("provider resumed: %v", d.resumed)
	}
}

func TestAgentRootSlashRejectsContinueWithName(t *testing.T) {
	storeDir := t.TempDir()
	saveSessions(t, storeDir, &subagent.Session{ID: "id-a", Name: "a"})
	_, err := runWithStore(t, &recordingAgentDriver{}, storeDir, "-p", "/compact", "-c", "--name", "a")
	if err == nil || !strings.Contains(err.Error(), "--continue cannot be combined with --name") {
		t.Fatalf("err = %v", err)
	}
}

func TestStatsModeDoesNotRepeatAnAlreadyShownFinalMessage(t *testing.T) {
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: will do it")}}} // the only message is also the reply
	got := runScripted(t, d, "start", "--model", "luna", "task", "--name", "w", "--stream", "stats")
	if strings.Count(got, "will do it") != 1 || strings.Contains(got, "CONFIRM:") {
		t.Fatalf("final message repeated or raw tag shown:\n%s", got)
	}
	if !strings.Contains(got, "[reply: 0s]\n(the final message was already shown above)\n") {
		t.Fatalf("missing reply marker:\n%s", got)
	}
}

func TestAgentRoleStoredEnvAndPreamble(t *testing.T) {
	t.Setenv(agentRoleEnv, "")
	t.Setenv(agentSessionEnv, "")
	storeDir := t.TempDir()
	d := &scriptDriver{steps: []step{{0, subagent.Event{Kind: "session", Text: "thread-1"}}, {0, msg("CONFIRM: ok")}, {0, msg("done")}}}
	if _, err := runWithStore(t, d, storeDir, "start", "--name", "boss", "--role", "orchestrator", "run the sprint"); err != nil {
		t.Fatal(err)
	}
	if d.envRole != "orchestrator" || d.envSession != "boss" {
		t.Fatalf("child env role=%q session=%q", d.envRole, d.envSession)
	}
	if !strings.Contains(d.prompt, "Harnez role: orchestrator. You coordinate; you never write code") {
		t.Fatalf("preamble lacks orchestrator rules:\n%s", d.prompt)
	}
	if os.Getenv(agentRoleEnv) != "" || os.Getenv(agentSessionEnv) != "" {
		t.Fatal("agent environment leaked out of the command")
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sess, err := store.Get("thread-1")
	if err != nil || sess.Role != "orchestrator" {
		t.Fatalf("stored role = %q, err=%v", sess.Role, err)
	}
	// resume keeps the stored role and rejects a conflicting one
	if _, err := runWithStore(t, d, storeDir, "resume", "--name", "boss", "go on"); err != nil {
		t.Fatal(err)
	}
	if d.envRole != "orchestrator" || !strings.Contains(d.prompt, "Harnez role: orchestrator.") {
		t.Fatalf("resume env role=%q prompt:\n%s", d.envRole, d.prompt)
	}
	if _, err := runWithStore(t, d, storeDir, "resume", "--name", "boss", "--role", "developer", "x"); err == nil || !strings.Contains(err.Error(), "conflicts with session") {
		t.Fatalf("conflicting --role err = %v", err)
	}
}

func TestAgentStartDefaultsToDeveloperRole(t *testing.T) {
	t.Setenv(agentRoleEnv, "")
	d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
	if _, err := runWithStore(t, d, t.TempDir(), "start", "task"); err != nil {
		t.Fatal(err)
	}
	if d.envRole != "developer" || !strings.Contains(d.prompt, "You are a leaf worker") || !strings.Contains(d.prompt, "Never run `harnez agent`") {
		t.Fatalf("env=%q prompt:\n%s", d.envRole, d.prompt)
	}
	if _, err := runWithStore(t, d, t.TempDir(), "start", "--role", "wizard", "task"); err == nil || !strings.Contains(err.Error(), "unknown agent role") {
		t.Fatalf("unknown role err = %v", err)
	}
}

func TestLeafRolesCannotStartOrManageAgents(t *testing.T) {
	storeDir := t.TempDir()
	saveSessions(t, storeDir, &subagent.Session{ID: "id-a", Name: "a"})
	for _, role := range []string{"developer", "reviewer", "advisor"} {
		t.Setenv(agentRoleEnv, role)
		for _, args := range [][]string{
			{"start", "task"}, {"resume", "--name", "a", "x"}, {"-p", "hello"}, {"--name", "a", "hello"}, {"-p", "/stop", "--name", "a"},
			{"compact", "--name", "a"}, {"stop", "--name", "a"}, {"delete", "--name", "a"}, {"chat"}, {"enable"},
		} {
			d := &scriptDriver{steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
			_, err := runWithStore(t, d, storeDir, args...)
			if err == nil || !strings.Contains(err.Error(), "is a leaf worker") {
				t.Fatalf("%s %v: err = %v, want leaf-worker refusal", role, args, err)
			}
			if len(d.resumed) != 0 || d.prompt != "" {
				t.Fatalf("%s %v reached the provider", role, args)
			}
		}
		for _, args := range [][]string{{"list"}, {"status"}, {"status", "--name", "a"}, {"models"}} {
			if _, err := runWithStore(t, &recordingAgentDriver{}, storeDir, args...); err != nil {
				t.Fatalf("%s %v must stay available: %v", role, args, err)
			}
		}
	}
}

func TestOrchestratorMayStartHelpersButNotOrchestrators(t *testing.T) {
	t.Setenv(agentRoleEnv, "orchestrator")
	t.Setenv(agentSessionEnv, "boss")
	storeDir := t.TempDir()
	for _, role := range []string{"", "developer", "reviewer", "advisor"} {
		d := &scriptDriver{sid: "thread-" + role, steps: []step{{0, msg("CONFIRM: ok")}, {0, msg("done")}}}
		args := []string{"start", "task"}
		if role != "" {
			args = append(args, "--role", role)
		}
		if _, err := runWithStore(t, d, storeDir, args...); err != nil {
			t.Fatalf("role %q: %v", role, err)
		}
		if d.envSession == "boss" {
			t.Fatal("helper must get its own session name as parent id, not the orchestrator's")
		}
	}
	d := &scriptDriver{steps: []step{{0, msg("done")}}}
	_, err := runWithStore(t, d, storeDir, "start", "--role", "orchestrator", "task")
	if err == nil || !strings.Contains(err.Error(), "may only start developer, reviewer, advisor sessions") || d.prompt != "" {
		t.Fatalf("nested orchestrator err = %v", err)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sessions, _ := store.List("", true)
	for _, s := range sessions {
		if s.ParentSessionID != "boss" {
			t.Fatalf("session %s parent = %q, want boss", s.Name, s.ParentSessionID)
		}
	}
}

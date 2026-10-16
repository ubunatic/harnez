package subagent

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"ubunatic.com/harnez/internal/claude"
)

func TestInteractiveCommand(t *testing.T) {
	base := InteractiveOptions{SessionID: "registry-id", Name: "calm-otter", Dir: "/work"}
	for _, tc := range []struct {
		name       string
		opts       InteractiveOptions
		providerID string
		command    string
		args       []string
	}{
		{"codex chat", withInteractiveModel(base, Model{Provider: "codex", Name: "gpt-5.6-luna"}), "", "codex", []string{"-m", "gpt-5.6-luna", "-C", "/work"}},
		{"codex attach", withInteractiveModel(base, Model{Provider: "codex", Name: "gpt-5.6-luna"}), "thread-id", "codex", []string{"resume", "thread-id", "-m", "gpt-5.6-luna", "-C", "/work"}},
		{"claude chat", withInteractiveModel(base, Model{Provider: "claude", Name: "haiku"}), "", "claude", []string{"--model", "haiku", "--name", "calm-otter", "--session-id", "registry-id"}},
		{"claude attach", withInteractiveModel(base, Model{Provider: "claude", Name: "haiku"}), "registry-id", "claude", []string{"--resume", "registry-id", "--model", "haiku"}},
		{"agy chat", withInteractiveModel(base, Model{Provider: "agy", Name: "gemini-3.7-flash", Tier: "low"}), "", "agy", []string{"--model", "gemini-3.7-flash", "--effort", "low"}},
		{"codex chat prompt", withInteractivePrompt(withInteractiveModel(base, Model{Provider: "codex", Name: "gpt-5.6-luna"}), "opening prompt"), "", "codex", []string{"-m", "gpt-5.6-luna", "-C", "/work", "opening prompt"}},
		{"claude chat prompt", withInteractivePrompt(withInteractiveModel(base, Model{Provider: "claude", Name: "haiku"}), "opening prompt"), "", "claude", []string{"--model", "haiku", "--name", "calm-otter", "--session-id", "registry-id", "--", "opening prompt"}},
		{"agy chat prompt", withInteractivePrompt(withInteractiveModel(base, Model{Provider: "agy", Name: "gemini-3.7-flash", Tier: "low"}), "opening prompt"), "", "agy", []string{"--model", "gemini-3.7-flash", "--prompt-interactive", "opening prompt", "--effort", "low"}},
		{"codex attach prompt", withInteractivePrompt(withInteractiveModel(base, Model{Provider: "codex", Name: "gpt-5.6-luna"}), "next prompt"), "thread-id", "codex", []string{"resume", "thread-id", "-m", "gpt-5.6-luna", "-C", "/work", "next prompt"}},
		{"claude attach prompt", withInteractivePrompt(withInteractiveModel(base, Model{Provider: "claude", Name: "haiku"}), "next prompt"), "registry-id", "claude", []string{"--resume", "registry-id", "--model", "haiku", "--", "next prompt"}},
		{"agy attach prompt", withInteractivePrompt(withInteractiveModel(base, Model{Provider: "agy", Name: "gemini-3.7-flash", Tier: "low"}), "next prompt"), "conversation-id", "agy", []string{"--conversation", "conversation-id", "--model", "gemini-3.7-flash", "--prompt-interactive", "next prompt", "--effort", "low"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command, args, err := interactiveCommand(tc.opts, tc.providerID)
			if err != nil || command != tc.command || !reflect.DeepEqual(args, tc.args) {
				t.Fatalf("interactiveCommand = %q %#v, %v; want %q %#v", command, args, err, tc.command, tc.args)
			}
		})
	}
}

func withInteractivePrompt(opts InteractiveOptions, prompt string) InteractiveOptions {
	opts.Prompt = prompt
	return opts
}

func withInteractiveModel(opts InteractiveOptions, model Model) InteractiveOptions {
	opts.Model = model
	return opts
}

func TestInteractiveCommandUnsupportedProvider(t *testing.T) {
	_, _, err := interactiveCommand(InteractiveOptions{Model: Model{Provider: "local"}}, "")
	if err == nil {
		t.Fatal("expected unsupported provider error")
	}
}

func TestAgyInteractiveLaunchEnvironmentInstallsShimAndSetsPath(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	original := []string{"PATH=" + binDir + string(os.PathListSeparator) + "/usr/bin:/bin", "ANTIGRAVITY_AGENT=0"}
	environ, err := agyInteractiveLaunchEnv(original, home)
	if err != nil {
		t.Fatal(err)
	}
	if got := environmentValue(environ, "ANTIGRAVITY_AGENT"); got != "1" {
		t.Fatalf("ANTIGRAVITY_AGENT = %q, want 1", got)
	}
	shimDir := filepath.Join(home, ".harnez", "shims")
	paths := filepath.SplitList(environmentValue(environ, "PATH"))
	if len(paths) < 2 || paths[0] != shimDir || paths[1] != binDir {
		t.Fatalf("agy PATH = %#v, want shim first and original PATH after", paths)
	}
	shim, err := os.ReadFile(filepath.Join(shimDir, "bash"))
	if err != nil {
		t.Fatal(err)
	}
	if string(shim) != claude.BashShimContent {
		t.Fatalf("installed shim content does not match managed bash shim")
	}
}

func TestInteractiveAgyUsesMeterEnvironmentThroughPTY(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	agy := filepath.Join(bin, "agy")
	if err := os.WriteFile(agy, []byte("#!/bin/sh\nprintf '%s\\n' \"$HARNEZ_SESSION_ID\" \"$HARNEZ_AGY_METER_SESSION_ID\" \"$ANTIGRAVITY_AGENT\" \"$HTTPS_PROXY\" \"$SSL_CERT_FILE\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin:/bin")
	var stdout bytes.Buffer
	opts := InteractiveOptions{
		Model:         Model{Provider: "agy", Name: "gemini-3.7-flash"},
		SessionID:     "harnez-session",
		Dir:           home,
		Stdin:         bytes.NewBuffer(nil),
		Stdout:        &stdout,
		Stderr:        &bytes.Buffer{},
		ControlSocket: filepath.Join(home, "control.sock"),
	}
	if err := runInteractiveCommand(context.Background(), "agy", nil, opts); err != nil {
		t.Fatalf("run interactive agy: %v", err)
	}
	for _, want := range []string{"harnez-session", "1", "http://127.0.0.1:", ".harnez/agymeter/roots.pem"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("PTY output %q does not contain %q", stdout.String(), want)
		}
	}
}

func TestControlBrokerPromptCompactAndStop(t *testing.T) {
	path := t.TempDir() + "/control.sock"
	listener, err := listenControl(path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var terminal bytes.Buffer
	go serveControls(listener, &terminal, cmd.Process, nil, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := SendControl(ctx, path, "prompt", "review this"); err != nil {
		t.Fatal(err)
	}
	if err := SendControl(ctx, path, "compact", ""); err != nil {
		t.Fatal(err)
	}
	if got := terminal.String(); got != "review this\n/compact\n" {
		t.Fatalf("terminal input = %q", got)
	}
	if err := SendControl(ctx, path, "stop", ""); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("stop control did not terminate owned process")
	}
}

func TestControlPromptWritesToPTYAndSplitsMultilineInput(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := listenControl(path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go serveControls(listener, master, nil, nil, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	const prompt = "first line\nsecond line\nthird line"
	if err := SendControl(ctx, path, "prompt", prompt); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(prompt)+1)
	if _, err := io.ReadFull(slave, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != prompt+"\n" {
		t.Fatalf("PTY input = %q, want prompt bytes followed by Enter", got)
	}
}

func TestControlPromptDoesNotUseBracketedPasteFraming(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := listenControl(path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go serveControls(listener, master, nil, nil, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	const prompt = "one\ntwo"
	if err := SendControl(ctx, path, "prompt", prompt); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(prompt)+1)
	if _, err := io.ReadFull(slave, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != prompt+"\n" {
		t.Fatalf("PTY input = %q, want raw newline-delimited input without bracketed-paste markers", got)
	}
}

func TestSendControlRejectsUnknownAction(t *testing.T) {
	path := t.TempDir() + "/control.sock"
	listener, err := listenControl(path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go serveControls(listener, io.Discard, nil, nil, nil, nil)
	if err := SendControl(context.Background(), path, "unknown", ""); err == nil {
		t.Fatal("expected unknown control action error")
	}
}

func TestControlBrokerInboxAndTurnActions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := listenControl(path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	inbox, err := OpenInbox(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	turn := NewTurnState(nil)
	go serveControls(listener, io.Discard, nil, inbox, turn, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	queued, err := SendControlRequest(ctx, path, ControlRequest{Action: "enqueue", Sender: "sender-1", Prompt: "hello"})
	if err != nil || queued.MessageID == "" || queued.Queued != 1 {
		t.Fatalf("enqueue response = %#v, err=%v", queued, err)
	}
	listed, err := SendControlRequest(ctx, path, ControlRequest{Action: "inbox"})
	if err != nil || len(listed.Messages) != 1 || listed.Messages[0].ID != queued.MessageID {
		t.Fatalf("inbox response = %#v, err=%v", listed, err)
	}
	if _, err := SendControlRequest(ctx, path, ControlRequest{Action: "turn", Event: "tool-start"}); err != nil {
		t.Fatal(err)
	}
	if turn.Idle(time.Now().Add(time.Hour), 0, 0) {
		t.Fatal("tool-start did not mark turn busy")
	}
	if _, err := SendControlRequest(ctx, path, ControlRequest{Action: "turn", Event: "tool-end"}); err != nil {
		t.Fatal(err)
	}
}

func TestDeliverInboxWaitsForIdleAndWritesFIFOBracketedPaste(t *testing.T) {
	inbox, err := OpenInbox(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inbox.Enqueue("sender", "one\ntwo"); err != nil {
		t.Fatal(err)
	}
	if _, err := inbox.Enqueue("sender", "second"); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	turn := NewTurnState(func() time.Time { return now })
	turn.ToolStarted()
	var terminal lockedBuffer
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		deliverInbox(ctx, inbox, turn, &terminal, nil, 0, 0)
	}()
	inbox.signal()
	time.Sleep(30 * time.Millisecond)
	if terminal.Len() != 0 {
		t.Fatalf("busy turn wrote %q", terminal.String())
	}
	turn.ToolFinished()
	want := "\x1b[200~one\ntwo\x1b[201~\r\x1b[200~second\x1b[201~\r"
	// Wait for both messages: they are written one after the other.
	deadline := time.Now().Add(2 * time.Second)
	for terminal.Len() < len(want) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := terminal.String(); got != want {
		t.Fatalf("delivery = %q, want %q", got, want)
	}
	cancel()
	<-done
	if got := inbox.Depth(); got != 0 {
		t.Fatalf("remaining inbox depth = %d", got)
	}
}

// lockedBuffer is a bytes.Buffer safe for a writer goroutine and a reading test.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

package subagent

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"reflect"
	"testing"
	"time"
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			command, args, err := interactiveCommand(tc.opts, tc.providerID)
			if err != nil || command != tc.command || !reflect.DeepEqual(args, tc.args) {
				t.Fatalf("interactiveCommand = %q %#v, %v; want %q %#v", command, args, err, tc.command, tc.args)
			}
		})
	}
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
	go serveControls(listener, &terminal, cmd.Process)
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

func TestSendControlRejectsUnknownAction(t *testing.T) {
	path := t.TempDir() + "/control.sock"
	listener, err := listenControl(path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go serveControls(listener, io.Discard, nil)
	if err := SendControl(context.Background(), path, "unknown", ""); err == nil {
		t.Fatal("expected unknown control action error")
	}
}

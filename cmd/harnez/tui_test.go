package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/creack/pty"
)

func TestTUICmdRender(t *testing.T) {
	cmd := newRootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"tui", "--render", "--model", "claude:sonnet", "--session", "test-tui-session", "--cols", "80", "--rows", "24"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("harnez tui --render error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "harnez tui") || !strings.Contains(out, "claude:sonnet") {
		t.Errorf("expected header in tui output, got:\n%s", out)
	}
	if !strings.Contains(out, "─") {
		t.Errorf("expected divider line in tui output")
	}
	if !strings.Contains(out, "> ") {
		t.Errorf("expected prompt symbol '> ' in tui output")
	}
}

func TestTUICmdRealTerminalPTY(t *testing.T) {
	if testing.Short() {
		t.Skip("real PTY: full test tier")
	}
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open error: %v", err)
	}
	defer master.Close()

	cmd := newRootCmd()
	cmd.SetOut(slave)
	cmd.SetErr(slave)
	cmd.SetArgs([]string{"tui", "--render", "--model", "codex:luna", "--session", "tui-design-001"})

	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Execute()
		_ = slave.Close()
	}()

	outBytes, readErr := io.ReadAll(master)
	if readErr != nil && readErr != io.EOF && !os.IsNotExist(readErr) && !strings.Contains(readErr.Error(), "input/output error") {
		t.Fatalf("read master pty error: %v", readErr)
	}

	if execErr := <-errCh; execErr != nil {
		t.Fatalf("cmd.Execute error: %v", execErr)
	}

	output := string(outBytes)
	if !strings.Contains(output, "harnez tui") {
		t.Errorf("PTY output missing 'harnez tui', got:\n%s", output)
	}
	if !strings.Contains(output, "codex:luna") {
		t.Errorf("PTY output missing 'codex:luna', got:\n%s", output)
	}
	if !strings.Contains(output, "> ") {
		t.Errorf("PTY output missing prompt symbol '> ', got:\n%s", output)
	}
}

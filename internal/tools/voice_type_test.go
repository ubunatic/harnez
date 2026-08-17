package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestBuildDotoolCommands(t *testing.T) {
	t.Run("single line without delay", func(t *testing.T) {
		got := BuildDotoolCommands("hello world", 0)
		want := "type hello world\n"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("single line with delay", func(t *testing.T) {
		got := BuildDotoolCommands("hello", 15)
		want := "typedelay 15\ntypehold 15\ntype hello\n"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("multiple lines with enter keys", func(t *testing.T) {
		got := BuildDotoolCommands("line1\nline2\nline3", 0)
		want := "type line1\nkey enter\ntype line2\nkey enter\ntype line3\n"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

func TestDotoolPipePath(t *testing.T) {
	t.Run("default pipe path", func(t *testing.T) {
		got := dotoolPipePath(func(string) string { return "" })
		if got != "/tmp/dotool-pipe" {
			t.Fatalf("got %q, want /tmp/dotool-pipe", got)
		}
	})

	t.Run("custom pipe path from env", func(t *testing.T) {
		got := dotoolPipePath(func(k string) string {
			if k == "DOTOOL_PIPE" {
				return "/custom/fifo"
			}
			return ""
		})
		if got != "/custom/fifo" {
			t.Fatalf("got %q, want /custom/fifo", got)
		}
	})
}

func TestCopyText(t *testing.T) {
	t.Run("missing wl-copy returns error", func(t *testing.T) {
		d := testDeps(nil)
		d.LookPath = func(string) (string, error) { return "", errors.New("missing") }
		err := CopyText(context.Background(), d, "copied text")
		if err == nil || !strings.Contains(err.Error(), "wl-copy not found") {
			t.Fatalf("expected missing wl-copy error, got %v", err)
		}
	})

	t.Run("successful copy passes text to wl-copy stdin", func(t *testing.T) {
		d := testDeps(nil)
		d.LookPath = func(name string) (string, error) {
			if name == "wl-copy" {
				return "/usr/bin/wl-copy", nil
			}
			return "", errors.New("missing")
		}
		var capturedStdin, capturedCmd string
		d.RunStdin = func(_ context.Context, stdin, cmd string, args ...string) error {
			capturedStdin = stdin
			capturedCmd = cmd
			return nil
		}
		if err := CopyText(context.Background(), d, "hello clipboard"); err != nil {
			t.Fatalf("CopyText failed: %v", err)
		}
		if capturedCmd != "wl-copy" || capturedStdin != "hello clipboard" {
			t.Fatalf("unexpected call: cmd=%q, stdin=%q", capturedCmd, capturedStdin)
		}
	})
}

func TestTypeText(t *testing.T) {
	t.Run("empty text is no-op", func(t *testing.T) {
		d := testDeps(nil)
		if err := TypeText(context.Background(), d, ""); err != nil {
			t.Fatalf("expected no error on empty text: %v", err)
		}
	})

	t.Run("cold path runs dotool with config type delay", func(t *testing.T) {
		dir := t.TempDir()
		voxtypeDir := filepath.Join(dir, ".config", "voxtype")
		if err := os.MkdirAll(voxtypeDir, 0755); err != nil {
			t.Fatal(err)
		}
		configContent := "[output]\ntype_delay_ms = 8\n"
		if err := os.WriteFile(filepath.Join(voxtypeDir, "config.toml"), []byte(configContent), 0644); err != nil {
			t.Fatal(err)
		}

		d := testDeps(nil)
		d.Getenv = func(k string) string {
			if k == "HOME" {
				return dir
			}
			if k == "DOTOOL_PIPE" {
				return filepath.Join(dir, "nonexistent-pipe")
			}
			return ""
		}
		d.LookPath = func(name string) (string, error) {
			if name == "dotool" {
				return "/usr/bin/dotool", nil
			}
			return "", errors.New("missing")
		}
		var ranCmd, ranStdin string
		d.RunStdin = func(_ context.Context, stdin, cmd string, args ...string) error {
			ranCmd = cmd
			ranStdin = stdin
			return nil
		}

		if err := TypeText(context.Background(), d, "hello"); err != nil {
			t.Fatalf("TypeText: %v", err)
		}
		if ranCmd != "dotool" {
			t.Fatalf("expected dotool command, got %q", ranCmd)
		}
		if !strings.Contains(ranStdin, "typedelay 8\n") || !strings.Contains(ranStdin, "type hello\n") {
			t.Fatalf("unexpected stdin: %q", ranStdin)
		}
	})

	t.Run("daemon ready path runs dotoolc and falls back on error", func(t *testing.T) {
		dir := t.TempDir()
		pipePath := filepath.Join(dir, "test-pipe")
		if err := syscall.Mkfifo(pipePath, 0600); err != nil {
			t.Fatal(err)
		}
		// Open reader end in background so non-blocking open succeeds
		r, err := os.OpenFile(pipePath, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()

		d := testDeps(nil)
		d.Getenv = func(k string) string {
			if k == "DOTOOL_PIPE" {
				return pipePath
			}
			return ""
		}
		d.LookPath = func(name string) (string, error) {
			return "/usr/bin/" + name, nil
		}

		// First try: dotoolc succeeds
		var calls []string
		d.RunStdin = func(_ context.Context, stdin, cmd string, args ...string) error {
			calls = append(calls, cmd)
			return nil
		}
		if err := TypeText(context.Background(), d, "fast"); err != nil {
			t.Fatalf("TypeText fast path: %v", err)
		}
		if len(calls) != 1 || calls[0] != "dotoolc" {
			t.Fatalf("expected [dotoolc], got %v", calls)
		}

		// Second try: dotoolc fails, falls back to dotool
		calls = nil
		d.RunStdin = func(_ context.Context, stdin, cmd string, args ...string) error {
			calls = append(calls, cmd)
			if cmd == "dotoolc" {
				return errors.New("dotoolc failed")
			}
			return nil
		}
		if err := TypeText(context.Background(), d, "fallback"); err != nil {
			t.Fatalf("TypeText fallback: %v", err)
		}
		if len(calls) != 2 || calls[0] != "dotoolc" || calls[1] != "dotool" {
			t.Fatalf("expected [dotoolc, dotool], got %v", calls)
		}
	})
}

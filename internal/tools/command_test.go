package tools

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"ubunatic.com/harnez"
)

func TestStatusExitCode(t *testing.T) {
	spec, err := harnez.DefaultFS.ReadFile("spec/tools/voice-input.yaml")
	if err != nil {
		t.Fatal(err)
	}
	pendingSpec := []byte(strings.Replace(string(spec), "canary_verified: true", "canary_verified: false", 1))
	verifiedSpec := []byte(strings.Replace(string(spec), "canary_verified: false", "canary_verified: true", 1))
	fsys := fstest.MapFS{"spec/tools/voice-input.yaml": {Data: pendingSpec}}

	t.Run("bare listing succeeds when not ready", func(t *testing.T) {
		var out bytes.Buffer
		cmd, err := NewCommand(fsys, testDeps(&out))
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs(nil)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("bare tools: %v", err)
		}
	})

	t.Run("specific status fails when not ready", func(t *testing.T) {
		var out bytes.Buffer
		cmd, err := NewCommand(fsys, testDeps(&out))
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"status", "voice-input"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected nonzero not-ready status")
		}
		if !strings.Contains(out.String(), "unsupported") {
			t.Fatal(out.String())
		}
	})

	t.Run("specific ready status succeeds", func(t *testing.T) {
		readyFS := fstest.MapFS{"spec/tools/voice-input.yaml": {Data: verifiedSpec}}
		var out bytes.Buffer
		d := testDeps(&out)
		d.LookPath = func(string) (string, error) { return "/bin/tool", nil }
		d.Run = func(context.Context, string, ...string) error { return nil }
		cmd, err := NewCommand(readyFS, d)
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"status", "voice-input"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("ready status: %v", err)
		}
	})
}

func TestVoiceInputModeCommand(t *testing.T) {
	spec, err := harnez.DefaultFS.ReadFile("spec/tools/voice-input.yaml")
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{"spec/tools/voice-input.yaml": {Data: spec}}

	t.Run("bare mode reports current state", func(t *testing.T) {
		var out bytes.Buffer
		d := testDeps(&out)
		d.Run = func(context.Context, string, ...string) error { return errors.New("inactive") }
		cmd, err := NewCommand(fsys, d)
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"voice-input", "mode"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("bare mode: %v", err)
		}
		if !strings.Contains(out.String(), "neither") {
			t.Fatal(out.String())
		}
	})

	t.Run("mode streaming rejects a missing config", func(t *testing.T) {
		var out bytes.Buffer
		d := testDeps(&out)
		d.Run = func(context.Context, string, ...string) error { return errors.New("inactive") }
		cmd, err := NewCommand(fsys, d)
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"voice-input", "mode", "streaming"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "streaming config missing") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("mode rejects an unknown argument", func(t *testing.T) {
		var out bytes.Buffer
		cmd, err := NewCommand(fsys, testDeps(&out))
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"voice-input", "mode", "bogus"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "invalid mode") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestVoiceInputHistoryCommands(t *testing.T) {
	spec, err := harnez.DefaultFS.ReadFile("spec/tools/voice-input.yaml")
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{"spec/tools/voice-input.yaml": {Data: spec}}

	dir := t.TempDir()
	homeDir := dir

	t.Run("record echoes stdin and saves to history", func(t *testing.T) {
		var out bytes.Buffer
		d := testDeps(&out)
		d.Getenv = func(k string) string {
			if k == "HOME" {
				return homeDir
			}
			return ""
		}
		d.Stdin = strings.NewReader("dictated text to record\n")

		cmd, err := NewCommand(fsys, d)
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"voice-input", "history", "record"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("history record failed: %v", err)
		}
		if out.String() != "dictated text to record\n" {
			t.Fatalf("expected stdout echo, got %q", out.String())
		}

		entries, err := ListHistory(HistoryPath(homeDir))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Text != "dictated text to record" {
			t.Fatalf("unexpected entries: %+v", entries)
		}
	})

	t.Run("list prints history entries", func(t *testing.T) {
		var out bytes.Buffer
		d := testDeps(&out)
		d.Getenv = func(k string) string {
			if k == "HOME" {
				return homeDir
			}
			return ""
		}

		cmd, err := NewCommand(fsys, d)
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"voice-input", "history", "list"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("history list failed: %v", err)
		}
		if !strings.Contains(out.String(), "dictated text to record") {
			t.Fatalf("expected list output to contain entry text, got: %s", out.String())
		}
	})

	t.Run("list format json output", func(t *testing.T) {
		var out bytes.Buffer
		d := testDeps(&out)
		d.Getenv = func(k string) string {
			if k == "HOME" {
				return homeDir
			}
			return ""
		}

		cmd, err := NewCommand(fsys, d)
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"voice-input", "history", "list", "--format", "json"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("history list json failed: %v", err)
		}
		if !strings.Contains(out.String(), `"text": "dictated text to record"`) {
			t.Fatalf("expected json output to contain entry text, got: %s", out.String())
		}
	})

	t.Run("copy copies entry text via wl-copy", func(t *testing.T) {
		entries, err := ListHistory(HistoryPath(homeDir))
		if err != nil || len(entries) == 0 {
			t.Fatalf("no history entries: %v", err)
		}
		targetID := entries[0].ID

		var out bytes.Buffer
		d := testDeps(&out)
		d.Getenv = func(k string) string {
			if k == "HOME" {
				return homeDir
			}
			return ""
		}
		d.LookPath = func(name string) (string, error) {
			if name == "wl-copy" {
				return "/usr/bin/wl-copy", nil
			}
			return "", errors.New("missing")
		}
		var copiedStdin string
		d.RunStdin = func(_ context.Context, stdin, cmd string, args ...string) error {
			if cmd == "wl-copy" {
				copiedStdin = stdin
				return nil
			}
			return errors.New("unexpected cmd")
		}

		cmd, err := NewCommand(fsys, d)
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"voice-input", "history", "copy", targetID})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("history copy failed: %v", err)
		}
		if copiedStdin != "dictated text to record" {
			t.Fatalf("expected 'dictated text to record', got %q", copiedStdin)
		}
	})

	t.Run("retype types entry text via dotool", func(t *testing.T) {
		entries, err := ListHistory(HistoryPath(homeDir))
		if err != nil || len(entries) == 0 {
			t.Fatalf("no history entries: %v", err)
		}
		targetID := entries[0].ID

		var out bytes.Buffer
		d := testDeps(&out)
		d.Getenv = func(k string) string {
			if k == "HOME" {
				return homeDir
			}
			return ""
		}
		d.LookPath = func(name string) (string, error) {
			if name == "dotool" {
				return "/usr/bin/dotool", nil
			}
			return "", errors.New("missing")
		}
		var typedStdin string
		d.RunStdin = func(_ context.Context, stdin, cmd string, args ...string) error {
			if cmd == "dotool" {
				typedStdin = stdin
				return nil
			}
			return errors.New("unexpected cmd")
		}

		cmd, err := NewCommand(fsys, d)
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"voice-input", "history", "retype", targetID})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("history retype failed: %v", err)
		}
		if !strings.Contains(typedStdin, "type dictated text to record\n") {
			t.Fatalf("unexpected typed stdin: %q", typedStdin)
		}
	})

	t.Run("clear removes all history", func(t *testing.T) {
		var out bytes.Buffer
		d := testDeps(&out)
		d.Getenv = func(k string) string {
			if k == "HOME" {
				return homeDir
			}
			return ""
		}

		cmd, err := NewCommand(fsys, d)
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"voice-input", "history", "clear"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("history clear failed: %v", err)
		}
		entries, err := ListHistory(HistoryPath(homeDir))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("expected empty history after clear, got %d", len(entries))
		}
	})
}

func TestVoiceInputConfigCommands(t *testing.T) {
	spec, err := harnez.DefaultFS.ReadFile("spec/tools/voice-input.yaml")
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{"spec/tools/voice-input.yaml": {Data: spec}}

	dir := t.TempDir()
	homeDir := dir
	voxtypeDir := filepath.Join(homeDir, ".config", "voxtype")
	if err := os.MkdirAll(voxtypeDir, 0755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(voxtypeDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("[output]\ndriver = \"dotool\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("config get reports default when key absent", func(t *testing.T) {
		var out bytes.Buffer
		d := testDeps(&out)
		d.Getenv = func(k string) string {
			if k == "HOME" {
				return homeDir
			}
			return ""
		}

		cmd, err := NewCommand(fsys, d)
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"voice-input", "config", "get", "type-delay-ms"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("config get: %v", err)
		}
		if !strings.Contains(out.String(), "0 (default") {
			t.Fatalf("expected default message, got: %s", out.String())
		}
	})

	t.Run("config set updates value and explains restart requirement", func(t *testing.T) {
		var out bytes.Buffer
		d := testDeps(&out)
		d.Getenv = func(k string) string {
			if k == "HOME" {
				return homeDir
			}
			return ""
		}

		cmd, err := NewCommand(fsys, d)
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"voice-input", "config", "set", "type-delay-ms", "18"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("config set: %v", err)
		}
		if !strings.Contains(out.String(), "type_delay_ms set to 18") || !strings.Contains(out.String(), "systemctl --user restart") {
			t.Fatalf("unexpected output: %s", out.String())
		}

		val, ok, err := ReadTypeDelayMs(configPath)
		if err != nil || !ok || val != 18 {
			t.Fatalf("expected 18 in config, got ok=%v, val=%d, err=%v", ok, val, err)
		}
	})

	t.Run("config get returns updated value", func(t *testing.T) {
		var out bytes.Buffer
		d := testDeps(&out)
		d.Getenv = func(k string) string {
			if k == "HOME" {
				return homeDir
			}
			return ""
		}

		cmd, err := NewCommand(fsys, d)
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"voice-input", "config", "get", "type-delay-ms"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("config get: %v", err)
		}
		if strings.TrimSpace(out.String()) != "18" {
			t.Fatalf("expected '18', got %q", out.String())
		}
	})

	t.Run("config rejects invalid keys or values", func(t *testing.T) {
		var out bytes.Buffer
		d := testDeps(&out)
		d.Getenv = func(k string) string {
			if k == "HOME" {
				return homeDir
			}
			return ""
		}

		cmd, err := NewCommand(fsys, d)
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"voice-input", "config", "get", "bad-key"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown config key") {
			t.Fatalf("expected error on bad key get, got: %v", err)
		}

		cmd.SetArgs([]string{"voice-input", "config", "set", "type-delay-ms", "not-a-number"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "invalid milliseconds") {
			t.Fatalf("expected error on non-numeric set, got: %v", err)
		}
	})
}

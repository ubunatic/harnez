package tools

import (
	"bytes"
	"context"
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
	fsys := fstest.MapFS{"spec/tools/voice-input.yaml": {Data: spec}}

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
		if !strings.Contains(out.String(), "missing") {
			t.Fatal(out.String())
		}
	})

	t.Run("specific ready status succeeds", func(t *testing.T) {
		readyFS := fstest.MapFS{"spec/tools/voice-input.yaml": {Data: spec}}
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

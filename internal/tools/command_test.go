package tools

import (
	"bytes"
	"context"
	"errors"
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

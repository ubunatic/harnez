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

func TestVoiceInputDelegation(t *testing.T) {
	spec, err := harnez.DefaultFS.ReadFile("spec/tools/voice-input.yaml")
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{"spec/tools/voice-input.yaml": {Data: spec}}

	t.Run("prints guidance when voxi is missing", func(t *testing.T) {
		var out bytes.Buffer
		d := testDeps(&out)
		d.LookPath = func(string) (string, error) { return "", errors.New("not found") }

		cmd, err := NewCommand(fsys, d)
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"voice-input", "mode"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "voxi (ubunatic/voxi) is not installed") {
			t.Fatalf("expected install guidance, got: %s", out.String())
		}
	})

	t.Run("delegates to voxi when present", func(t *testing.T) {
		var out bytes.Buffer
		var invokedBinary string
		var invokedArgs []string
		d := testDeps(&out)
		d.LookPath = func(name string) (string, error) {
			if name == "voxi" {
				return "/usr/local/bin/voxi", nil
			}
			return "", errors.New("not found")
		}
		d.Run = func(ctx context.Context, name string, args ...string) error {
			invokedBinary = name
			invokedArgs = args
			return nil
		}

		cmd, err := NewCommand(fsys, d)
		if err != nil {
			t.Fatal(err)
		}
		cmd.SetArgs([]string{"voice-input", "mode", "eager"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if invokedBinary != "/usr/local/bin/voxi" || len(invokedArgs) != 2 || invokedArgs[0] != "mode" || invokedArgs[1] != "eager" {
			t.Fatalf("unexpected delegation call: %s %v", invokedBinary, invokedArgs)
		}
	})
}

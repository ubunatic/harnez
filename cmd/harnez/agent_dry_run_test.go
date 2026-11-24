package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/subagent"
)

func TestAgentDryRunDoesNotLaunchOrWrite(t *testing.T) {
	dir := t.TempDir()
	bin := t.TempDir()
	marker := filepath.Join(bin, "ran")
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("#!/bin/sh\ntouch '"+marker+"'\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	store, err := subagent.NewSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(t.TempDir(), "telemetry.db")
	deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }, dbPath: db}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	req := startRequest{DryRun: true, Prompt: "secret", PromptSource: "-p", Name: "worker", Role: "advisor", ModelSpec: "codex:luna:low", Dir: t.TempDir(), StreamMode: streamFull}
	if err := runStart(t.Context(), cmd, deps, req); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"dry-run: codex:", "role: advisor", "name: worker", "argv:", "<prompt: 6 bytes from -p>"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in %s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "secret") {
		t.Fatal("prompt leaked")
	}
	sessions, err := store.List("", true)
	if err != nil || len(sessions) != 0 {
		t.Fatalf("sessions=%v err=%v", sessions, err)
	}
	for _, path := range []string{marker, db} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("unexpected file %s: %v", path, err)
		}
	}
	req.ModelSpec = "invalid-provider:no-such-model"
	req.DryRun = false
	normalErr := runStart(t.Context(), cmd, deps, req)
	req.DryRun = true
	dryErr := runStart(t.Context(), cmd, deps, req)
	if normalErr == nil || dryErr == nil || normalErr.Error() != dryErr.Error() {
		t.Fatalf("normal=%v dry=%v", normalErr, dryErr)
	}
}

func TestAgentResumeDryRunDoesNotRepair(t *testing.T) {
	dir := t.TempDir()
	store, err := subagent.NewSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "gpt-5.4", Tier: "low", Role: "advisor", WorkingDir: t.TempDir(), Status: "running", HarnessType: "harnez"}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	deps := agentDeps{store: func() (*subagent.FileSessionStore, error) { return store, nil }, parent: func() string { return "" }, find: func(_ *cobra.Command, s *subagent.FileSessionStore, _ string) (*subagent.Session, error) {
		return s.Get("sid")
	}}
	if err := runResume(t.Context(), cmd, deps, resumeRequest{DryRun: true, Name: "worker", Prompt: "hello", PromptSource: "-p", StreamMode: streamFull}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get("sid")
	if err != nil || got.Status != "running" || got.LastError != "" {
		t.Fatalf("session=%+v err=%v", got, err)
	}
	after, err := os.ReadDir(dir)
	if err != nil || len(after) != len(before) {
		t.Fatalf("directory changed: %v", err)
	}
}

func TestDryRunCobraLeavesStoreAndTelemetryUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	bin := t.TempDir()
	marker := filepath.Join(bin, "provider-ran")
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("#!/bin/sh\ntouch '"+marker+"'\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir := t.TempDir()
	store, err := subagent.NewSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "gpt-5.4", Tier: "low", Role: "advisor", WorkingDir: t.TempDir(), Status: "running", HarnessType: "harnez"}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	snapshot := func() map[string]string {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		result := make(map[string]string)
		for _, e := range entries {
			if e.IsDir() {
				t.Fatalf("unexpected directory %s", e.Name())
			}
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			result[e.Name()] = string(b)
		}
		return result
	}
	before := snapshot()
	for _, args := range [][]string{
		{"start", "--dry-run", "--name", "new-worker", "--role", "advisor", "--model", "codex:luna:low", "-p", "hello"},
		{"resume", "--dry-run", "--name", "worker", "-p", "hello"},
	} {
		cmd := newAgentCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(append(args, "--store-dir", dir))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v: %s", args, err, out.String())
		}
		if !strings.Contains(out.String(), "<prompt: 5 bytes from -p>") {
			t.Fatalf("preview: %s", out.String())
		}
		if got := snapshot(); !reflect.DeepEqual(got, before) {
			t.Fatalf("store changed: before %v, after %v", before, got)
		}
		for _, path := range []string{marker, filepath.Join(home, "data", "harnez", "telemetry.sqlite")} {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("unexpected file %s: %v", path, err)
			}
		}
	}
}

func TestAgentStartDryRunInRunnerModeSkipsStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	runnerRoot := filepath.Join(t.TempDir(), "runner-sess")
	if err := os.Mkdir(runnerRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", runnerRoot)

	for _, tc := range []struct {
		name     string
		args     []string
		wantName string
	}{
		{
			name:     "explicit name",
			args:     []string{"start", "--dry-run", "--role", "developer", "--model", "codex:luna:low", "--name", "dry", "hello"},
			wantName: "name: dry",
		},
		{
			name: "omitted name",
			args: []string{"start", "--dry-run", "--role", "developer", "--model", "codex:luna:low", "hello"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newAgentCmd()
			var out, errOut bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errOut)
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("dry-run in runner mode failed: %v (stderr=%q)", err, errOut.String())
			}
			got := out.String()
			if !strings.Contains(got, "role: developer") {
				t.Fatalf("preview missing role: %s", got)
			}
			if tc.wantName != "" && !strings.Contains(got, tc.wantName) {
				t.Fatalf("preview missing %q: %s", tc.wantName, got)
			}
			if !strings.Contains(got, "note: name uniqueness is checked against the host session registry at start") {
				t.Fatalf("preview missing uniqueness note: %s", got)
			}
			if _, err := os.Stat(filepath.Join(home, ".harnez", "agents")); !os.IsNotExist(err) {
				t.Fatalf("expected ~/.harnez/agents not to be created, got err=%v", err)
			}
		})
	}

	// Spy on store() in both runStart and withHostTrackingLine to prove zero invocations.
	storeCalls := 0
	spyStore := func() (*subagent.FileSessionStore, error) {
		storeCalls++
		return nil, os.ErrInvalid
	}
	spyCmd := &cobra.Command{Use: "start", RunE: func(c *cobra.Command, _ []string) error {
		return runStart(t.Context(), c, agentDeps{
			store:  spyStore,
			parent: func() string { return "host-parent" },
		}, startRequest{
			DryRun:     true,
			Role:       "developer",
			ModelSpec:  "codex:luna:low",
			Name:       "dry-spy",
			Prompt:     "hello",
			Dir:        ".",
			StreamMode: streamFull,
		})
	}}
	spyCmd.Flags().Bool("dry-run", false, "")
	var spyOut, spyErr bytes.Buffer
	spyCmd.SetOut(&spyOut)
	spyCmd.SetErr(&spyErr)
	withHostTrackingLine(spyCmd, spyStore, func() string { return "host-parent" }, func() bool { return false })
	spyCmd.SetArgs([]string{"--dry-run"})
	if err := spyCmd.Execute(); err != nil {
		t.Fatalf("spy dry-run failed: %v", err)
	}
	if storeCalls != 0 {
		t.Fatalf("store() called %d times during runner-mode --dry-run, want 0", storeCalls)
	}
}

func TestPromptFlagErrorRedactsPrompt(t *testing.T) {
	for _, prefix := range [][]string{nil, {"start"}} {
		for _, dry := range []bool{false, true} {
			if dry && len(prefix) == 0 {
				continue
			}
			args := append([]string{}, prefix...)
			if dry {
				args = append(args, "--dry-run")
			}
			args = append(args, "-p", "--model private secret")
			cmd := newAgentCmd()
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs(args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), `flag "--model" (22 bytes)`) || strings.Contains(err.Error(), "private") {
				t.Fatalf("args %v: %v", args, err)
			}
		}
	}
}

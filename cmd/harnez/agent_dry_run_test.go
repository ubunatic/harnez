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

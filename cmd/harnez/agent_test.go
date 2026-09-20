package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
)

func TestAgentCommandSurface(t *testing.T) {
	c := newAgentCmd()
	if len(c.Commands()) == 0 {
		t.Fatal("agent command has no children")
	}
	for _, name := range []string{"start", "resume", "list", "status", "compact", "stop", "delete", "enable", "disable"} {
		found := false
		for _, child := range c.Commands() {
			if child.Name() == name {
				found = true
			}
		}
		if !found {
			t.Errorf("missing agent subcommand %q", name)
		}
	}
}

func TestAgentRepoStatusAndPolicyCommands(t *testing.T) {
	dir := t.TempDir()
	store := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.local.md"), []byte("local notes\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"enable", "-d", dir, "--store-dir", store})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "AGENTS.local.md"))
	if err != nil || !bytes.Contains(content, []byte("subagent_mode: harnez")) || !bytes.Contains(content, []byte("local notes")) {
		t.Fatalf("enable did not update local overlay: %v\n%s", err, content)
	}
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "-d", dir, "--store-dir", store})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Subagent Policy State: Enabled (./AGENTS.local.md)")) || !bytes.Contains(out.Bytes(), []byte("Repository Agent Sessions:")) {
		t.Fatalf("status output = %s", out.String())
	}
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"disable", "-d", dir, "--store-dir", store})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(filepath.Join(dir, "AGENTS.local.md"))
	if err != nil || !bytes.Contains(content, []byte("subagent_mode: native")) {
		t.Fatalf("disable did not update local overlay: %v\n%s", err, content)
	}
}

func TestAgentRepoStatusUnsetJSONAndSingleSession(t *testing.T) {
	dir, storeDir := t.TempDir(), t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "inside", Name: "demo", Provider: "codex", Model: "model", Status: "completed", WorkingDir: dir, LastActiveAt: time.Now()}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	cmd := newAgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "-d", dir, "--store-dir", storeDir, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Policy struct {
			Mode string `json:"mode"`
		} `json:"policy"`
		Sessions []subagent.Session `json:"sessions"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Policy.Mode != "unset" || len(got.Sessions) != 1 || got.Sessions[0].ID != "inside" {
		t.Fatalf("repo status = %#v", got)
	}
	out.Reset()
	cmd = newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"status", "inside", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil || !bytes.Contains(out.Bytes(), []byte("demo\nStatus: completed")) {
		t.Fatalf("single session status: err=%v output=%s", err, out.String())
	}
}

type recordingAgentDriver struct{ dir string }

func (d *recordingAgentDriver) Run(_ context.Context, opts subagent.RunOptions) (*subagent.TurnResult, error) {
	d.dir = opts.Dir
	return &subagent.TurnResult{SessionID: "recorded", Response: "ok"}, nil
}
func (d *recordingAgentDriver) Resume(context.Context, string, string) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{}, nil
}
func (d *recordingAgentDriver) Compact(context.Context, string) (*subagent.TurnResult, error) {
	return &subagent.TurnResult{}, nil
}
func (d *recordingAgentDriver) Stop(context.Context, string) error   { return nil }
func (d *recordingAgentDriver) Delete(context.Context, string) error { return nil }

func TestAgentStartStoresCanonicalWorkingDir(t *testing.T) {
	old := agentDriver
	recorder := &recordingAgentDriver{}
	agentDriver = func(subagent.Model) subagent.Driver { return recorder }
	defer func() { agentDriver = old }()
	storeDir := t.TempDir()
	cmd := newAgentCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{"start", "codex:luna", "prompt", "-d", ".", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(".")
	if err != nil || recorder.dir != want {
		t.Fatalf("driver working directory = %q, want %q", recorder.dir, want)
	}
	store, _ := subagent.NewSessionStore(storeDir)
	sess, err := store.Get("recorded")
	if err != nil || sess.WorkingDir != want {
		t.Fatalf("saved working directory = %q, err=%v", sess.WorkingDir, err)
	}
}

func TestAgentHaikuAliases(t *testing.T) {
	for _, spec := range []string{"claude:haiku", "claude:haiku:latest"} {
		m, err := subagent.ResolveModel(spec)
		if err != nil || m.Provider != "claude" || m.Name != "haiku" || m.Tier != "low" {
			t.Fatalf("%s resolved to %#v (%v)", spec, m, err)
		}
	}
}

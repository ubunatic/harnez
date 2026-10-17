package main

import (
	"bytes"
	"context"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

type recordingAgentDriver struct {
	dir     string
	prompt  string
	stopped []string
	deleted []string
	runHook func()
}

type resumeOutcomeDriver struct {
	recordingAgentDriver
	err      error
	resumes  int
	terminal bool
}

func (d *resumeOutcomeDriver) Resume(context.Context, string, string, subagent.Model) (*subagent.TurnResult, error) {
	d.resumes++
	if d.err != nil {
		return nil, d.err
	}
	return &subagent.TurnResult{Response: "resumed"}, nil
}

func (d *resumeOutcomeDriver) CheckResumable(string) (bool, string) {
	if d.terminal {
		return false, "session record is terminal"
	}
	return true, ""
}

func saveResumeSession(t *testing.T, dir, id, name, provider string) *subagent.FileSessionStore {
	t.Helper()
	store, err := subagent.NewSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: id, Name: name, Provider: provider, Model: "model", Tier: "low", Status: "completed", ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	return store
}

// runWithStore runs an agent command against a preloaded store directory.
func runWithStore(t *testing.T, d subagent.Driver, storeDir string, args ...string) (string, error) {
	t.Helper()
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return d }
	defer func() { agentDriver = old }()
	var out bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs(append(args, "--store-dir", storeDir))
	err := cmd.Execute()
	return out.String(), err
}

func saveSessions(t *testing.T, storeDir string, sessions ...*subagent.Session) {
	t.Helper()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sessions {
		if s.Provider == "" {
			s.Provider, s.Model = "codex", "gpt-6-luna"
		}
		if s.Status == "" {
			s.Status = "completed"
		}
		if s.WorkingDir == "" {
			s.WorkingDir = "."
		}
		if s.ContextTokens == 0 {
			s.ContextTokens = 100
		}
		if err := store.Save(s); err != nil {
			t.Fatal(err)
		}
	}
}

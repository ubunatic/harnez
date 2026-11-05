package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/modelblock"
	"ubunatic.com/harnez/internal/subagent"
)

// No cmd/harnez test may reach the real state dir: TestMain points HOME and
// XDG_STATE_HOME at a temp dir, and the block file lives under it.
func TestModelBlockPathIsUnderIsolatedHome(t *testing.T) {
	if testIsolatedHome == "" {
		t.Skip("child helper process shares the parent's isolation")
	}
	if p := modelblock.Path(); !strings.HasPrefix(p, testIsolatedHome) {
		t.Fatalf("block file %s is outside the isolated test home %s", p, testIsolatedHome)
	}
}

// Every resume must honor a block on the session's model, whatever the provider.
func TestResumeRefusesBlockedSessionModelForAllProviders(t *testing.T) {
	for _, provider := range []string{"agy", "codex", "claude"} {
		t.Run(provider, func(t *testing.T) {
			isolateModelState(t)
			d := &resumeOutcomeDriver{}
			storeDir := t.TempDir()
			saveResumeSession(t, storeDir, "sid", "worker", provider)
			now := time.Now()
			if err := modelblock.Add(modelblock.Path(), modelblock.Block{Provider: provider, Model: "model", Until: now.Add(time.Hour), Reason: "manual"}, now); err != nil {
				t.Fatal(err)
			}
			_, err := runWithStore(t, d, storeDir, "resume", "--name", "worker", "hi")
			if err == nil || !strings.Contains(err.Error(), "blocked until") || d.resumes != 0 {
				t.Fatalf("resume = %v, resumes=%d; want refusal naming the block", err, d.resumes)
			}
			if _, err := runWithStore(t, d, storeDir, "resume", "--name", "worker", "--allow-exhausted-quota", "hi"); err != nil || d.resumes != 1 {
				t.Fatalf("override resume = %v, resumes=%d", err, d.resumes)
			}
		})
	}
}

// resume --interactive must honor a block too, and the override must work there.
func TestInteractiveResumeHonorsModelBlock(t *testing.T) {
	isolateModelState(t)
	old := agentInteractiveRunner
	storeDir := t.TempDir()
	runner := &recordingInteractiveRunner{storeDir: storeDir}
	agentInteractiveRunner = runner
	defer func() { agentInteractiveRunner = old }()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "codex-id", Name: "codex-chat", Provider: "codex", Model: "gpt-6-luna", Tier: "low", ProviderSessionID: "pid", HarnessType: "interactive", Status: "completed"}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := modelblock.Add(modelblock.Path(), modelblock.Block{Provider: "codex", Model: "gpt-6-luna", Until: now.Add(time.Hour), Reason: "manual"}, now); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) error {
		cmd := newAgentCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs(append([]string{"resume", "-i", "--name", "codex-chat", "--store-dir", storeDir}, args...))
		return cmd.Execute()
	}
	if err := run(); err == nil || !strings.Contains(err.Error(), "blocked until") || runner.attachID != "" {
		t.Fatalf("interactive resume = %v, attachID=%q; want refusal before attach", err, runner.attachID)
	}
	if err := run("--allow-exhausted-quota"); err != nil || runner.attachID != "pid" {
		t.Fatalf("override interactive resume = %v, attachID=%q", err, runner.attachID)
	}
}

// An explicit --model matching a blocked session must pass with the override.
func TestResumeModelFlagHonorsOverrideOnBlockedModel(t *testing.T) {
	isolateModelState(t)
	d := &resumeOutcomeDriver{}
	storeDir := t.TempDir()
	store, _ := subagent.NewSessionStore(storeDir)
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "codex", Model: "gpt-6-luna", Tier: "low", Status: "completed", ContextTokens: 100}); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := modelblock.Add(modelblock.Path(), modelblock.Block{Provider: "codex", Model: "gpt-6-luna", Until: now.Add(time.Hour)}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := runWithStore(t, d, storeDir, "resume", "--name", "worker", "--model", "codex:luna:low", "hi"); err == nil || d.resumes != 0 {
		t.Fatalf("resume without override = %v, resumes=%d; want refusal", err, d.resumes)
	}
	if _, err := runWithStore(t, d, storeDir, "resume", "--name", "worker", "--model", "codex:luna:low", "--allow-exhausted-quota", "hi"); err != nil || d.resumes != 1 {
		t.Fatalf("override resume = %v, resumes=%d", err, d.resumes)
	}
}

func TestResumeIgnoresExpiredBlock(t *testing.T) {
	isolateModelState(t)
	d := &resumeOutcomeDriver{}
	storeDir := t.TempDir()
	saveResumeSession(t, storeDir, "sid", "worker", "agy")
	past := time.Now().Add(-2 * time.Hour)
	if err := modelblock.Add(modelblock.Path(), modelblock.Block{Provider: "agy", Model: "model", Until: past.Add(time.Hour)}, past); err != nil {
		t.Fatal(err)
	}
	if _, err := runWithStore(t, d, storeDir, "resume", "--name", "worker", "hi"); err != nil || d.resumes != 1 {
		t.Fatalf("resume with expired block = %v, resumes=%d", err, d.resumes)
	}
}

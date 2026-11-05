package main

import (
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/modelblock"
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

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
)

// hostAccessResume resumes the stored session "worker" as the caller with the
// given host id; workerEnv simulates a harnez leaf worker (HARNEZ_SESSION_ID set).
func hostAccessResume(t *testing.T, parent, lastHost, host, workerEnv string) (*subagent.FileSessionStore, error) {
	t.Helper()
	t.Setenv(agentRoleEnv, "")
	t.Setenv(agentSessionEnv, workerEnv)
	old := agentDriver
	agentDriver = func(subagent.Model, string) subagent.Driver { return &resumeOutcomeDriver{} }
	defer func() { agentDriver = old }()
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "sid", Name: "worker", Provider: "fake", Model: "model", Tier: "low", Status: "completed", ContextTokens: 10, ParentSessionID: parent, LastHostSessionID: lastHost}); err != nil {
		t.Fatal(err)
	}
	deps := agentDeps{
		store:  func() (*subagent.FileSessionStore, error) { return store, nil },
		parent: func() string { return host },
		find: func(_ *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
			return s.Find(id)
		},
		availability: func(string, string) usage.ProviderQuotaAvailability {
			return usage.ProviderQuotaAvailability{State: "available"}
		},
	}
	return store, runResume(newAgentCmd(), deps, resumeRequest{Name: "worker", Prompt: "hi", StreamMode: streamFull})
}

func TestHostResumesLegacyAgentWithEmptyParent(t *testing.T) {
	store, err := hostAccessResume(t, "", "", "host-a", "")
	if err != nil {
		t.Fatalf("host must resume legacy agent: %v", err)
	}
	for _, h := range []string{"host-a"} {
		xs, _ := store.List(h, false)
		if len(xs) != 1 {
			t.Fatalf("host %s lists %d agents after resume, want 1", h, len(xs))
		}
	}
}

func TestHostResumesOtherHostsAgent(t *testing.T) {
	store, err := hostAccessResume(t, "host-a", "", "host-b", "")
	if err != nil {
		t.Fatalf("host must resume other host's agent: %v", err)
	}
	for _, h := range []string{"host-a", "host-b"} {
		xs, _ := store.List(h, false)
		if len(xs) != 1 {
			t.Fatalf("host %s lists %d agents after hand-off, want 1", h, len(xs))
		}
	}
}

func TestWorkerStillRefusedOutsideLineage(t *testing.T) {
	for _, parent := range []string{"", "host-a"} {
		_, err := hostAccessResume(t, parent, "", "worker-x", "worker-x")
		if err == nil || !strings.Contains(err.Error(), "outside caller lineage") {
			t.Fatalf("worker with parent %q must be refused, err=%v", parent, err)
		}
	}
}

func listWithHost(t *testing.T, host string, extra ...string) (string, string) {
	t.Helper()
	for _, k := range []string{"AGY_CONVERSATION_ID", "CLAUDE_CODE_SESSION_ID", "CLAUDE_SESSION_ID", "ANTIGRAVITY_CONVERSATION_ID", "ANTIGRAVITY_SESSION_ID", "CODEX_SESSION_ID", "CODEX_THREAD_ID"} {
		t.Setenv(k, "")
	}
	t.Setenv(agentSessionEnv, host)
	storeDir := t.TempDir()
	st, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	saveTracked(t, st, "1", "mine", "host-a", "", "completed")
	saveTracked(t, st, "2", "legacy", "", "", "completed")
	saveTracked(t, st, "3", "theirs", "host-b", "", "completed")
	var out, errOut bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(append([]string{"list", "--store-dir", storeDir}, extra...))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	return out.String(), errOut.String()
}

func TestListHintShownForOtherSessionAgents(t *testing.T) {
	out, errOut := listWithHost(t, "host-a")
	if !strings.Contains(out, "mine") || strings.Contains(out, "theirs") {
		t.Fatalf("scoped list = %q", out)
	}
	if want := "harnez: 2 more agents in other sessions; use --all-sessions"; !strings.Contains(errOut, want) {
		t.Fatalf("stderr = %q, want %q", errOut, want)
	}
}

func TestListHintNotShownWhenNothingHidden(t *testing.T) {
	if _, errOut := listWithHost(t, "host-a", "--all-sessions"); strings.Contains(errOut, "more agents") {
		t.Fatalf("--all-sessions must not hint, stderr=%q", errOut)
	}
	if _, errOut := listWithHost(t, ""); strings.Contains(errOut, "more agents") {
		t.Fatalf("unscoped caller must not hint, stderr=%q", errOut)
	}
}

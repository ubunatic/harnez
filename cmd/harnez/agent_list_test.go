package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func TestAppendExternalAgentSessionsDeduplicatesManagedProviderIdentity(t *testing.T) {
	managed := &subagent.Session{ID: "managed", ProviderSessionID: "provider-id", Provider: "codex"}
	listed := []*subagent.Session{managed}
	external := []*subagent.Session{
		{ID: "provider-id", ProviderSessionID: "provider-id", Provider: "codex", HarnessType: "external"},
		{ID: "other-id", ProviderSessionID: "other-id", Provider: "codex", HarnessType: "external"},
	}
	got := appendExternalAgentSessions(listed, []*subagent.Session{managed}, external, nil)
	if len(got) != 2 || got[1].ID != "other-id" {
		t.Fatalf("merged sessions = %#v, want managed plus nonduplicate external", got)
	}
}

func TestAppendExternalAgentSessionsDoesNotRediscoverDeletedProviderSession(t *testing.T) {
	deleted := &subagent.Session{ID: "archived", ProviderSessionID: "provider-id", Provider: "codex"}
	external := []*subagent.Session{{ID: "provider-id", ProviderSessionID: "provider-id", Provider: "codex", HarnessType: "external"}}
	got := appendExternalAgentSessions(nil, nil, external, []*subagent.Session{deleted})
	if len(got) != 0 {
		t.Fatalf("rediscovered archived session: %#v", got)
	}
}

func TestAgentListIncludesExternalSessionsOnlyInGlobalViews(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(agentSessionEnv, "host-a")
	t.Setenv("AGY_CONVERSATION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("ANTIGRAVITY_CONVERSATION_ID", "")
	t.Setenv("ANTIGRAVITY_SESSION_ID", "")

	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&subagent.Session{ID: "managed-id", Name: "managed", Provider: "codex", ProviderSessionID: "managed-provider-id", ParentSessionID: "host-a", Status: "running"}); err != nil {
		t.Fatal(err)
	}
	rollout := filepath.Join(home, ".codex/sessions/2026/10/02/rollout-external.jsonl")
	if err := os.MkdirAll(filepath.Dir(rollout), 0o700); err != nil {
		t.Fatal(err)
	}
	metadata := `{"type":"session_meta","payload":{"id":"external-session-id","session_id":"external-session-id","cwd":"/work/external-project","model_provider":"openai","source":"cli","timestamp":"2026-10-02T10:00:00Z"}}` + "\n"
	if err := os.WriteFile(rollout, []byte(metadata), 0o600); err != nil {
		t.Fatal(err)
	}

	var scoped bytes.Buffer
	cmd := newAgentCmd()
	cmd.SetOut(&scoped)
	cmd.SetArgs([]string{"list", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(scoped.String(), "managed") || strings.Contains(scoped.String(), "external-session-id") {
		t.Fatalf("host-scoped list = %q, want only its managed agents", scoped.String())
	}

	var global bytes.Buffer
	cmd = newAgentCmd()
	cmd.SetOut(&global)
	cmd.SetArgs([]string{"list", "--all-sessions", "--dead", "--json", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var sessions []subagent.Session
	if err := json.Unmarshal(global.Bytes(), &sessions); err != nil {
		t.Fatalf("decode global list: %v; output=%s", err, global.String())
	}
	if len(sessions) != 2 {
		t.Fatalf("global list contains %d entries, want managed and external: %s", len(sessions), global.String())
	}
	var table bytes.Buffer
	cmd = newAgentCmd()
	cmd.SetOut(&table)
	cmd.SetArgs([]string{"list", "--all-sessions", "--dead", "--store-dir", storeDir})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(table.String(), "SOURCE\tWORKING_DIR") || !strings.Contains(table.String(), "cli\t/work/external-project") {
		t.Fatalf("global table does not expose external project metadata: %q", table.String())
	}
	for _, session := range sessions {
		if session.ID == "external-session-id" {
			if session.Provider != "codex" || session.Status != "available" || session.WorkingDir != "/work/external-project" || session.Source != "cli" {
				t.Errorf("external metadata = %#v", session)
			}
			return
		}
	}
	t.Fatalf("external session missing from global list: %s", global.String())
}

func TestAgentListDefaultsToActiveAndHasDeadArchivedFilters(t *testing.T) {
	storeDir := t.TempDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, sess := range []*subagent.Session{
		{ID: "live", Name: "live", Provider: "fake", Status: "running"},
		{ID: "done", Name: "done", Provider: "fake", Status: "completed"},
		{ID: "old", Name: "old", Provider: "fake", Status: "completed"},
	} {
		if err := store.Save(sess); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Delete("old"); err != nil {
		t.Fatal(err)
	}
	list := func(args ...string) string {
		t.Helper()
		var out bytes.Buffer
		cmd := newAgentCmd()
		cmd.SetOut(&out)
		cmd.SetArgs(append(append([]string{"list", "--all-sessions"}, args...), "--store-dir", storeDir))
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	if got := list(); !strings.Contains(got, "live") || strings.Contains(got, "done") || strings.Contains(got, "old") {
		t.Fatalf("default list = %q, want active only", got)
	}
	if got := list("--dead"); !strings.Contains(got, "live") || !strings.Contains(got, "done") || strings.Contains(got, "old") {
		t.Fatalf("--dead list = %q, want active and inactive non-archived", got)
	}
	if got := list("--archived"); !strings.Contains(got, "live") || strings.Contains(got, "done") || !strings.Contains(got, "old") {
		t.Fatalf("--archived list = %q, want active and archived", got)
	}
	if got := list("--dead", "--archived"); !strings.Contains(got, "live") || !strings.Contains(got, "done") || !strings.Contains(got, "old") {
		t.Fatalf("--dead --archived list = %q, want all statuses and archive", got)
	}
}

func TestAgentListGlobalAndChildrenViews(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(agentSessionEnv, "")
	t.Setenv("AGY_CONVERSATION_ID", "")
	t.Setenv("CODEX_THREAD_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("ANTIGRAVITY_CONVERSATION_ID", "")
	t.Setenv("ANTIGRAVITY_SESSION_ID", "")

	rollout := filepath.Join(home, ".codex/sessions/2026/10/02/rollout-external.jsonl")
	if err := os.MkdirAll(filepath.Dir(rollout), 0o700); err != nil {
		t.Fatal(err)
	}
	metadata := `{"type":"session_meta","payload":{"id":"external-session-id","session_id":"external-session-id","cwd":"/work/external-project","model_provider":"openai","source":"cli","timestamp":"2026-10-02T10:00:00Z"}}` + "\n"
	if err := os.WriteFile(rollout, []byte(metadata), 0o600); err != nil {
		t.Fatal(err)
	}
	storeDir := t.TempDir()

	for _, args := range [][]string{{"list", "--dead"}, {"list", "--children", "--dead"}} {
		var out bytes.Buffer
		cmd := newAgentCmd()
		cmd.SetOut(&out)
		cmd.SetArgs(append(args, "--store-dir", storeDir))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute %v: %v", args, err)
		}
		wantExternal := len(args) == 2
		if gotExternal := strings.Contains(out.String(), "external-session-id"); gotExternal != wantExternal {
			t.Errorf("list %v includes external session = %v, want %v: %q", args[1:], gotExternal, wantExternal, out.String())
		}
	}

	cmd := newAgentCmd()
	cmd.SetArgs([]string{"list", "--all-sessions", "--children", "--store-dir", storeDir})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("conflicting list flags error = %v, want explicit incompatibility", err)
	}
}

func TestAgentListThenResumeByDisplayedIDAndName(t *testing.T) {
	for _, identifier := range []string{"sid", "worker"} {
		t.Run(identifier, func(t *testing.T) {
			old := agentDriver
			d := &resumeOutcomeDriver{}
			agentDriver = func(subagent.Model, string) subagent.Driver { return d }
			defer func() { agentDriver = old }()
			storeDir := t.TempDir()
			saveResumeSession(t, storeDir, "sid", "worker", "fake")
			var listed bytes.Buffer
			cmd := newAgentCmd()
			cmd.SetOut(&listed)
			cmd.SetArgs([]string{"list", "--all-sessions", "--dead", "--store-dir", storeDir})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(listed.String(), "sid\tworker") {
				t.Fatalf("list = %q", listed.String())
			}
			cmd = newAgentCmd()
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs([]string{"resume", "--name", identifier, "hi", "--store-dir", storeDir})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

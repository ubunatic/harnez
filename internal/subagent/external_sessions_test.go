package subagent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverExternalSessionsParsesAndDeduplicatesCodexRollouts(t *testing.T) {
	home := t.TempDir()
	paths := []string{
		".codex/sessions/2026/10/02/rollout-one.jsonl",
		".codex/sessions/2026/10/03/rollout-copy.jsonl",
	}
	line := `{"type":"session_meta","payload":{"id":"session-12345678","session_id":"session-12345678","cwd":"/work/example","model_provider":"openai","source":"cli","timestamp":"2026-10-02T10:00:00Z"}}` + "\n"
	for _, relative := range paths {
		path := filepath.Join(home, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	sessions, err := DiscoverExternalSessions(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("discovered %d sessions, want duplicate ID collapsed to one: %#v", len(sessions), sessions)
	}
	session := sessions[0]
	if session.ID != "session-12345678" || session.ProviderID() != "session-12345678" {
		t.Errorf("identity = %q / %q", session.ID, session.ProviderID())
	}
	if session.Provider != "codex" || session.Source != "cli" || session.ModelProvider != "openai" {
		t.Errorf("provider metadata = %#v", session)
	}
	if session.WorkingDir != "/work/example" || session.Status != "available" || session.HarnessType != "external" {
		t.Errorf("session metadata = %#v", session)
	}
	if session.CreatedAt.IsZero() {
		t.Error("created_at was not extracted from session metadata")
	}
}

func TestDiscoverExternalSessionsSkipsMalformedAndUnrelatedRollouts(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".codex/sessions/2026/10/02")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"rollout-bad.jsonl":        "not json\n",
		"rollout-other.jsonl":      `{"type":"event_msg","payload":{}}` + "\n",
		"rollout-missing-id.jsonl": `{"type":"session_meta","payload":{"cwd":"/work/example"}}` + "\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sessions, err := DiscoverExternalSessions(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("discovered malformed records: %#v", sessions)
	}
}

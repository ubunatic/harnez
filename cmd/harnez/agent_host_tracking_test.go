package main

import (
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func saveTracked(t *testing.T, st *subagent.FileSessionStore, id, name, parent, lastHost, status string) {
	t.Helper()
	if err := st.Save(&subagent.Session{ID: id, Name: name, ParentSessionID: parent, LastHostSessionID: lastHost, Status: status, Provider: "codex"}); err != nil {
		t.Fatal(err)
	}
}

func TestHostSessionTrackingLine(t *testing.T) {
	st, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	saveTracked(t, st, "1", "rev-b", "host-a", "", "completed")
	saveTracked(t, st, "2", "dev-a", "host-a", "", "running")
	saveTracked(t, st, "3", "adv-c", "host-a", "", "completed")
	saveTracked(t, st, "4", "other", "host-b", "", "running")

	want := "harnez: this session started 3 agents (1 running): adv-c, dev-a, rev-b"
	if got := hostSessionTrackingLine(st, "host-a"); got != want {
		t.Fatalf("line = %q, want %q", got, want)
	}
	if got := hostSessionTrackingLine(st, "host-b"); got != "harnez: this session started 1 agent (1 running): other" {
		t.Fatalf("singular line = %q", got)
	}
	if got := hostSessionTrackingLine(st, ""); got != "" {
		t.Fatalf("no host id must print nothing, got %q", got)
	}
	if got := hostSessionTrackingLine(st, "unknown-host"); got != "" {
		t.Fatalf("unknown host must print nothing, got %q", got)
	}
}

func TestHostSessionTrackingLineSameNameTwice(t *testing.T) {
	st, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	saveTracked(t, st, "1", "w", "host-a", "", "completed")
	saveTracked(t, st, "2", "w", "host-a", "", "running")
	got := hostSessionTrackingLine(st, "host-a")
	if !strings.Contains(got, "started 2 agents (1 running): w, w") {
		t.Fatalf("same name must count per session, got %q", got)
	}
}

func TestHostSessionTrackingLineResumeFromOtherSession(t *testing.T) {
	st, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	saveTracked(t, st, "1", "w", "host-a", "host-b", "completed")
	for _, host := range []string{"host-a", "host-b"} {
		if got := hostSessionTrackingLine(st, host); got != "harnez: this session started 1 agent (0 running): w" {
			t.Fatalf("host %s line = %q", host, got)
		}
	}
}

func TestCurrentHostSessionEnvPriority(t *testing.T) {
	t.Setenv("HARNEZ_SESSION_ID", "worker-1")
	t.Setenv("AGY_CONVERSATION_ID", "agy-1")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "claude-1")
	if got := currentHostSession(); got != "worker-1" {
		t.Fatalf("got %q, want worker-1", got)
	}
	t.Setenv("HARNEZ_SESSION_ID", "")
	if got := currentHostSession(); got != "agy-1" {
		t.Fatalf("got %q, want agy-1", got)
	}
	t.Setenv("AGY_CONVERSATION_ID", "")
	if got := currentHostSession(); got != "claude-1" {
		t.Fatalf("got %q, want claude-1", got)
	}
}

func TestCurrentHostSessionPlainTerminalIsEmpty(t *testing.T) {
	for _, k := range []string{"HARNEZ_SESSION_ID", "AGY_CONVERSATION_ID", "CLAUDE_CODE_SESSION_ID", "CLAUDE_SESSION_ID", "ANTIGRAVITY_CONVERSATION_ID", "ANTIGRAVITY_SESSION_ID", "CODEX_SESSION_ID", "CODEX_THREAD_ID"} {
		t.Setenv(k, "")
	}
	if got := currentHostSession(); got != "" {
		t.Fatalf("plain terminal must have no host id, got %q", got)
	}
}

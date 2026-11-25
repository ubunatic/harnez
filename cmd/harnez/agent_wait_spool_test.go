package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

// agent wait falls back to the spool only for an entry the registry confirms. A
// verified entry is waited on read-only; a stale or unverified one is refused.
func TestAgentWaitUsesOnlyVerifiedSpoolEntries(t *testing.T) {
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	wait := func(t *testing.T, id string) (string, error) {
		var out bytes.Buffer
		cmd := newAgentCmd()
		cmd.SetOut(&out)
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"--store-dir", t.TempDir(), "wait", "--timeout", "1s", "--json", id})
		err := cmd.Execute()
		return out.String(), err
	}
	// save puts one completed session in its own registry and returns that directory.
	save := func(t *testing.T, id string) string {
		t.Helper()
		registry := t.TempDir()
		store, err := subagent.NewSessionStore(registry)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Create(&subagent.Session{ID: id, Name: id, Status: "completed", Response: "finished"}); err != nil {
			t.Fatal(err)
		}
		return registry
	}

	t.Run("verified entry is waited on", func(t *testing.T) {
		isolateSpool(t)
		save(t, "spw1")
		out, err := wait(t, "spw1")
		if err != nil || !strings.Contains(out, `"response":"finished"`) {
			t.Fatalf("wait on verified spool entry: err=%v output=%s", err, out)
		}
	})

	t.Run("stale entry is refused", func(t *testing.T) {
		isolateSpool(t)
		registry := save(t, "spw2")
		if err := os.RemoveAll(registry); err != nil {
			t.Fatal(err)
		}
		out, err := wait(t, "spw2")
		if err == nil || strings.Contains(out, "finished") {
			t.Fatalf("wait on stale spool entry: err=%v output=%s", err, out)
		}
	})

	t.Run("unverified entry is refused", func(t *testing.T) {
		isolateSpool(t)
		registry := save(t, "spw3")
		// The registry file now names another session, so the pointer is unverified.
		if err := os.WriteFile(filepath.Join(registry, "spw3.json"), []byte(`{"id":"other","name":"spw3","status":"completed","response":"forged"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		out, err := wait(t, "spw3")
		if err == nil || strings.Contains(out, "forged") {
			t.Fatalf("wait on unverified spool entry: err=%v output=%s", err, out)
		}
	})
}

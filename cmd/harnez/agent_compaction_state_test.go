package main

import (
	"bytes"
	"encoding/json"
	"github.com/spf13/cobra"
	"testing"
	"ubunatic.com/harnez/internal/subagent"
)

func TestSaveCompactResult(t *testing.T) {
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "test", ContextTokens: 71705, TokensSinceCompact: 100}
	result := &subagent.TurnResult{CompactionObserved: true, ContextTokensBeforeCompact: 71705, ContextTokens: 17063}
	if err := saveCompactResult(store, sess, result); err != nil {
		t.Fatal(err)
	}
	saved, err := store.Get("test")
	if err != nil {
		t.Fatal(err)
	}
	if saved.ContextTokens != 17063 || saved.CompactionCount != 1 || saved.TokensSinceCompact != 0 {
		t.Fatalf("saved session: %+v", saved)
	}
}

func TestSaveCompactResultProviderUsage(t *testing.T) {
	for _, tc := range []struct {
		provider string
		tokens   int
	}{{"claude", 2092}, {"agy", -1}} {
		t.Run(tc.provider, func(t *testing.T) {
			store, err := subagent.NewSessionStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			sess := &subagent.Session{ID: "test", Provider: tc.provider, ContextTokens: 10000}
			if err := saveCompactResult(store, sess, &subagent.TurnResult{CompactionObserved: true, ContextTokens: tc.tokens}); err != nil {
				t.Fatal(err)
			}
			saved, err := store.Get("test")
			if err != nil || saved.ContextTokens != tc.tokens || saved.CompactionCount != 1 {
				t.Fatalf("saved = %+v, error = %v", saved, err)
			}
		})
	}
}

func TestStatusJSONMissingTelemetry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	if err := statusSession(cmd, &subagent.Session{ID: "s"}, true, false, 0, false); err != nil {
		t.Fatal(err)
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(output.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if string(value["compaction_pair"]) != "null" {
		t.Fatalf("output = %s", output.String())
	}
}

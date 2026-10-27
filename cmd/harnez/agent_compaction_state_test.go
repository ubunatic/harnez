package main

import (
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

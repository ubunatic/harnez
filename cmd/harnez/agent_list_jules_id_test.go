package main

import (
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func TestAgentListShowsJulesID(t *testing.T) {
	got := withJulesID("yes", &subagent.Session{Provider: "jules", ProviderSessionID: "4295776973374618428"})
	if got != "yes jules-id=4295776973374618428" {
		t.Fatalf("got %q", got)
	}
	if got := withJulesID("yes", &subagent.Session{Provider: "codex", ProviderSessionID: "x"}); got != "yes" {
		t.Fatalf("non-jules row changed: %q", got)
	}
}

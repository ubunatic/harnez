package main

import (
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func TestFailedStartNameIsReleased(t *testing.T) {
	s, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	failed := &subagent.Session{ID: "2c8f6221-aaaa", Name: "jules-audit-149", Provider: "jules", Status: "failed"}
	remote := &subagent.Session{ID: "3d9e7332-bbbb", Name: "jules-audit-150", Provider: "jules", Status: "failed", ProviderSessionID: "123"}
	for _, x := range []*subagent.Session{failed, remote} {
		if err := s.Create(x); err != nil {
			t.Fatal(err)
		}
	}
	if !releaseFailedStartName(s, failed) {
		t.Fatal("name of a start that failed before any provider session was not released")
	}
	if _, err := s.Find("jules-audit-149"); err == nil {
		t.Fatal("old record still answers to the name")
	}
	if got, err := s.Find("jules-audit-149-failed-2c8f6221"); err != nil || got.ID != failed.ID {
		t.Fatalf("old record lost: %v %v", got, err)
	}
	if releaseFailedStartName(s, remote) {
		t.Fatal("a failed session with a provider session must keep its name")
	}
}

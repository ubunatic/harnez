package main

import (
	"bytes"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

// 1186: a taken explicit --name gets the first free suffix, and stderr names it.
func TestSuffixTakenName(t *testing.T) {
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for id, name := range map[string]string{"id-1": "x", "id-2": "x-2"} {
		if err := store.Create(&subagent.Session{ID: id, Name: name, Status: "done"}); err != nil {
			t.Fatal(err)
		}
	}
	var msg bytes.Buffer
	got, err := suffixTakenName(store, "x", &msg)
	if err != nil || got != "x-3" {
		t.Fatalf("suffixTakenName = %q, %v; want x-3", got, err)
	}
	if want := "harnez: name x taken; using x-3\n"; msg.String() != want {
		t.Fatalf("stderr = %q, want %q", msg.String(), want)
	}
	if err := store.Create(&subagent.Session{ID: "id-3", Name: got}); err != nil {
		t.Fatalf("create suffixed session: %v", err)
	}
}

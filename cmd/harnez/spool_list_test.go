package main

import (
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

func TestAppendSpoolSessionsShowsVerifiedRowOnce(t *testing.T) {
	t.Setenv("HARNEZ_SPOOL_DIR", t.TempDir())
	confined, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := confined.Create(&subagent.Session{ID: "conf1", Name: "conf", Status: "running"}); err != nil {
		t.Fatal(err)
	}

	xs, err := appendSpoolSessions(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(xs) != 1 || xs[0].ID != "conf1" || xs[0].Source != "spool-verified" {
		t.Fatalf("rows = %+v, want one verified conf1 row", xs)
	}

	// The same session in the host registry must not appear a second time.
	registry := []*subagent.Session{{ID: "conf1", Name: "conf"}}
	xs, err = appendSpoolSessions(nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(xs) != 0 {
		t.Fatalf("registry entry duplicated from spool: %+v", xs)
	}
}

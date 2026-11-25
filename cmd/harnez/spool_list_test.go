package main

import (
	"bytes"
	"strings"
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

// Spool rows must meet the same status filters as registry rows: the default
// view keeps running and active, --dead keeps every status, and --archived adds
// archived rows without adding completed ones.
func TestAgentListSpoolRowsFollowRegistryFilters(t *testing.T) {
	isolateSpool(t)
	// Each session lives in its own store; the list's registry does not hold it,
	// so the spool pointer is the only trace the list can read.
	statuses := map[string]string{
		"sp-run":  "running",
		"sp-act":  "active",
		"sp-done": "completed",
		"sp-fail": "failed",
	}
	for name, status := range statuses {
		st, err := subagent.NewSessionStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if err := st.Create(&subagent.Session{ID: name, Name: name, Provider: "fake", Status: status}); err != nil {
			t.Fatal(err)
		}
	}
	storeDir := t.TempDir()
	if _, err := subagent.NewSessionStore(storeDir); err != nil {
		t.Fatal(err)
	}
	all := []string{"sp-run", "sp-act", "sp-done", "sp-fail"}
	want := map[string][]string{
		"":                  {"sp-run", "sp-act"},
		"--dead":            all,
		"--archived":        {"sp-run", "sp-act"},
		"--dead --archived": all,
	}
	for flags, names := range want {
		var out bytes.Buffer
		cmd := newAgentCmd()
		cmd.SetOut(&out)
		args := append([]string{"list", "--all-sessions"}, strings.Fields(flags)...)
		cmd.SetArgs(append(args, "--store-dir", storeDir))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("list %q: %v", flags, err)
		}
		shown := make(map[string]string)
		for _, line := range strings.Split(out.String(), "\n") {
			cols := strings.Split(line, "\t")
			if len(cols) > 8 {
				shown[cols[1]] = cols[8]
			}
		}
		for _, name := range all {
			source, got := shown[name]
			if wanted := contains(names, name); got != wanted {
				t.Errorf("list %q shows %s = %v, want %v:\n%s", flags, name, got, wanted, out.String())
			}
			if got && source != "spool-verified" {
				t.Errorf("list %q: %s SOURCE = %q, want spool-verified", flags, name, source)
			}
		}
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

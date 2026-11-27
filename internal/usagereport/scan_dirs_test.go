package usagereport

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanDirsDedupesSessionAcrossDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir1 := filepath.Join(home, ".claude")
	dir2 := filepath.Join(home, ".local", "state", "taia", "claude")

	// dir1 and dir2 both have session s1; dir1 must win deterministically.
	writeFile(t, filepath.Join(dir1, "projects", "-p", "s1.jsonl"),
		line("m1", "claude-haiku-5-5", "2026-11-13T10:01:00Z", 5))
	writeFile(t, filepath.Join(dir1, "projects", "-p", "s1", "subagents", "a.jsonl"),
		line("m1", "claude-haiku-5-5", "2026-11-13T10:01:01Z", 11))
	writeFile(t, filepath.Join(dir2, "projects", "-p", "s1.jsonl"),
		line("m2", "claude-haiku-5-5", "2026-11-13T10:02:00Z", 99))
	writeFile(t, filepath.Join(dir2, "projects", "-p", "s1", "subagents", "b.jsonl"),
		line("m2b", "claude-haiku-5-5", "2026-11-13T10:02:01Z", 88))
	writeFile(t, filepath.Join(dir2, "projects", "-p", "s2.jsonl"),
		line("m3", "claude-haiku-5-5", "2026-11-13T10:03:00Z", 7))

	calls, warnings, err := ScanDirs([]string{dir1, dir2}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2 (m1 from dir1, m3 from dir2): %+v", len(calls), calls)
	}
	byID := map[string]Call{}
	for _, c := range calls {
		byID[c.MessageID] = c
	}
	if c, ok := byID["m1"]; !ok || c.Usage.Output != 11 || c.Source != "~/.claude" {
		t.Fatalf("m1 = %+v, want output 11 and source ~/.claude", c)
	}
	if _, ok := byID["m2"]; ok {
		t.Fatalf("m2 from dir2 should have been deduped by session s1")
	}
	if _, ok := byID["m2b"]; ok {
		t.Fatalf("m2b nested in dir2 should have been deduped by session s1")
	}
	if c, ok := byID["m3"]; !ok || c.Source != "~/.local/state/taia/claude" {
		t.Fatalf("m3 = %+v, want source ~/.local/state/taia/claude", c)
	}

	rep := Build(calls, nil, Options{
		Since: time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC),
	})
	var jsonBuf bytes.Buffer
	if err := RenderJSON(&jsonBuf, rep); err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Sessions []struct {
			Session string `json:"session"`
			Source  string `json:"source"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(jsonBuf.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, s := range parsed.Sessions {
		sources[s.Session] = s.Source
	}
	if sources["s1"] != "~/.claude" || sources["s2"] != "~/.local/state/taia/claude" {
		t.Fatalf("json sources = %+v", sources)
	}

	var textBuf bytes.Buffer
	if err := RenderText(&textBuf, rep, 1); err != nil {
		t.Fatal(err)
	}
	out := textBuf.String()
	if !strings.Contains(out, "SOURCE") || !strings.Contains(out, "~/.local/state/taia/claude") {
		t.Fatalf("text output missing SOURCE column or source label:\n%s", out)
	}
}

func TestScanDirsMissingDirSkippedWithNote(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir1 := filepath.Join(home, ".claude")
	absent := filepath.Join(home, "absent-claude")
	writeFile(t, filepath.Join(dir1, "projects", "-p", "s1.jsonl"),
		line("m1", "claude-haiku-5-5", "2026-11-13T10:01:00Z", 5))

	calls, warnings, err := ScanDirs([]string{dir1, absent}, time.Time{})
	if err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "~/absent-claude") {
		t.Fatalf("warnings = %v, want note for ~/absent-claude", warnings)
	}
}

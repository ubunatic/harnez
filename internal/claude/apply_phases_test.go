package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestApplyReportAddStatSkipsEmptyDetail(t *testing.T) {
	r := &applyReport{}
	r.addStat("skills", "")
	r.addStat("commands", "a, b")
	if len(r.stats) != 1 || r.stats[0] != (applyStat{"commands", "a, b"}) {
		t.Fatalf("stats = %+v, want only the non-empty commands line", r.stats)
	}
}

func TestApplyCommandsWritesOncePerTargetAndHonoursSelection(t *testing.T) {
	cfg := &Config{
		FS:       fstest.MapFS{},
		Commands: []Command{{Name: "hello", Description: "greet", Content: "Say hi."}},
	}
	target := t.TempDir()

	r := &applyReport{}
	if err := applyCommands(r, cfg, nil, target); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(target, "commands", "hello.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Say hi.") || !strings.Contains(string(data), `description: "greet"`) {
		t.Fatalf("command file = %q", data)
	}
	if r.changes != 1 || len(r.stats) != 1 || r.stats[0].detail != "hello" {
		t.Fatalf("first apply report = %+v, want 1 change and a commands stat", r)
	}

	again := &applyReport{}
	if err := applyCommands(again, cfg, nil, target); err != nil {
		t.Fatal(err)
	}
	if again.changes != 0 {
		t.Fatalf("re-apply changes = %d, want 0 (idempotent)", again.changes)
	}

	other := t.TempDir()
	skipped := &applyReport{}
	if err := applyCommands(skipped, cfg, Set{Component("docs"): true}, other); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(other, "commands")); !os.IsNotExist(err) || skipped.changes != 0 || len(skipped.stats) != 0 {
		t.Fatalf("selection without skills must install nothing: stat err=%v report=%+v", err, skipped)
	}
}

func TestApplyGlobalDocsSelectionAndUnknownDoc(t *testing.T) {
	cfg := &Config{FS: fstest.MapFS{}}
	r := &applyReport{}
	if err := applyGlobalDocs(r, cfg, "full", []string{"missing"}, false, Set{Component("skills"): true}, t.TempDir()); err != nil {
		t.Fatalf("docs outside the selection must be skipped, got %v", err)
	}
	if r.changes != 0 {
		t.Fatalf("changes = %d, want 0", r.changes)
	}
	if err := applyGlobalDocs(r, cfg, "full", []string{"missing"}, false, nil, t.TempDir()); err == nil || !strings.Contains(err.Error(), "unknown doc: missing") {
		t.Fatalf("unknown doc error = %v", err)
	}
}

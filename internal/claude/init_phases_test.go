package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"ubunatic.com/harnez/internal/markdown"
)

func TestInitLocalSectionsGating(t *testing.T) {
	dir := t.TempDir()
	agentsPath := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# Project\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{AgentsMD: AgentsMD{Local: AgentsMDTarget{
		Target:   "./AGENTS.md",
		Sections: []MDSection{{Name: "Rules", Content: "- be kind"}},
	}}}

	ir := &initReport{}
	if err := initLocalSections(ir, cfg, nil, "", agentsPath, false); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(agentsPath); string(data) != "# Project\n" || ir.changes != 0 {
		t.Fatalf("unsafe managed blocks must leave AGENTS.md untouched: %q, changes %d", data, ir.changes)
	}

	if err := initLocalSections(ir, cfg, nil, "solo", agentsPath, true); err == nil || !strings.Contains(err.Error(), `unknown repo mode "solo"`) {
		t.Fatalf("unknown repo mode error = %v", err)
	}
}

func TestInitProjectSummaryDisabledIsNoop(t *testing.T) {
	dir := t.TempDir()
	ir := &initReport{}
	if err := initProjectSummary(ir, dir, false, filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if ir.changes != 0 {
		t.Fatalf("changes = %d, want 0", ir.changes)
	}
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("disabled summary must not create AGENTS.md (stat err %v)", err)
	}
}

func TestInitLocalSectionsPreservesUserTextAndStickyBlocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := os.WriteFile(path, []byte("# User-owned text\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Obsolete", "Project Summary", "Repo Setup"} {
		if _, _, err := markdown.Apply(path, name, "keep "+name); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &Config{AgentsMD: AgentsMD{
		Local: AgentsMDTarget{Target: "./AGENTS.md", Sections: []MDSection{{Name: "Rules", Content: "rule content"}}},
	}}
	r := &initReport{}
	if err := initLocalSections(r, cfg, nil, "", path, true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# User-owned text", "rule content", "keep Project Summary", "keep Repo Setup"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %q from %q", want, data)
		}
	}
	if strings.Contains(string(data), "Obsolete") || r.changes != 2 {
		t.Fatalf("prune and section write: changes=%d, content=%q", r.changes, data)
	}
	again := &initReport{}
	if err := initLocalSections(again, cfg, nil, "", path, true); err != nil || again.changes != 0 {
		t.Fatalf("repeat: changes=%d err=%v", again.changes, err)
	}
}

func TestInitCopyDocsTemplateAndManagedTail(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{FS: fstest.MapFS{
		"doc.md":             {Data: []byte("bundled doc\n")},
		"templates/Makefile": {Data: []byte("# template\n")},
	}, AgentsMD: AgentsMD{Languages: map[string]Language{
		"doc": {Source: "doc.md", Local: "./Doc.md", Template: "templates/Makefile", Targets: "missing-targets.md"},
	}}}
	r := &initReport{}
	// A newly scaffolded template already has its targets; do not read Targets.
	if err := initCopyDocs(r, true, cfg, dir, []string{"doc"}, nil); err != nil || r.changes != 2 {
		t.Fatalf("copy and template: changes=%d err=%v", r.changes, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "Makefile"))
	if err != nil || string(data) != "# template\n" {
		t.Fatalf("template=%q err=%v", data, err)
	}
	path := filepath.Join(dir, "Doc.md")
	data, err = os.ReadFile(path)
	if err != nil || string(data) != "bundled doc\n\n<!-- harnez:stop -->\n" {
		t.Fatalf("doc=%q err=%v", data, err)
	}
	const local = "\n## Local\nkeep my notes\n"
	if err := os.WriteFile(path, append(data, []byte(local)...), 0644); err != nil {
		t.Fatal(err)
	}
	lang := cfg.AgentsMD.Languages["doc"]
	lang.Targets = ""
	cfg.AgentsMD.Languages["doc"] = lang
	again := &initReport{}
	if err := initCopyDocs(again, true, cfg, dir, []string{"doc"}, nil); err != nil || again.changes != 0 {
		t.Fatalf("repeat: changes=%d err=%v", again.changes, err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(data)+local {
		t.Fatalf("local tail changed: %q, %v", got, err)
	}
}

package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

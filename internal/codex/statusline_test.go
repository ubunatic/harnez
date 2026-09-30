package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyStatusLineRemovesInvalidLegacyItemAndKeepsUserItems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := `[tui]
status_line = ["model-context", "model-with-reasoning", "user-item", "current-dir"]
theme = "dark"
`
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := ApplyStatusLine(path)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("ApplyStatusLine did not update the legacy status-line item")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, removed := range []string{"model-context", "model-with-reasoning"} {
		if strings.Contains(got, removed) {
			t.Errorf("replaced status-line item %q remains: %s", removed, got)
		}
	}
	for _, want := range []string{"user-item", "theme = \"dark\"", "\"model\"", "weekly-limit"} {
		if !strings.Contains(got, want) {
			t.Errorf("config does not preserve/add %q: %s", want, got)
		}
	}
}

func TestApplyStatusLineIdempotentAfterLegacyItemRemoval(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if _, err := ApplyStatusLine(path); err != nil {
		t.Fatal(err)
	}
	changed, err := ApplyStatusLine(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("ApplyStatusLine changed the already-updated config")
	}
}

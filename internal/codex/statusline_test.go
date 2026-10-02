package codex

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestApplyStatusLineWritesExactlySpecItemsAndPreservesOtherTUISettings(t *testing.T) {
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
	for _, removed := range []string{"user-item", "context-remaining", "five-hour-limit", "weekly-limit"} {
		if strings.Contains(got, removed) {
			t.Errorf("unexpected status-line item %q remains: %s", removed, got)
		}
	}
	if !strings.Contains(got, "theme = \"dark\"") {
		t.Errorf("unrelated TUI setting was not preserved: %s", got)
	}
	doc := readTOML(path)
	tui := doc["tui"].(map[string]any)
	want := []string{"current-dir", "thread-name", "model", "context-window-size", "context-used"}
	if items := stringArray(tui["status_line"]); !reflect.DeepEqual(items, want) {
		t.Fatalf("status_line = %#v, want exactly %#v", items, want)
	}
}

func TestApplyStatusLineIsIdempotent(t *testing.T) {
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

func TestStatusLineStatusDetectsAdditionalItemsAsDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if _, err := ApplyStatusLine(path); err != nil {
		t.Fatal(err)
	}
	if installed, drifted := StatusLineStatus(path); !installed || drifted {
		t.Fatalf("StatusLineStatus() = (%v, %v), want (true, false)", installed, drifted)
	}
	if err := os.WriteFile(path, []byte(`[tui]
status_line = ["current-dir", "thread-name", "model", "context-window-size", "context-used", "extra"]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if installed, drifted := StatusLineStatus(path); installed || !drifted {
		t.Fatalf("StatusLineStatus() with extra item = (%v, %v), want (false, true)", installed, drifted)
	}
}

func TestRemoveStatusLineRemovesCurrentAndLegacyItems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := `[tui]
status_line = ["current-dir", "context-remaining", "model", "user-item", "weekly-limit"]
`
	if err := os.WriteFile(path, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := RemoveStatusLine(path)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("RemoveStatusLine did not remove managed items")
	}
	doc := readTOML(path)
	tui := doc["tui"].(map[string]any)
	if items := stringArray(tui["status_line"]); !reflect.DeepEqual(items, []string{"user-item"}) {
		t.Fatalf("status_line after removal = %#v, want [user-item]", items)
	}
}

package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Hook entries come from spec/hooks.yaml, so a config.yaml that still lists a
// hooks key is an error rather than a second, silently ignored source (issue
// 1304 M3 follow-up 2).
func TestLoadConfigRejectsHooksKey(t *testing.T) {
	docs := map[string]string{
		"empty list":   "hooks: []\n",
		"custom entry": "hooks:\n  - event: Stop\n    command: harnez hook Stop\n",
	}
	for name, doc := range docs {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadConfig(path)
		if err == nil || !strings.Contains(err.Error(), "remove the hooks key") {
			t.Errorf("%s: LoadConfig error = %v, want a remove-the-hooks-key error", name, err)
		}
	}
}

func TestLoadConfigEmbeddedHasNoHooksKey(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Hooks) != 8 {
		t.Fatalf("hooks derived from spec = %d, want 8", len(cfg.Hooks))
	}
}

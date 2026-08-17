package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTypeDelayMs(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	t.Run("missing file returns 0 and false", func(t *testing.T) {
		val, ok, err := ReadTypeDelayMs(filepath.Join(dir, "nonexistent.toml"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || val != 0 {
			t.Fatalf("expected ok=false, val=0; got ok=%v, val=%d", ok, val)
		}
	})

	t.Run("config without type_delay_ms returns 0 and false", func(t *testing.T) {
		content := `[output]
driver = "dotool"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		val, ok, err := ReadTypeDelayMs(configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || val != 0 {
			t.Fatalf("expected ok=false, val=0; got ok=%v, val=%d", ok, val)
		}
	})

	t.Run("commented type_delay_ms is ignored", func(t *testing.T) {
		content := `[output]
# type_delay_ms = 25
driver = "dotool"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		val, ok, err := ReadTypeDelayMs(configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || val != 0 {
			t.Fatalf("expected ok=false, val=0; got ok=%v, val=%d", ok, val)
		}
	})

	t.Run("active type_delay_ms is read correctly", func(t *testing.T) {
		content := `[output]
  type_delay_ms = 12
driver = "dotool"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		val, ok, err := ReadTypeDelayMs(configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || val != 12 {
			t.Fatalf("expected ok=true, val=12; got ok=%v, val=%d", ok, val)
		}
	})
}

func TestSetTypeDelayMs(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	t.Run("rejects negative delay", func(t *testing.T) {
		err := SetTypeDelayMs(configPath, -1)
		if err == nil || !strings.Contains(err.Error(), "must be >= 0") {
			t.Fatalf("expected negative error, got %v", err)
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		err := SetTypeDelayMs(filepath.Join(dir, "nonexistent.toml"), 10)
		if err == nil {
			t.Fatal("expected error on nonexistent file")
		}
	})

	t.Run("replaces existing type_delay_ms preserving comments", func(t *testing.T) {
		initial := `# Header comment
[output]
# Delay setting
  type_delay_ms  =  5
driver = "dotool"
`
		if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
			t.Fatal(err)
		}
		if err := SetTypeDelayMs(configPath, 20); err != nil {
			t.Fatalf("SetTypeDelayMs: %v", err)
		}
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		expected := `# Header comment
[output]
# Delay setting
  type_delay_ms  =  20
driver = "dotool"
`
		if string(data) != expected {
			t.Fatalf("content mismatch:\ngot:\n%s\nwant:\n%s", string(data), expected)
		}
	})

	t.Run("inserts type_delay_ms after [output] when absent", func(t *testing.T) {
		initial := `# Header comment
[output]
driver = "dotool"
`
		if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
			t.Fatal(err)
		}
		if err := SetTypeDelayMs(configPath, 15); err != nil {
			t.Fatalf("SetTypeDelayMs: %v", err)
		}
		val, ok, err := ReadTypeDelayMs(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if !ok || val != 15 {
			t.Fatalf("expected 15, got ok=%v, val=%d", ok, val)
		}
	})

	t.Run("fails when [output] section is missing entirely", func(t *testing.T) {
		initial := `# No output section
[model]
name = "base.en"
`
		if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
			t.Fatal(err)
		}
		err := SetTypeDelayMs(configPath, 10)
		if err == nil || !strings.Contains(err.Error(), "no [output] section found") {
			t.Fatalf("expected missing [output] error, got %v", err)
		}
	})
}

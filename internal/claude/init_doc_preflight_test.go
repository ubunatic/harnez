package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestInitDocPreflightVariants(t *testing.T) {
	for _, tt := range []struct {
		name, explicit, liteSource, want string
	}{
		{"keep lite", "", "lite.md", "lite"},
		{"explicit full", "full", "lite.md", "full"},
		{"no lite source", "", "", ""},
		{"missing lite source", "", "missing.md", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			original := "<!-- harnez:variant=lite -->\nold\n\n<!-- harnez:stop -->\nlocal\n"
			if err := os.WriteFile(filepath.Join(dir, "Doc.md"), []byte(original), 0644); err != nil {
				t.Fatal(err)
			}
			cfg := &Config{FS: fstest.MapFS{
				"full.md": {Data: []byte("full\n")},
				"lite.md": {Data: []byte("<!-- harnez:variant=lite -->\nlite\n")},
			}, AgentsMD: AgentsMD{Languages: map[string]Language{
				"doc": {Source: "full.md", LiteSource: tt.liteSource, Local: "./Doc.md"},
			}}}
			got, err := initDocPreflight(cfg, dir, []string{"doc", "unknown"}, tt.explicit)
			if err != nil || got["doc"] != tt.want || len(got) != 1 {
				t.Fatalf("variants = %v, err=%v; want doc=%q only", got, err, tt.want)
			}
			data, err := os.ReadFile(filepath.Join(dir, "Doc.md"))
			if err != nil || string(data) != original {
				t.Fatalf("preflight changed destination: %q, %v", data, err)
			}
		})
	}
}

func TestInitDocPreflightChecksEveryDocWithoutWriting(t *testing.T) {
	for _, missingSource := range []bool{false, true} {
		t.Run(map[bool]string{false: "local append", true: "missing source"}[missingSource], func(t *testing.T) {
			dir := t.TempDir()
			original := "bundled\n\n## Local\nkeep\nmore\n"
			if err := os.WriteFile(filepath.Join(dir, "Second.md"), []byte(original), 0644); err != nil {
				t.Fatal(err)
			}
			source, wantError := "doc.md", "appended local section"
			if missingSource {
				source, wantError = "missing.md", "read source"
			}
			cfg := &Config{FS: fstest.MapFS{"doc.md": {Data: []byte("bundled\n")}},
				AgentsMD: AgentsMD{Languages: map[string]Language{
					"first":  {Source: "doc.md", Local: "./First.md"},
					"second": {Source: source, Local: "./Second.md"},
				}}}
			if _, err := initDocPreflight(cfg, dir, []string{"first", "second"}, ""); err == nil || !strings.Contains(err.Error(), wantError) {
				t.Fatalf("preflight error = %v; want %q", err, wantError)
			}
			if _, err := os.Stat(filepath.Join(dir, "First.md")); !os.IsNotExist(err) {
				t.Fatalf("first doc was written before validation: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(dir, "Second.md"))
			if err != nil || string(data) != original {
				t.Fatalf("second doc was modified: %q, %v", data, err)
			}
		})
	}
}

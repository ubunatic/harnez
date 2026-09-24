// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// TestRunInitWithVariantLiteFallsBackSilentlyForDocsWithoutLiteSource covers
// issue 360's acceptance criterion that --variant lite must not error for a
// doc entry with no lite_source — it silently installs the full source
// instead, alongside a sibling doc that does have one.
func TestRunInitWithVariantLiteFallsBackSilentlyForDocsWithoutLiteSource(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# proj\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		FS: fstest.MapFS{
			"go-full.md":   &fstest.MapFile{Data: []byte("# Go full\n")},
			"loop-full.md": &fstest.MapFile{Data: []byte("# Loop full\n")},
			"loop-lite.md": &fstest.MapFile{Data: []byte("<!-- harnez:variant=lite -->\n# Loop tagline\n")},
		},
		AgentsMD: AgentsMD{Languages: map[string]Language{
			"golang":       {Source: "go-full.md", Local: "./docs/Go.md"},
			"agentic-loop": {Source: "loop-full.md", LiteSource: "loop-lite.md", Local: "./docs/AgenticLoop.md"},
		}},
	}
	err := RunInitWithVariant(dir, cfg, []string{"golang", "agentic-loop"}, "", true, false, false, false, nil, false, false, "lite")
	if err != nil {
		t.Fatalf("--variant lite errored for a doc without lite_source: %v", err)
	}
	goData, err := os.ReadFile(filepath.Join(dir, "docs", "Go.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(goData), "# Go full\n\n<!-- harnez:stop -->") {
		t.Fatalf("doc without lite_source did not silently fall back to full content, got:\n%s", goData)
	}
	loopData, err := os.ReadFile(filepath.Join(dir, "docs", "AgenticLoop.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(loopData), "<!-- harnez:variant=lite -->") {
		t.Fatalf("doc with lite_source was not installed as lite under --variant lite, got:\n%s", loopData)
	}
}

func TestRunInitDetectsExistingDocVariant(t *testing.T) {
	tests := []struct {
		name, existing, variant, liteSource, want string
	}{
		{"lite marker retained", "<!-- harnez:variant=lite -->\n# old\n", "", "loop-lite.md", "<!-- harnez:variant=lite -->"},
		{"no marker defaults full", "# old\n", "", "loop-lite.md", "# full"},
		{"explicit full overrides lite", "<!-- harnez:variant=lite -->\n# old\n", "full", "loop-lite.md", "# full"},
		{"missing lite source falls back", "<!-- harnez:variant=lite -->\n# old\n", "", "missing-lite.md", "# full"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# project\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			local := filepath.Join(dir, "docs", "AgenticLoop.md")
			if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(local, []byte(tt.existing), 0o600); err != nil {
				t.Fatal(err)
			}
			files := fstest.MapFS{
				"loop-full.md": &fstest.MapFile{Data: []byte("# full\n")},
			}
			if tt.liteSource == "loop-lite.md" {
				files[tt.liteSource] = &fstest.MapFile{Data: []byte("<!-- harnez:variant=lite -->\n# lite\n")}
			}
			cfg := &Config{FS: files, AgentsMD: AgentsMD{Languages: map[string]Language{
				"agentic-loop": {Source: "loop-full.md", LiteSource: tt.liteSource, Local: "docs/AgenticLoop.md"},
			}}}
			if err := RunInitWithVariant(dir, cfg, []string{"agentic-loop"}, "", true, false, false, false, nil, false, false, tt.variant); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(local)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(got), tt.want) {
				t.Fatalf("doc content = %q, want prefix %q", got, tt.want)
			}
		})
	}
}

func TestSwitchDocVariantSwapsToLiteAndBack(t *testing.T) {
	sourceDir := t.TempDir()
	repoDir := t.TempDir()
	writeCaptureFile(t, sourceDir, "docs/Go.md", "# Go full\n")
	writeCaptureFile(t, sourceDir, "docs/Go.lite.md", "<!-- harnez:variant=lite -->\n# Go tagline\n")
	writeCaptureFile(t, repoDir, "docs/Go.md", "# Go full\n")
	cfg := &Config{
		Dir: sourceDir,
		FS:  os.DirFS(sourceDir),
		AgentsMD: AgentsMD{Languages: map[string]Language{
			"go": {Source: "docs/Go.md", LiteSource: "docs/Go.lite.md", Local: "docs/Go.md"},
		}},
	}
	localPath := filepath.Join(repoDir, "docs", "Go.md")
	if _, err := writeManagedDoc(localPath, []byte("# Go full\n")); err != nil {
		t.Fatal(err)
	}

	changed, err := SwitchDocVariant(repoDir, cfg, "go", "lite")
	if err != nil {
		t.Fatalf("switch to lite: %v", err)
	}
	if !changed {
		t.Fatal("expected switch to lite to report changed=true")
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "<!-- harnez:variant=lite -->") {
		t.Fatalf("local doc missing lite variant marker after switch, got:\n%s", data)
	}

	changed, err = SwitchDocVariant(repoDir, cfg, "go", "full")
	if err != nil {
		t.Fatalf("switch to full: %v", err)
	}
	if !changed {
		t.Fatal("expected switch to full to report changed=true")
	}
	data, err = os.ReadFile(localPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "# Go full\n\n<!-- harnez:stop -->") {
		t.Fatalf("local doc not restored to full content, got:\n%s", data)
	}
}

func TestSwitchDocVariantPreservesLocalStopSection(t *testing.T) {
	sourceDir := t.TempDir()
	repoDir := t.TempDir()
	writeCaptureFile(t, sourceDir, "docs/Go.md", "# Go full\n")
	writeCaptureFile(t, sourceDir, "docs/Go.lite.md", "<!-- harnez:variant=lite -->\n# Go tagline\n")
	writeCaptureFile(t, repoDir, "docs/Go.md", "# Go full\n<!-- harnez:stop -->\n## Local note\n")
	cfg := &Config{
		Dir: sourceDir,
		FS:  os.DirFS(sourceDir),
		AgentsMD: AgentsMD{Languages: map[string]Language{
			"go": {Source: "docs/Go.md", LiteSource: "docs/Go.lite.md", Local: "docs/Go.md"},
		}},
	}
	if _, err := SwitchDocVariant(repoDir, cfg, "go", "lite"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(repoDir, "docs", "Go.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "## Local note") {
		t.Fatalf("local customization was dropped by variant switch, got:\n%s", data)
	}
	if !strings.HasPrefix(string(data), "<!-- harnez:variant=lite -->") {
		t.Fatalf("managed content was not swapped to lite, got:\n%s", data)
	}
}

func TestSwitchDocVariantErrors(t *testing.T) {
	sourceDir := t.TempDir()
	repoDir := t.TempDir()
	writeCaptureFile(t, sourceDir, "docs/Go.md", "# Go full\n")
	cfg := &Config{
		Dir: sourceDir,
		FS:  os.DirFS(sourceDir),
		AgentsMD: AgentsMD{Languages: map[string]Language{
			"go":       {Source: "docs/Go.md", Local: "docs/Go.md"},
			"no-local": {Source: "docs/Go.md"},
		}},
	}

	if _, err := SwitchDocVariant(repoDir, cfg, "go", "medium"); err == nil {
		t.Error("expected error for invalid variant")
	}
	if _, err := SwitchDocVariant(repoDir, cfg, "nope", "lite"); err == nil {
		t.Error("expected error for unknown doc")
	}
	if _, err := SwitchDocVariant(repoDir, cfg, "no-local", "lite"); err == nil {
		t.Error("expected error for doc with no local target")
	}
	if _, err := SwitchDocVariant(repoDir, cfg, "go", "lite"); err == nil {
		t.Error("expected error switching to lite when lite_source is unset")
	}
}

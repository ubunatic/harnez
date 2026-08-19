// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func testConfig() *Config {
	return &Config{
		Docs: []string{"golang", "make"},
		AgentsMD: AgentsMD{
			Languages: map[string]Language{
				"golang":       {Default: "auto"},
				"make":         {Default: "auto"},
				"canary":       {Default: "true"},
				"spec":         {Default: "true"},
				"agentic-loop": {Default: "true"},
			},
		},
	}
}

func TestDocNamesInOrder(t *testing.T) {
	cfg := testConfig()
	got := docNamesInOrder(cfg)
	want := []string{"golang", "make", "agentic-loop", "canary", "spec"} // docs: order, then sorted rest
	if len(got) != len(want) {
		t.Fatalf("docNamesInOrder = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("docNamesInOrder = %v, want %v", got, want)
		}
	}
	// must be stable across calls (issue 011)
	for range 10 {
		again := docNamesInOrder(cfg)
		for i := range want {
			if again[i] != got[i] {
				t.Fatal("docNamesInOrder is not deterministic")
			}
		}
	}
}

func TestValidateDocNames(t *testing.T) {
	cfg := testConfig()
	if err := validateDocNames(cfg, []string{"golang", "canary"}); err != nil {
		t.Fatalf("valid names rejected: %v", err)
	}
	err := validateDocNames(cfg, []string{"golang", "bogus"})
	if err == nil {
		t.Fatal("unknown doc name accepted")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should name the unknown doc: %v", err)
	}
}

func TestDetectDoc(t *testing.T) {
	dir := t.TempDir()
	if detectDoc(dir, "zig") {
		t.Error("detectDoc(empty, zig) should be false")
	}
	// test zig detection via build.zig
	zigFile := dir + "/build.zig"
	if err := testingWriteFile(zigFile, "const std = @import(\"std\");\n"); err != nil {
		t.Fatal(err)
	}
	if !detectDoc(dir, "zig") {
		t.Error("detectDoc(dir with build.zig, zig) should be true")
	}
}

func testingWriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func TestMergeDocs(t *testing.T) {
	tests := []struct {
		name       string
		fromConfig []string
		fromFlag   []string
		want       []string
	}{
		{
			name:       "duplicate in flag",
			fromConfig: []string{"golang"},
			fromFlag:   []string{"make", "make"},
			want:       []string{"golang", "make"},
		},
		{
			name:       "flag duplicates config",
			fromConfig: []string{"golang", "make"},
			fromFlag:   []string{"golang", "canary"},
			want:       []string{"golang", "make", "canary"},
		},
		{
			name:       "multiple duplicates in flag with nil config",
			fromConfig: nil,
			fromFlag:   []string{"canary", "canary", "canary"},
			want:       []string{"canary"},
		},
		{
			name:       "empty inputs",
			fromConfig: nil,
			fromFlag:   nil,
			want:       []string{},
		},
		{
			name:       "empty slices",
			fromConfig: []string{},
			fromFlag:   []string{},
			want:       []string{},
		},
		{
			name:       "duplicate in config preserved once and deduped against flag",
			fromConfig: []string{"golang", "golang"},
			fromFlag:   []string{"golang", "zig", "zig"},
			want:       []string{"golang", "zig"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeDocs(tc.fromConfig, tc.fromFlag)
			if !slices.Equal(got, tc.want) {
				t.Errorf("mergeDocs(%v, %v) = %v; want %v", tc.fromConfig, tc.fromFlag, got, tc.want)
			}
		})
	}
}

func TestAutoDetectDocs_PolyglotMatrix(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}

	tests := []struct {
		name      string
		files     []string
		explicit  []string
		wantOrder []string
	}{
		{
			name: "Go + Zig + C + Shell + Makefile (polyglot like emojig/fontwidth)",
			files: []string{
				"go.mod",
				"build.zig",
				"main.c",
				"scripts/run.sh",
				"Makefile",
			},
			explicit: nil,
			wantOrder: []string{
				"golang", "bash", "make", "zig", "cpp", "markdown", "git", "canary", "spec", "agentic-loop",
			},
		},
		{
			name: "Rust + Zig (like conreel / ziggo)",
			files: []string{
				"Cargo.toml",
				"build.zig.zon",
			},
			explicit: nil,
			wantOrder: []string{
				"rust", "zig", "markdown", "git", "canary", "spec", "agentic-loop",
			},
		},
		{
			name: "Zig standalone with no Makefile (like zterm)",
			files: []string{
				"build.zig",
				"src/main.zig",
			},
			explicit: nil,
			wantOrder: []string{
				"zig", "markdown", "git", "canary", "spec", "agentic-loop",
			},
		},
		{
			name: "Non-standard build driver (Taskfile.yml, requirements.txt, like homeserver/pdf-doctor)",
			files: []string{
				"Taskfile.yml",
				"requirements.txt",
			},
			explicit: nil,
			wantOrder: []string{
				"markdown", "git", "canary", "spec", "agentic-loop",
			},
		},
		{
			name: "Explicit flag deduplicates with auto-detected",
			files: []string{
				"go.mod",
				"Makefile",
			},
			explicit: []string{"golang", "canary"},
			wantOrder: []string{
				"make", "markdown", "git", "spec", "agentic-loop",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tc.files {
				full := dir + "/" + f
				if err := os.MkdirAll(dir+"/"+filepathDir(f), 0o755); err != nil {
					t.Fatalf("mkdir failed: %v", err)
				}
				if err := testingWriteFile(full, "// fixture"); err != nil {
					t.Fatalf("write fixture file %s: %v", full, err)
				}
			}
			got := autoDetectDocs(dir, cfg, tc.explicit)
			if !slices.Equal(got, tc.wantOrder) {
				t.Errorf("autoDetectDocs() = %v, want %v", got, tc.wantOrder)
			}
		})
	}
}

func filepathDir(p string) string {
	idx := strings.LastIndex(p, "/")
	if idx <= 0 {
		return "."
	}
	return p[:idx]
}



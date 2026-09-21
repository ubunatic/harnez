// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func testConfig() *Config {
	return &Config{
		Docs: []string{"golang", "make"},
		DocsProfiles: map[string][]string{
			"core": {"agentic-loop", "spec"},
			"dev":  {"agentic-loop", "spec", "golang"},
		},
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

func TestResolveDocDependenciesTransitiveAndDeterministic(t *testing.T) {
	cfg := &Config{AgentsMD: AgentsMD{Languages: map[string]Language{
		"leaf":   {DependsOn: []string{"middle"}},
		"middle": {DependsOn: []string{"base"}},
		"base":   {},
		"other":  {},
	}}}
	got, err := resolveDocDependencies(cfg, []string{"leaf", "other", "leaf"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"base", "middle", "leaf", "other"}; !slices.Equal(got, want) {
		t.Fatalf("closure = %v, want %v", got, want)
	}
}

func TestResolveDocDependenciesActionableErrors(t *testing.T) {
	t.Run("unknown", func(t *testing.T) {
		cfg := &Config{AgentsMD: AgentsMD{Languages: map[string]Language{
			"top": {DependsOn: []string{"missing"}},
		}}}
		_, err := resolveDocDependencies(cfg, []string{"top"})
		if err == nil || !strings.Contains(err.Error(), `doc "top" depends on unknown doc "missing"`) ||
			!strings.Contains(err.Error(), "define it or remove depends_on") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("cycle", func(t *testing.T) {
		cfg := &Config{AgentsMD: AgentsMD{Languages: map[string]Language{
			"a": {DependsOn: []string{"b"}}, "b": {DependsOn: []string{"a"}},
		}}}
		_, err := resolveDocDependencies(cfg, []string{"a"})
		if err == nil || !strings.Contains(err.Error(), "dependency cycle") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidateCopyableDocCatalogClassifiesReferences(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{"undeclared hard", "See [base](Base.md).\n", `undeclared hard reference "Base.md"`},
		{"missing illustrative", "See [study](../studies/example.md).\n", `repository-relative illustrative reference "../studies/example.md"`},
		{"bare missing illustrative", "See `docs/studies/example.md`.\n", `references unavailable project material "docs/studies/example.md"`},
		{"destination assumption", "Write findings to `docs/feedback/`.\n", `assumes project destination "docs/feedback/"`},
		{"optional destination without fallback", "Use `docs/feedback/` when present.\n", `assumes project destination "docs/feedback/"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				FS: fstest.MapFS{
					"top.md":  &fstest.MapFile{Data: []byte(tc.doc)},
					"base.md": &fstest.MapFile{Data: []byte("# Base\n")},
				},
				AgentsMD: AgentsMD{Languages: map[string]Language{
					"top":  {Source: "top.md", Local: "docs/Top.md"},
					"base": {Source: "base.md", Local: "docs/Base.md"},
				}},
			}
			err := ValidateCopyableDocCatalog(cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestValidateCopyableDocCatalogRejectsImplicitCapability(t *testing.T) {
	cfg := &Config{
		FS: fstest.MapFS{"deploy.md": &fstest.MapFile{Data: []byte("# Deploy\n")}},
		AgentsMD: AgentsMD{Languages: map[string]Language{
			"deploy": {Source: "deploy.md", Local: "docs/Deploy.md", Default: "true", Capabilities: []string{"remote-deployment"}},
		}},
	}
	err := ValidateCopyableDocCatalog(cfg)
	if err == nil || !strings.Contains(err.Error(), "must use default: false") {
		t.Fatalf("unexpected capability validation error: %v", err)
	}
}

func TestRunInitEmojigShapeCopiesHardDependencyWithoutCapabilityBoilerplate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# emojig\n\nNo daemon or remote host.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loop := "See [tracking](IssueTracking.md). Optional examples are explained inline.\n"
	cfg := &Config{
		FS: fstest.MapFS{
			"loop.md":   &fstest.MapFile{Data: []byte(loop)},
			"issues.md": &fstest.MapFile{Data: []byte("# Tracking\n")},
			"deploy.md": &fstest.MapFile{Data: []byte("# Remote deployment\n")},
		},
		AgentsMD: AgentsMD{Languages: map[string]Language{
			"agentic-loop":            {Source: "loop.md", Local: "./docs/AgenticLoop.md", DependsOn: []string{"issue-tracking"}},
			"issue-tracking":          {Source: "issues.md", Local: "./docs/IssueTracking.md"},
			"deployment-transparency": {Source: "deploy.md", Local: "./docs/DeploymentTransparency.md", Capabilities: []string{"remote-deployment"}},
		}},
	}
	if err := RunInit(dir, cfg, []string{"agentic-loop"}, "", true, false, false, false); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, "docs", "AgenticLoop.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := RunInit(dir, cfg, []string{"agentic-loop"}, "", true, false, false, false); err != nil {
		t.Fatalf("idempotent rerun: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(dir, "docs", "AgenticLoop.md"))
	if err != nil || !slices.Equal(first, second) {
		t.Fatalf("rerun changed copied doc: %v", err)
	}
	for _, name := range []string{"AgenticLoop.md", "IssueTracking.md"} {
		if _, err := os.Stat(filepath.Join(dir, "docs", name)); err != nil {
			t.Fatalf("hard dependency %s not copied: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "DeploymentTransparency.md")); !os.IsNotExist(err) {
		t.Fatalf("capability doc copied without opt-in: %v", err)
	}
	for _, entry := range []string{"AgenticLoop.md", "IssueTracking.md"} {
		data, err := fs.ReadFile(os.DirFS(filepath.Join(dir, "docs")), entry)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "docs/studies/") {
			t.Fatalf("missing illustrative reference survived in %s", entry)
		}
	}
}

func TestEmbeddedCopyableDocGraph(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	all := docNamesInOrder(cfg)
	closure, err := resolveDocDependencies(cfg, all)
	if err != nil {
		t.Fatalf("invalid complete copied-doc graph: %v", err)
	}
	if len(closure) != len(all) {
		t.Fatalf("complete graph lost nodes: got %d, want %d", len(closure), len(all))
	}
	if err := ValidateCopyableDocCatalog(cfg); err != nil {
		t.Fatalf("invalid complete copied-doc reference graph: %v", err)
	}
	loop := cfg.AgentsMD.Languages["agentic-loop"]
	if !slices.Equal(loop.DependsOn, []string{"issue-tracking"}) {
		t.Fatalf("Agentic Loop hard dependencies = %v", loop.DependsOn)
	}
	deploy := cfg.AgentsMD.Languages["deployment-transparency"]
	if deploy.Default != "false" || !slices.Equal(deploy.Capabilities, []string{"remote-deployment"}) {
		t.Fatalf("deployment capability metadata = default:%q capabilities:%v", deploy.Default, deploy.Capabilities)
	}
	data, err := fs.ReadFile(cfg.FS, loop.Source)
	if err != nil {
		t.Fatal(err)
	}
	for _, dangling := range []string{
		"docs/studies/2026-08-30-usage-panel-integration-and-the-agy-collector-cliff.md",
		"docs/studies/2026-09-04-three-days-to-a-public-release.md",
		"](DeploymentTransparency.md)",
	} {
		if strings.Contains(string(data), dangling) {
			t.Errorf("portable Agentic Loop retains unavailable reference %q", dangling)
		}
	}
}

func TestRunInitEmbeddedEmojigShapeHasNoUnavailableProjectMaterial(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# emojig\n\nNo daemon or remote host.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RunInit(dir, cfg, []string{"agentic-loop"}, "", true, false, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "IssueTracking.md")); err != nil {
		t.Fatalf("hard dependency missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "DeploymentTransparency.md")); !os.IsNotExist(err) {
		t.Fatalf("inapplicable capability doc copied: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "docs"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, "docs", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if match := unavailableProjectDocRE.Find(data); match != nil {
			t.Errorf("%s retains unavailable project material %q", entry.Name(), match)
		}
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
	if err := validateDocNames(cfg, []string{"golang", "canary", "core", "dev", "full", "all"}); err != nil {
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
	if detectDoc(dir, "gorelease") {
		t.Error("detectDoc(empty, gorelease) should be false")
	}
	if err := testingWriteFile(filepath.Join(dir, "go.mod"), "module example\n\ngo 1.23\n"); err != nil {
		t.Fatal(err)
	}
	if !detectDoc(dir, "gorelease") {
		t.Error("detectDoc(dir with go.mod, gorelease) should be true")
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
				"agentic-loop", "bash", "canary", "cpp", "git", "golang", "gorelease", "issue-tracking", "make", "markdown", "spec", "zig",
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
				"agentic-loop", "canary", "git", "issue-tracking", "markdown", "rust", "spec", "zig",
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
				"agentic-loop", "canary", "git", "issue-tracking", "markdown", "spec", "zig",
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
				"agentic-loop", "canary", "git", "issue-tracking", "markdown", "spec",
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
				"agentic-loop", "git", "gorelease", "issue-tracking", "make", "markdown", "spec",
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

// TestAllConfigDeclaredCopyableDocsHaveBundledMarker is a guard that ensures every
// source file declared under agents_md.languages in config.yaml carries the
// <!-- harnez:bundled --> marker. It reads from the embedded FS so it covers
// exactly the set that actually gets installed — not a broader glob.
// When a new copyable doc is added to config.yaml without the marker, this test fails.
func TestAllConfigDeclaredCopyableDocsHaveBundledMarker(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	const marker = "<!-- harnez:bundled -->"
	for name, lang := range cfg.AgentsMD.Languages {
		if lang.Source == "" {
			continue
		}
		data, err := fs.ReadFile(cfg.FS, lang.Source)
		if err != nil {
			t.Errorf("doc %q: cannot read source %q from embedded FS: %v", name, lang.Source, err)
			continue
		}
		if !strings.Contains(string(data), marker) {
			t.Errorf("doc %q (source: %q) is missing %s", name, lang.Source, marker)
		}
	}
}

func TestCanaryDoesNotHardReferenceOptionalPrototypingDoc(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("load embedded config: %v", err)
	}
	data, err := fs.ReadFile(cfg.FS, "docs/other/Canary.md")
	if err != nil {
		t.Fatalf("read bundled Canary.md: %v", err)
	}
	if strings.Contains(string(data), "@docs/PrototypingFeatures.md") {
		t.Fatal("bundled Canary.md must not hard-reference optional PrototypingFeatures.md")
	}
}

func TestConfigDefaultZeroDocs(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	if len(cfg.Docs) != 0 {
		t.Fatalf("expected default config.yaml docs to be empty [], got %v", cfg.Docs)
	}
}

func TestExpandDocNames(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	allDocs := docNamesInOrder(cfg)
	if len(allDocs) == 0 {
		t.Fatal("expected non-empty catalog of docs")
	}

	// all expands to all docs in order
	expandedAll := expandDocNames(cfg, []string{"all"})
	if !slices.Equal(expandedAll, allDocs) {
		t.Fatalf("expandDocNames(all) = %v, want %v", expandedAll, allDocs)
	}

	// core profile
	expandedCore := expandDocNames(cfg, []string{"core"})
	wantCore := []string{"agentic-loop", "issue-tracking"}
	if !slices.Equal(expandedCore, wantCore) {
		t.Fatalf("expandDocNames(core) = %v, want %v", expandedCore, wantCore)
	}

	// dev profile
	expandedDev := expandDocNames(cfg, []string{"dev"})
	wantDev := []string{"agentic-loop", "issue-tracking", "bash", "git"}
	if !slices.Equal(expandedDev, wantDev) {
		t.Fatalf("expandDocNames(dev) = %v, want %v", expandedDev, wantDev)
	}

	// full profile
	expandedFull := expandDocNames(cfg, []string{"full"})
	wantFull := cfg.DocsProfiles["full"]
	if !slices.Equal(expandedFull, wantFull) {
		t.Fatalf("expandDocNames(full) = %v, want %v", expandedFull, wantFull)
	}

	// mixed with explicit doc and deduplication
	mixed := expandDocNames(cfg, []string{"core", "golang", "core"})
	wantMixed := []string{"agentic-loop", "issue-tracking", "golang"}
	if !slices.Equal(mixed, wantMixed) {
		t.Fatalf("expandDocNames(core, golang, core) = %v, want %v", mixed, wantMixed)
	}

	// deduplication across profiles
	dedup := expandDocNames(cfg, []string{"core", "dev", "core"})
	wantDedup := []string{"agentic-loop", "issue-tracking", "bash", "git"}
	if !slices.Equal(dedup, wantDedup) {
		t.Fatalf("expandDocNames(core, dev, core) = %v, want %v", dedup, wantDedup)
	}

	specific := expandDocNames(cfg, []string{"golang", "bash"})
	if want := []string{"golang", "bash"}; !slices.Equal(specific, want) {
		t.Fatalf("expandDocNames(golang, bash) = %v, want %v", specific, want)
	}
}

func TestApplyOptInDocsAndCleanUnmanaged(t *testing.T) {
	targetDir := t.TempDir()
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	cfg.PrimeAgentTarget = ""

	// 1. Bare apply with default zero docs -> 0 docs written
	if err := ApplyAll(targetDir, cfg, nil, false, false); err != nil {
		t.Fatalf("ApplyAll failed: %v", err)
	}
	goDoc := filepath.Join(targetDir, "docs", "Go.md")
	bashDoc := filepath.Join(targetDir, "docs", "Bash.md")
	if _, err := os.Stat(goDoc); !os.IsNotExist(err) {
		t.Fatalf("expected Go.md not to exist after bare apply")
	}

	// 2. Opt-in apply with --docs golang -> only Go.md is installed
	if err := ApplyAll(targetDir, cfg, []string{"golang"}, false, false); err != nil {
		t.Fatalf("ApplyAll with golang failed: %v", err)
	}
	if _, err := os.Stat(goDoc); err != nil {
		t.Fatalf("expected Go.md to exist after opt-in apply: %v", err)
	}
	if _, err := os.Stat(bashDoc); !os.IsNotExist(err) {
		t.Fatalf("expected Bash.md not to exist when only golang requested")
	}

	// 3. CleanUnmanagedDocs with keepDocs []string{"bash"} -> Go.md removed
	removed, err := CleanUnmanagedDocs(targetDir, cfg, []string{"bash"})
	if err != nil {
		t.Fatalf("CleanUnmanagedDocs failed: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed doc, got %d", removed)
	}
	if _, err := os.Stat(goDoc); !os.IsNotExist(err) {
		t.Fatalf("expected Go.md to be removed by CleanUnmanagedDocs")
	}

	// 4. Opt-in apply with --docs all -> Go.md and Bash.md installed
	if err := ApplyAll(targetDir, cfg, []string{"all"}, false, false); err != nil {
		t.Fatalf("ApplyAll with all failed: %v", err)
	}
	if _, err := os.Stat(goDoc); err != nil {
		t.Fatalf("expected Go.md to exist after apply all: %v", err)
	}
	if _, err := os.Stat(bashDoc); err != nil {
		t.Fatalf("expected Bash.md to exist after apply all: %v", err)
	}

	// 5. CleanAll -> removes all docs
	if err := CleanAll(targetDir, cfg); err != nil {
		t.Fatalf("CleanAll failed: %v", err)
	}
	if _, err := os.Stat(goDoc); !os.IsNotExist(err) {
		t.Fatalf("expected Go.md to be removed by CleanAll")
	}
	if _, err := os.Stat(bashDoc); !os.IsNotExist(err) {
		t.Fatalf("expected Bash.md to be removed by CleanAll")
	}
}

func TestApplyDocsProfileCore(t *testing.T) {
	targetDir := t.TempDir()
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded failed: %v", err)
	}
	cfg.PrimeAgentTarget = ""

	if err := ApplyAll(targetDir, cfg, []string{"core"}, false, false); err != nil {
		t.Fatalf("ApplyAll with core failed: %v", err)
	}
	agenticDoc := filepath.Join(targetDir, "docs", "AgenticLoop.md")
	issueDoc := filepath.Join(targetDir, "docs", "IssueTracking.md")
	goDoc := filepath.Join(targetDir, "docs", "Go.md")
	bashDoc := filepath.Join(targetDir, "docs", "Bash.md")

	if _, err := os.Stat(agenticDoc); err != nil {
		t.Fatalf("expected AgenticLoop.md to exist: %v", err)
	}
	if _, err := os.Stat(issueDoc); err != nil {
		t.Fatalf("expected IssueTracking.md to exist: %v", err)
	}
	if _, err := os.Stat(goDoc); !os.IsNotExist(err) {
		t.Fatalf("expected Go.md not to exist with core profile")
	}
	if _, err := os.Stat(bashDoc); !os.IsNotExist(err) {
		t.Fatalf("expected Bash.md not to exist with core profile")
	}
}

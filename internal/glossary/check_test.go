package glossary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"AGENTS.md", "AGENTS.md", true},
		{"AGENTS.md", "docs/AGENTS.md", false},
		{".harnez/rules/*.md", ".harnez/rules/Output.md", true},
		{".harnez/rules/*.md", ".harnez/rules/sub/Output.md", false},
		{"docs/**/*.md", "docs/Spec.md", true},
		{"docs/**/*.md", "docs/other/Spec.md", true},
		{"docs/**/*.md", "docs/a/b/c.md", true},
		{"docs/**/*.md", "docs/Spec.txt", false},
		{"docs/studies/**", "docs/studies/983-retro.md", true},
		{"docs/studies/**", "docs/studies/2026/983.md", true},
		{"**/*_test.go", "internal/glossary/check_test.go", true},
		{"**/*_test.go", "check_test.go", true},
		{"**/*_test.go", "internal/glossary/check.go", false},
		{"cmd/**/*.go", "cmd/harnez/main.go", true},
		{"issues/CLOSED.md", "issues/CLOSED.md", true},
	}

	for _, tc := range tests {
		got := MatchGlob(tc.pattern, tc.path)
		if got != tc.want {
			t.Errorf("MatchGlob(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestCheckFileWordBoundariesAndCaseSensitivity(t *testing.T) {
	g, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		name       string
		path       string
		content    string
		wantCount  int
		wantLine   int
		wantCol    int
		wantPhrase string
		wantRepl   string
	}{
		{
			name:       "matches case-insensitive phrase on word boundary",
			path:       "docs/Guide.md",
			content:    "Check the Context Used before compaction.\n",
			wantCount:  1,
			wantLine:   1,
			wantCol:    11,
			wantPhrase: "Context Used",
			wantRepl:   `"context usage"`,
		},
		{
			name:      "ignores partial word prefix subcontext size",
			path:      "docs/Guide.md",
			content:   "The subcontext size is unrelated.\n",
			wantCount: 0,
		},
		{
			name:       "case-sensitive phrase matches exact uppercase CONTEXT_USED",
			path:       "docs/Guide.md",
			content:    "Column header CONTEXT_USED is deprecated.\n",
			wantCount:  1,
			wantLine:   1,
			wantCol:    15,
			wantPhrase: "CONTEXT_USED",
			wantRepl:   `"CONTEXT_USAGE"`,
		},
		{
			name:       "shared forbidden phrase deduplicates into single finding with combined replacement",
			path:       "docs/Guide.md",
			content:    "Do not rely on context size here.\n",
			wantCount:  1,
			wantLine:   1,
			wantCol:    16,
			wantPhrase: "context size",
			wantRepl:   `"context usage" or "context window"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			findings := g.CheckFile(tc.path, []byte(tc.content))
			if len(findings) != tc.wantCount {
				t.Fatalf("len(findings) = %d, want %d; findings = %+v", len(findings), tc.wantCount, findings)
			}
			if tc.wantCount == 0 {
				return
			}
			f := findings[0]
			if f.Line != tc.wantLine || f.Col != tc.wantCol {
				t.Errorf("pos = %d:%d, want %d:%d", f.Line, f.Col, tc.wantLine, tc.wantCol)
			}
			if f.Phrase != tc.wantPhrase {
				t.Errorf("Phrase = %q, want %q", f.Phrase, tc.wantPhrase)
			}
			if f.Replacement != tc.wantRepl {
				t.Errorf("Replacement = %q, want %q", f.Replacement, tc.wantRepl)
			}
		})
	}
}

func TestCheckFileMarkdownFencesAndClosedIssues(t *testing.T) {
	g, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	t.Run("skips fenced code blocks in markdown", func(t *testing.T) {
		md := strings.Join([]string{
			"# Heading",
			"```go",
			"// context size inside backtick fence should be ignored",
			"```",
			"~~~yaml",
			"note: context length inside tilde fence should be ignored",
			"~~~",
			"Prose outside fence mentioning context used is flagged.",
		}, "\n")

		findings := g.CheckFile("docs/Architecture.md", []byte(md))
		if len(findings) != 1 {
			t.Fatalf("len(findings) = %d, want 1: %+v", len(findings), findings)
		}
		if findings[0].Line != 8 || findings[0].Phrase != "context used" {
			t.Errorf("finding = %+v, want line 8 'context used'", findings[0])
		}
	})

	t.Run("skips closed issue tickets", func(t *testing.T) {
		closedIssue := "# 100 — Old ticket\n\n**Status**: Closed\n**Priority**: P2\n\nMentions context size and CONTEXT_SIZE.\n"
		if findings := g.CheckFile("issues/100-old.md", []byte(closedIssue)); len(findings) != 0 {
			t.Errorf("expected 0 findings for closed issue, got %+v", findings)
		}

		openIssue := "# 101 — Open ticket\n\n**Status**: Open\n**Priority**: P2\n\nMentions context used.\n"
		if findings := g.CheckFile("issues/101-open.md", []byte(openIssue)); len(findings) != 1 {
			t.Errorf("expected 1 finding for open issue, got %+v", findings)
		}
	})

	t.Run("honors harnez-lint:ignore glossary comment", func(t *testing.T) {
		md := strings.Join([]string{
			"<!-- harnez-lint:ignore glossary -- historical quote -->",
			"This line mentions context size.",
			"And this line has context used. <!-- harnez-lint:ignore glossary -- inline -->",
			"Unsuppressed context length on line 4.",
		}, "\n")

		findings := g.CheckFile("docs/Notes.md", []byte(md))
		if len(findings) != 1 || findings[0].Line != 4 {
			t.Fatalf("findings = %+v, want only line 4 finding", findings)
		}
	})
}

func TestCheckFileGoCommentsAndStrings(t *testing.T) {
	g, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	src := strings.Join([]string{
		"package subagent",
		"",
		"// compute checks context used for the session.",
		"/*First line of block comment",
		"  second line has context size here",
		"*/",
		"func compute() string {",
		"\tvar context_size = 1 // identifier ignored",
		"\t_ = context_size",
		"\treturn \"header CONTEXT_SIZE value\"",
		"}",
	}, "\n")

	findings := g.CheckFile("internal/subagent/sample.go", []byte(src))
	if len(findings) != 3 {
		t.Fatalf("len(findings) = %d, want 3: %+v", len(findings), findings)
	}

	// Finding 1: line 3 "// compute checks context used..." -> col 19
	if findings[0].Line != 3 || findings[0].Col != 19 || findings[0].Phrase != "context used" {
		t.Errorf("findings[0] = %+v, want 3:19 'context used'", findings[0])
	}
	// Finding 2: line 5 "  second line has context size here" -> col 19
	if findings[1].Line != 5 || findings[1].Col != 19 || findings[1].Phrase != "context size" {
		t.Errorf("findings[1] = %+v, want 5:19 'context size'", findings[1])
	}
	// Finding 3: line 10 "\treturn \"header CONTEXT_SIZE value\"" -> col 17
	if findings[2].Line != 10 || findings[2].Col != 17 || findings[2].Phrase != "CONTEXT_SIZE" {
		t.Errorf("findings[2] = %+v, want 10:17 'CONTEXT_SIZE'", findings[2])
	}

	// Verify test files are excluded by default scope
	if testFindings := g.CheckFile("internal/subagent/sample_test.go", []byte(src)); len(testFindings) != 0 {
		t.Errorf("expected *_test.go to be excluded by default scope, got %+v", testFindings)
	}
}

func TestCheckDir(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, body string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite("docs/Active.md", "Avoid context used in docs.\n")
	mustWrite("docs/studies/Study.md", "Historical context size in study is excluded.\n")
	mustWrite("issues/CLOSED.md", "Closed index context size is excluded.\n")

	g, err := LoadForRepo(dir)
	if err != nil {
		t.Fatalf("LoadForRepo() error = %v", err)
	}
	findings, err := g.CheckDir(dir)
	if err != nil {
		t.Fatalf("CheckDir() error = %v", err)
	}
	if len(findings) != 1 || findings[0].File != "docs/Active.md" {
		t.Fatalf("findings = %+v, want single finding in docs/Active.md", findings)
	}
}

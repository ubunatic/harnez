package glossary

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadBaseGlossary(t *testing.T) {
	g, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if g.Version != 1 {
		t.Errorf("Version = %d, want 1", g.Version)
	}
	wantTerms := []string{"context usage", "context window", "compaction limit", "cached re-read"}
	if len(g.Terms) != len(wantTerms) {
		t.Fatalf("len(Terms) = %d, want %d", len(g.Terms), len(wantTerms))
	}
	for i, want := range wantTerms {
		if g.Terms[i].Term != want {
			t.Errorf("Terms[%d].Term = %q, want %q", i, g.Terms[i].Term, want)
		}
		if g.Terms[i].Status != StatusPreferred {
			t.Errorf("Terms[%d].Status = %q, want %q", i, g.Terms[i].Status, StatusPreferred)
		}
	}
	if !reflect.DeepEqual(g.Sources, []string{BaseSpecPath}) {
		t.Errorf("Sources = %v, want %v", g.Sources, []string{BaseSpecPath})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid minimal glossary",
			yaml: `
version: 1
terms:
  - term: "context usage"
    definition: "Tokens read on last call."
`,
			wantErr: false,
		},
		{
			name: "valid full fields",
			yaml: `
version: 1
include_base: false
default_scope:
  include: ["docs/**/*.md"]
  exclude: ["docs/studies/**"]
terms:
  - term: "context usage"
    status: "preferred"
    enabled: true
    definition: "Tokens read on last call."
    synonyms: ["active context"]
    code_names: ["ContextTokens"]
    forbidden:
      - phrase: "context used"
        replacement: "context usage"
        reason: "canonical noun phrase"
        case_sensitive: true
    scope:
      include: ["cmd/**/*.go"]
    examples: ["context usage 300k"]
    see_also: ["context window"]
`,
			wantErr: false,
		},
		{
			name: "missing version",
			yaml: `
terms:
  - term: "foo"
    definition: "bar"
`,
			wantErr: true,
		},
		{
			name: "unsupported version",
			yaml: `
version: 2
terms:
  - term: "foo"
    definition: "bar"
`,
			wantErr: true,
		},
		{
			name: "missing terms",
			yaml: `
version: 1
`,
			wantErr: true,
		},
		{
			name: "term missing definition",
			yaml: `
version: 1
terms:
  - term: "foo"
`,
			wantErr: true,
		},
		{
			name: "unknown top level property",
			yaml: `
version: 1
unknown_key: "bad"
terms:
  - term: "foo"
    definition: "bar"
`,
			wantErr: true,
		},
		{
			name: "unknown term property rejected by additionalProperties",
			yaml: `
version: 1
terms:
  - term: "foo"
    definition: "bar"
    extra: 123
`,
			wantErr: true,
		},
		{
			name: "invalid status enum",
			yaml: `
version: 1
terms:
  - term: "foo"
    status: "archived"
    definition: "bar"
`,
			wantErr: true,
		},
		{
			name: "forbidden missing replacement",
			yaml: `
version: 1
terms:
  - term: "foo"
    definition: "bar"
    forbidden:
      - phrase: "bad"
`,
			wantErr: true,
		},
		{
			name:    "malformed yaml syntax",
			yaml:    "version: [unclosed",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate([]byte(tc.yaml))
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestLoadForRepoAndMerge(t *testing.T) {
	t.Run("no override file returns base glossary", func(t *testing.T) {
		dir := t.TempDir()
		g, err := LoadForRepo(dir)
		if err != nil {
			t.Fatalf("LoadForRepo() error = %v", err)
		}
		if len(g.Terms) != 4 {
			t.Fatalf("len(Terms) = %d, want 4", len(g.Terms))
		}
		if !reflect.DeepEqual(g.Sources, []string{BaseSpecPath}) {
			t.Errorf("Sources = %v, want %v", g.Sources, []string{BaseSpecPath})
		}
	})

	t.Run("override replaces matching term case-insensitively, disables another, and appends new term", func(t *testing.T) {
		dir := t.TempDir()
		harnezDir := filepath.Join(dir, ".harnez")
		if err := os.MkdirAll(harnezDir, 0o755); err != nil {
			t.Fatal(err)
		}
		overrideYAML := `
version: 1
default_scope:
  include: ["custom/**/*.md"]
terms:
  - term: "Context Usage"
    definition: "Custom project definition for context usage."
  - term: "cached re-read"
    definition: "Disabled in this repo."
    enabled: false
  - term: "worktree isolation"
    definition: "Running a subagent in a dedicated git worktree."
`
		if err := os.WriteFile(filepath.Join(harnezDir, "glossary.yaml"), []byte(overrideYAML), 0o644); err != nil {
			t.Fatal(err)
		}

		g, err := LoadForRepo(dir)
		if err != nil {
			t.Fatalf("LoadForRepo() error = %v", err)
		}

		wantOrder := []string{"Context Usage", "context window", "compaction limit", "worktree isolation"}
		if len(g.Terms) != len(wantOrder) {
			t.Fatalf("len(Terms) = %d, want %d", len(g.Terms), len(wantOrder))
		}
		for i, want := range wantOrder {
			if g.Terms[i].Term != want {
				t.Errorf("Terms[%d].Term = %q, want %q", i, g.Terms[i].Term, want)
			}
		}
		if g.Terms[0].Definition != "Custom project definition for context usage." {
			t.Errorf("Terms[0].Definition = %q, want custom definition", g.Terms[0].Definition)
		}
		if !reflect.DeepEqual(g.DefaultScope.Include, []string{"custom/**/*.md"}) {
			t.Errorf("DefaultScope.Include = %v, want [custom/**/*.md]", g.DefaultScope.Include)
		}
		wantSources := []string{BaseSpecPath, RepoOverrideRel}
		if !reflect.DeepEqual(g.Sources, wantSources) {
			t.Errorf("Sources = %v, want %v", g.Sources, wantSources)
		}
	})

	t.Run("include_base false omits base terms", func(t *testing.T) {
		dir := t.TempDir()
		harnezDir := filepath.Join(dir, ".harnez")
		if err := os.MkdirAll(harnezDir, 0o755); err != nil {
			t.Fatal(err)
		}
		overrideYAML := `
version: 1
include_base: false
terms:
  - term: "only local"
    definition: "Local term only."
`
		if err := os.WriteFile(filepath.Join(harnezDir, "glossary.yaml"), []byte(overrideYAML), 0o644); err != nil {
			t.Fatal(err)
		}

		g, err := LoadForRepo(dir)
		if err != nil {
			t.Fatalf("LoadForRepo() error = %v", err)
		}
		if len(g.Terms) != 1 || g.Terms[0].Term != "only local" {
			t.Fatalf("Terms = %+v, want single 'only local' term", g.Terms)
		}
		if !reflect.DeepEqual(g.Sources, []string{RepoOverrideRel}) {
			t.Errorf("Sources = %v, want [%s]", g.Sources, RepoOverrideRel)
		}
	})

	t.Run("invalid override yaml returns error", func(t *testing.T) {
		dir := t.TempDir()
		harnezDir := filepath.Join(dir, ".harnez")
		if err := os.MkdirAll(harnezDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(harnezDir, "glossary.yaml"), []byte("version: 99\nterms: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := LoadForRepo(dir)
		if err == nil || !strings.Contains(err.Error(), RepoOverrideRel) {
			t.Fatalf("LoadForRepo() error = %v, want error mentioning %s", err, RepoOverrideRel)
		}
	})
}

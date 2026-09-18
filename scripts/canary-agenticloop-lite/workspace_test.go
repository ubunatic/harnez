package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestExpectedWorkspaceFiles(t *testing.T) {
	docs := []string{"/assets/AgenticLoop.lite.md", "/assets/Bash.lite.md"}
	checks := []struct {
		link, delivery string
		want           []string
	}{
		{"soft", "native", []string{"AGENTS.md"}},
		{"hard", "native", []string{"AGENTS.md", "AgenticLoop.lite.md", "Bash.lite.md"}},
		{"soft", "png", []string{"AGENTS.md", "AgenticLoop.lite.png", "Bash.lite.png"}},
	}
	for _, check := range checks {
		if got := expectedWorkspaceFiles(docs, check.link, check.delivery); !reflect.DeepEqual(got, check.want) {
			t.Errorf("%s/%s: got %#v, want %#v", check.link, check.delivery, got, check.want)
		}
	}
}

func TestCustomLinkFormats(t *testing.T) {
	saved := fixtureLinks
	defer func() { fixtureLinks = saved }()

	fixtureLinks = map[string]string{
		"soft": "See doc: {ref}",
		"hard": "Include: @{ref}",
	}

	work := t.TempDir()
	docPath := work + "/test.md"
	if err := os.WriteFile(docPath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := setupLinkedWorkspace(work, work, []string{docPath}, "soft", "native"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(work + "/AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "See doc: test.md") {
		t.Fatalf("expected custom soft link format, got:\n%s", string(content))
	}
}

func TestDefaultFrontmatterTitleInLinks(t *testing.T) {
	saved := fixtureLinks
	defer func() { fixtureLinks = saved }()

	fixtureLinks = map[string]string{
		"soft": "See {ref} for {title}.",
	}

	work := t.TempDir()
	docPath := work + "/Doc.md"
	docContent := "---\ntitle: Document Conventions\n---\n# Document Conventions\n\nContent here."
	if err := os.WriteFile(docPath, []byte(docContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := setupLinkedWorkspace(work, work, []string{docPath}, "soft", "native"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(work + "/AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "See Doc.md for Document Conventions.") {
		t.Fatalf("expected frontmatter title in soft link, got:\n%s", string(content))
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedWorkspaceVariantsAndDeliveries(t *testing.T) {
	root, cleanup, err := embeddedRepo()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for _, variant := range []string{"full", "lite"} {
		doc := filepath.Join(root, "docs", "AgenticLoop.md")
		if variant == "lite" {
			doc = filepath.Join(root, "docs", "practices", "AgenticLoop.lite.md")
		}
		for _, delivery := range []string{"native", "png"} {
			for _, link := range []string{"soft", "hard"} {
				work := t.TempDir()
				if err := setupLinkedWorkspace(work, root, []string{doc}, link, delivery); err != nil {
					t.Fatalf("%s/%s/%s: %v", variant, delivery, link, err)
				}
				if _, err := os.Stat(filepath.Join(work, "AGENTS.md")); err != nil {
					t.Fatalf("%s/%s/%s: missing AGENTS.md: %v", variant, delivery, link, err)
				}
			}
		}
	}
}

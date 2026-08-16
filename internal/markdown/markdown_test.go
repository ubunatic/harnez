package markdown_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/markdown"
)

func TestMarkdownMarkers_HarnezAndLegacy(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "AGENTS.md")

	// 1. Initial write with modern harnez marker
	changed, existed, err := markdown.Apply(mdFile, "Test Section", "Modern content")
	if err != nil {
		t.Fatalf("markdown.Apply failed: %v", err)
	}
	if !changed || existed {
		t.Errorf("Expected changed=true, existed=false, got changed=%v, existed=%v", changed, existed)
	}
	content, _ := os.ReadFile(mdFile)
	if !strings.Contains(string(content), "<!-- harnez:begin Test Section -->") {
		t.Errorf("Expected harnez marker, got: %s", string(content))
	}
	if !markdown.ContainsSection(mdFile, "Test Section") {
		t.Errorf("Expected ContainsSection to be true for modern marker")
	}

	// 2. Prepare legacy claudeconfig file
	legacyFile := filepath.Join(tmpDir, "LEGACY.md")
	legacyContent := "Preamble\n\n<!-- claudeconfig:begin Old Section -->\nOld content\n<!-- claudeconfig:end Old Section -->\n\nPostamble\n"
	if err := os.WriteFile(legacyFile, []byte(legacyContent), 0644); err != nil {
		t.Fatalf("Failed to write legacy file: %v", err)
	}

	if !markdown.ContainsSection(legacyFile, "Old Section") {
		t.Errorf("Expected ContainsSection to detect legacy claudeconfig section")
	}

	// 3. Apply update to legacy section: should upgrade markers to harnez
	changed, existed, err = markdown.Apply(legacyFile, "Old Section", "Upgraded content")
	if err != nil {
		t.Fatalf("Apply to legacy file failed: %v", err)
	}
	if !changed || !existed {
		t.Errorf("Expected changed=true, existed=true, got changed=%v, existed=%v", changed, existed)
	}
	upgradedContent, _ := os.ReadFile(legacyFile)
	if strings.Contains(string(upgradedContent), "claudeconfig:") {
		t.Errorf("Legacy marker should have been replaced, got: %s", string(upgradedContent))
	}
	if !strings.Contains(string(upgradedContent), "<!-- harnez:begin Old Section -->") {
		t.Errorf("Expected harnez begin marker, got: %s", string(upgradedContent))
	}
	if !strings.Contains(string(upgradedContent), "Upgraded content") {
		t.Errorf("Expected new content, got: %s", string(upgradedContent))
	}

	// 4. Clean legacy Makefile markers
	mkFile := filepath.Join(tmpDir, "Makefile")
	mkContent := "# header\n\n# claudeconfig:begin targets\nclean:\n\trm -f foo\n# claudeconfig:end targets\n"
	if err := os.WriteFile(mkFile, []byte(mkContent), 0644); err != nil {
		t.Fatalf("Failed to write Makefile: %v", err)
	}
	if !markdown.ContainsSectionMK(mkFile, "targets") {
		t.Errorf("Expected ContainsSectionMK to detect legacy marker")
	}
	removed, cleaned, err := markdown.CleanMK(mkFile, "targets")
	if err != nil {
		t.Fatalf("CleanMK failed: %v", err)
	}
	if removed || !cleaned {
		t.Errorf("Expected cleaned=true, removed=false, got cleaned=%v, removed=%v", cleaned, removed)
	}
	mkRemaining, _ := os.ReadFile(mkFile)
	if strings.Contains(string(mkRemaining), "claudeconfig") {
		t.Errorf("Makefile should not contain legacy targets after clean, got: %s", string(mkRemaining))
	}
}

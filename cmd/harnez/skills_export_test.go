package main

import (
	"bytes"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/claude"
)

func TestSitegenSkillsExportIsDeterministicAndComplete(t *testing.T) {
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	first, count, err := encodeSitegenSkillExport(cfg, strings.Repeat("a", 40))
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := encodeSitegenSkillExport(cfg, strings.Repeat("a", 40))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("export is not deterministic")
	}
	if count != len(cfg.Skills)+4 {
		t.Fatalf("count=%d, first-party=%d, bundled=4", count, len(cfg.Skills))
	}
	for _, want := range []string{"format_version: 1", "name: docup", "name: grilling", "kind: file-backed", "kind: inline", "kind: resource-backed", "license: AGPL-3.0-or-later", "license: MIT"} {
		if !bytes.Contains(first, []byte(want)) {
			t.Errorf("export missing %q", want)
		}
	}
}

func TestSitegenSkillsExportRejectsUnknownKindAndMissingLicense(t *testing.T) {
	cfg := &claude.Config{Skills: []claude.Command{{Name: "bad", Description: "bad", Content: "content"}}}
	if _, _, err := encodeSitegenSkillExport(cfg, strings.Repeat("a", 40)); err == nil || !strings.Contains(err.Error(), "license") {
		t.Fatalf("expected missing-license error, got %v", err)
	}
	if _, err := sitegenContentKind("unsupported"); err == nil {
		t.Fatal("expected unknown-kind rejection")
	}
}

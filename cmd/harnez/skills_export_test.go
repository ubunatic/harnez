package main

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"
	"testing/fstest"

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
	wantCount := len(cfg.Skills)
	for _, bundle := range cfg.BundledSkills {
		wantCount += len(bundle.Skills)
	}
	if count != wantCount {
		t.Fatalf("count=%d, registered=%d", count, wantCount)
	}
	for _, want := range []string{"format_version: 1", "name: docup", "kind: file-backed", "kind: inline", "kind: resource-backed", "license: AGPL-3.0-or-later"} {
		if !bytes.Contains(first, []byte(want)) {
			t.Errorf("export missing %q", want)
		}
	}
	for _, retired := range []string{"name: grilling\n", "name: grill-with-docs\n", "name: domain-modeling\n", "name: to-spec\n", "third_party/skills/mattpocock/"} {
		if bytes.Contains(first, []byte(retired)) {
			t.Errorf("export contains retired skill provenance %q", retired)
		}
	}
}

func TestSitegenSkillsExportPreservesConfiguredBundleProvenance(t *testing.T) {
	revision := strings.Repeat("a", 40)
	cfg := &claude.Config{
		BundledSkills: []claude.BundledSkillSource{{URL: "https://example.com/skills", Commit: revision, Dir: "vendor", Skills: []string{"example"}}},
		FS: fstest.MapFS{
			"vendor/LICENSE":          {Data: []byte("MIT License\n")},
			"vendor/example/SKILL.md": {Data: []byte("---\nname: example\ndescription: Example\n---\nExample body\n")},
		},
	}
	data, count, err := encodeSitegenSkillExport(cfg, revision)
	if err != nil || count != 1 {
		t.Fatalf("export fixture: count=%d, err=%v", count, err)
	}
	for _, want := range []string{"name: example", "origin: bundled", "source_revision: " + revision, "license: MIT", "source_path: vendor/example/SKILL.md", "Example body"} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("bundle export missing %q", want)
		}
	}
}

func TestSitegenSkillsExportRejectsModifiedOrRevisionlessBuild(t *testing.T) {
	revision := strings.Repeat("a", 40)
	for name, test := range map[string]struct {
		settings []debug.BuildSetting
		message  string
	}{
		"modified":                {settings: []debug.BuildSetting{{Key: "vcs.revision", Value: revision}, {Key: "vcs.modified", Value: "true"}}, message: "modified"},
		"revisionless":            {settings: []debug.BuildSetting{{Key: "vcs.modified", Value: "false"}}, message: "revision-less"},
		"missing modified marker": {settings: []debug.BuildSetting{{Key: "vcs.revision", Value: revision}}, message: "modified"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := sitegenRevisionFromSettings(test.settings); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatal("export revision accepted unsafe build metadata")
			}
		})
	}
	got, err := sitegenRevisionFromSettings([]debug.BuildSetting{{Key: "vcs.revision", Value: revision}, {Key: "vcs.modified", Value: "false"}})
	if err != nil || got != revision {
		t.Fatalf("clean revision = %q, %v", got, err)
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

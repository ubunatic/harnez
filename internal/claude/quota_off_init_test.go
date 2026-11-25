// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/claude"
)

// A repo without quota-1 rules must not get the Quota.md read item or quota-1
// test command, and re-running init must leave its AGENTS.md alone.
func TestRunInit_QuotaOffManagedTextHasNoQuotaCommands(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init", "-q").Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/quotaoff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := claude.LoadTestConfig(t)
	opts := claude.InitOptions{AssumeYes: true, Docs: []string{"gorelease"}}
	if err := claude.RunInitWithVariant(dir, cfg, opts); err != nil {
		t.Fatal(err)
	}
	agents := readFileForQuotaTest(t, filepath.Join(dir, "AGENTS.md"))
	if strings.Contains(agents, "Quota.md") {
		t.Errorf("AGENTS.md links Quota.md without quota-1 rules:\n%s", agents)
	}
	// Every managed rule and doc that init wrote must be free of test-q1.
	managed, err := filepath.Glob(filepath.Join(dir, "docs", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := filepath.Glob(filepath.Join(dir, ".harnez", "rules", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(managed) == 0 || len(rules) == 0 {
		t.Fatalf("init wrote no managed docs (%d) or rules (%d)", len(managed), len(rules))
	}
	for _, path := range append(managed, rules...) {
		body := readFileForQuotaTest(t, path)
		if strings.Contains(body, "test-q1") {
			t.Errorf("%s names test-q1 without quota-1 rules:\n%s", path, body)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".harnez", "rules", "Quota.md")); !os.IsNotExist(err) {
		t.Errorf("Quota.md written without quota-1 rules: %v", err)
	}
	makefile := readFileForQuotaTest(t, filepath.Join(dir, "Makefile"))
	if strings.Contains(makefile, "test-q1:") {
		t.Errorf("Makefile gained a test-q1 target without quota-1 rules:\n%s", makefile)
	}

	// A second run, as in an already-initialised repo, must not touch AGENTS.md.
	if err := claude.RunInitWithVariant(dir, cfg, claude.InitOptions{AssumeYes: true, Docs: []string{"gorelease"}}); err != nil {
		t.Fatal(err)
	}
	if again := readFileForQuotaTest(t, filepath.Join(dir, "AGENTS.md")); again != agents {
		t.Errorf("second init changed AGENTS.md:\nbefore:\n%s\nafter:\n%s", agents, again)
	}
}

// A repo.yaml without a rules key is valid and means quota-1 is off.
func TestParseRepoConfig_RulesOptional(t *testing.T) {
	doc := []byte(`version: 1
issues:
    tracker: harnez
    merge_driver: true
docs:
    studies: true
    copies: []
make:
    container_test: false
`)
	cfg, err := claude.ParseRepoConfig("repo.yaml", doc)
	if err != nil {
		t.Fatalf("repo.yaml without rules must validate: %v", err)
	}
	if cfg.Rules.Quota {
		t.Errorf("missing rules.quota must default to false")
	}
}

func readFileForQuotaTest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/assess"
)

func createDocHistoryFixture(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s failed: %v\nOutput: %s", strings.Join(args, " "), err, string(out))
		}
	}

	runGit("init")
	runGit("config", "user.name", "Test")
	runGit("config", "user.email", "test@example.com")

	if err := os.MkdirAll(filepath.Join(tmpDir, "docs", "lang"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "docs", "practices"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Commit 1: Add AGENTS.md and docs/lang/Go.md
	os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("# AGENTS\nInitial content\n"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "docs", "lang", "Go.md"), []byte("# Go Guidelines\nUse standard formatting.\n"), 0o644)
	runGit("add", ".")
	runGit("commit", "-m", "init docs")

	// Commit 2: Update docs/lang/Go.md and add docs/practices/Git.md
	os.WriteFile(filepath.Join(tmpDir, "docs", "lang", "Go.md"), []byte("# Go Guidelines\nUse standard formatting.\n\n## Concurrency\nPrefer channels and goroutines.\n"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "docs", "practices", "Git.md"), []byte("# Git Practices\nWrite clear commit messages.\n"), 0o644)
	runGit("add", ".")
	runGit("commit", "-m", "update go guidelines and add git practices")

	return tmpDir
}

func TestRunDocHistory_SingleFile(t *testing.T) {
	repoDir := createDocHistoryFixture(t)
	var out bytes.Buffer

	opts := docHistoryOptions{
		Dir:     repoDir,
		Color:   false,
		NoColor: true,
		JSON:    false,
		Targets: []string{"docs/lang/Go.md"},
	}

	if err := runDocHistory(&out, opts); err != nil {
		t.Fatalf("runDocHistory failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Document Token Evolution: docs/lang/Go.md") {
		t.Errorf("output missing header: %s", got)
	}
	if !strings.Contains(got, "Sparkline (Tokens):") {
		t.Errorf("output missing sparkline: %s", got)
	}
	if !strings.Contains(got, "init docs") || !strings.Contains(got, "update go guidelines") {
		t.Errorf("output missing commit subjects: %s", got)
	}
}

func TestRunDocHistory_SingleFile_JSON(t *testing.T) {
	repoDir := createDocHistoryFixture(t)
	var out bytes.Buffer

	opts := docHistoryOptions{
		Dir:     repoDir,
		JSON:    true,
		Targets: []string{"docs/lang/Go.md"},
	}

	if err := runDocHistory(&out, opts); err != nil {
		t.Fatalf("runDocHistory failed: %v", err)
	}

	var res assess.DocHistoryResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, out.String())
	}

	if res.FilePath != "docs/lang/Go.md" {
		t.Errorf("FilePath = %q; want docs/lang/Go.md", res.FilePath)
	}
	if res.TotalCommits != 2 {
		t.Errorf("TotalCommits = %d; want 2", res.TotalCommits)
	}
	if len(res.Snapshots) != 2 {
		t.Errorf("Snapshots len = %d; want 2", len(res.Snapshots))
	}
}

func TestRunDocHistory_MultiDoc_Default(t *testing.T) {
	repoDir := createDocHistoryFixture(t)
	var out bytes.Buffer

	opts := docHistoryOptions{
		Dir:     repoDir,
		Color:   false,
		NoColor: true,
		JSON:    false,
		Targets: nil,
	}

	if err := runDocHistory(&out, opts); err != nil {
		t.Fatalf("runDocHistory failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Multi-Doc Token Evolution & Stacked Category History") {
		t.Errorf("output missing multi-doc header: %s", got)
	}
	if !strings.Contains(got, "Category Breakdown Across Milestones") {
		t.Errorf("output missing category breakdown: %s", got)
	}
	if !strings.Contains(got, "Document Summary (Sorted by Current Token Footprint)") {
		t.Errorf("output missing document summary: %s", got)
	}
	if !strings.Contains(got, "docs/lang/Go.md") || !strings.Contains(got, "AGENTS.md") {
		t.Errorf("output missing expected docs: %s", got)
	}
}

func TestRunDocHistory_MultiDoc_DirAndGlobs(t *testing.T) {
	repoDir := createDocHistoryFixture(t)
	var out bytes.Buffer

	opts := docHistoryOptions{
		Dir:     repoDir,
		Color:   false,
		NoColor: true,
		JSON:    false,
		Targets: []string{"docs/lang", "docs/practices"},
	}

	if err := runDocHistory(&out, opts); err != nil {
		t.Fatalf("runDocHistory failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "docs/lang/Go.md") || !strings.Contains(got, "docs/practices/Git.md") {
		t.Errorf("output missing expected docs: %s", got)
	}
	// AGENTS.md should not be included since we restricted targets
	if strings.Contains(got, "AGENTS.md") {
		t.Errorf("output should not contain AGENTS.md when target is docs/lang docs/practices")
	}
}

func TestRunDocHistory_MultiDoc_JSON(t *testing.T) {
	repoDir := createDocHistoryFixture(t)
	var out bytes.Buffer

	opts := docHistoryOptions{
		Dir:     repoDir,
		JSON:    true,
		Targets: []string{"docs"},
	}

	if err := runDocHistory(&out, opts); err != nil {
		t.Fatalf("runDocHistory failed: %v", err)
	}

	var res assess.MultiDocResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput: %s", err, out.String())
	}

	if len(res.ResolvedFiles) != 2 {
		t.Errorf("ResolvedFiles len = %d; want 2", len(res.ResolvedFiles))
	}
	if len(res.Timeline) == 0 {
		t.Errorf("Timeline is empty")
	}
	if len(res.DocSummaries) != 2 {
		t.Errorf("DocSummaries len = %d; want 2", len(res.DocSummaries))
	}
}

func TestRunDocHistory_ColorToggles(t *testing.T) {
	repoDir := createDocHistoryFixture(t)

	// With color
	var outColor bytes.Buffer
	optsColor := docHistoryOptions{
		Dir:     repoDir,
		Color:   true,
		NoColor: false,
		Targets: []string{"docs"},
	}
	if err := runDocHistory(&outColor, optsColor); err != nil {
		t.Fatalf("runDocHistory with color failed: %v", err)
	}
	if !strings.Contains(outColor.String(), "\x1b[") {
		t.Errorf("expected ANSI color codes in colored output")
	}

	// With --no-color
	var outNoColor bytes.Buffer
	optsNoColor := docHistoryOptions{
		Dir:     repoDir,
		Color:   true,
		NoColor: true,
		Targets: []string{"docs"},
	}
	if err := runDocHistory(&outNoColor, optsNoColor); err != nil {
		t.Fatalf("runDocHistory with no-color failed: %v", err)
	}
	if strings.Contains(outNoColor.String(), "\x1b[") {
		t.Errorf("expected no ANSI color codes in no-color output")
	}
}

func TestDocHistoryCmd_CobraFlags(t *testing.T) {
	cmd := newDocHistoryCmd()

	if cmd.Use != "dochistory [files...]" {
		t.Errorf("cmd.Use = %q; want dochistory [files...]", cmd.Use)
	}

	hasAlias := false
	for _, a := range cmd.Aliases {
		if a == "doc-history" {
			hasAlias = true
			break
		}
	}
	if !hasAlias {
		t.Errorf("missing doc-history alias in %v", cmd.Aliases)
	}

	flags := []string{"dir", "color", "no-color", "json"}
	for _, f := range flags {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("missing flag --%s", f)
		}
	}
}

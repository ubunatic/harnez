package assess

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDiscoverDocFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a nested file hierarchy
	os.MkdirAll(filepath.Join(tmpDir, "docs", "lang"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "docs", "practices"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "docs", "empty"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)

	os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("agents"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "docs", "lang", "Go.md"), []byte("go"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "docs", "lang", "Rust.md"), []byte("rust"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "docs", "practices", "Sprint.md"), []byte("sprint"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "docs", "ignore.bin"), []byte{0x00, 0x01}, 0644)

	// Default targets: docs, AGENTS.md
	files, err := DiscoverDocFiles(tmpDir, nil)
	if err != nil {
		t.Fatalf("DiscoverDocFiles default failed: %v", err)
	}

	expected := map[string]bool{
		"AGENTS.md":                true,
		"docs/lang/Go.md":          true,
		"docs/lang/Rust.md":        true,
		"docs/practices/Sprint.md": true,
	}

	if len(files) != len(expected) {
		t.Fatalf("got %d files (%v); want %d", len(files), files, len(expected))
	}
	for _, f := range files {
		if !expected[f] {
			t.Errorf("unexpected discovered file: %s", f)
		}
	}

	// Glob pattern
	globFiles, err := DiscoverDocFiles(tmpDir, []string{"docs/lang/*.md"})
	if err != nil {
		t.Fatalf("DiscoverDocFiles glob failed: %v", err)
	}
	if len(globFiles) != 2 {
		t.Errorf("got %d glob files (%v); want 2", len(globFiles), globFiles)
	}
}

func TestBuildUnifiedTimelineAndActivePresence(t *testing.T) {
	t1 := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	t4 := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

	resolvedFiles := []string{"AGENTS.md", "docs/lang/Go.md", "docs/temp.md"}

	// doc 0 (AGENTS.md): present at t1 (100 tok), modified at t3 (150 tok)
	doc0 := DocHistoryResult{
		FilePath: "AGENTS.md",
		Snapshots: []DocSnapshot{
			{CommitTime: t1, CommitSHA: "c1", BlobSHA: "b1", EstTokens: 100, RawBytes: 400, Lines: 10, Words: 80},
			{CommitTime: t3, CommitSHA: "c3", BlobSHA: "b3", EstTokens: 150, RawBytes: 600, Lines: 15, Words: 120},
		},
	}

	// doc 1 (docs/lang/Go.md): created at t2 (200 tok), modified at t4 (250 tok)
	doc1 := DocHistoryResult{
		FilePath: "docs/lang/Go.md",
		Snapshots: []DocSnapshot{
			{CommitTime: t2, CommitSHA: "c2", BlobSHA: "b2", EstTokens: 200, RawBytes: 800, Lines: 20, Words: 160},
			{CommitTime: t4, CommitSHA: "c4", BlobSHA: "b4", EstTokens: 250, RawBytes: 1000, Lines: 25, Words: 200},
		},
	}

	// doc 2 (docs/temp.md): created at t1 (50 tok), deleted at t3
	doc2 := DocHistoryResult{
		FilePath: "docs/temp.md",
		Snapshots: []DocSnapshot{
			{CommitTime: t1, CommitSHA: "c1b", BlobSHA: "b5", EstTokens: 50, RawBytes: 200, Lines: 5, Words: 40},
			{CommitTime: t3, CommitSHA: "c3b", BlobSHA: "0000000000000000000000000000000000000000", Status: "D", EstTokens: 0},
		},
	}

	timeline, summaries := BuildUnifiedTimeline(resolvedFiles, []DocHistoryResult{doc0, doc1, doc2})

	// Expected timeline steps:
	// 1. t1 (c1): AGENTS.md (100 tok) -> Total: 100, Active: 1
	// 2. t1 (c1b): docs/temp.md (50 tok) -> Total: 150, Active: 2 (AGENTS 100, temp 50)
	// 3. t2 (c2): docs/lang/Go.md (200 tok) -> Total: 350, Active: 3 (AGENTS 100, Go 200, temp 50)
	// 4. t3 (c3): AGENTS.md (150 tok) -> Total: 400, Active: 3 (AGENTS 150, Go 200, temp 50)
	// 5. t3 (c3b): docs/temp.md deleted -> Total: 350, Active: 2 (AGENTS 150, Go 200)
	// 6. t4 (c4): docs/lang/Go.md (250 tok) -> Total: 400, Active: 2 (AGENTS 150, Go 250)

	if len(timeline) != 6 {
		t.Fatalf("got %d timeline snapshots; want 6", len(timeline))
	}

	// Verify step 1
	if timeline[0].TotalTokens != 100 || timeline[0].ActiveDocs != 1 {
		t.Errorf("step 0: TotalTokens = %d, ActiveDocs = %d; want 100, 1", timeline[0].TotalTokens, timeline[0].ActiveDocs)
	}

	// Verify step 2 (t1 with temp.md added)
	if timeline[1].TotalTokens != 150 || timeline[1].ActiveDocs != 2 {
		t.Errorf("step 1: TotalTokens = %d, ActiveDocs = %d; want 150, 2", timeline[1].TotalTokens, timeline[1].ActiveDocs)
	}

	// Verify step 3 (t2 with Go.md added)
	if timeline[2].TotalTokens != 350 || timeline[2].ActiveDocs != 3 {
		t.Errorf("step 2: TotalTokens = %d, ActiveDocs = %d; want 350, 3", timeline[2].TotalTokens, timeline[2].ActiveDocs)
	}

	// Verify step 5 (temp.md deleted at t3)
	if timeline[4].TotalTokens != 350 || timeline[4].ActiveDocs != 2 {
		t.Errorf("step 4: TotalTokens = %d, ActiveDocs = %d; want 350, 2", timeline[4].TotalTokens, timeline[4].ActiveDocs)
	}

	// Verify step 6 (Go.md updated at t4)
	if timeline[5].TotalTokens != 400 || timeline[5].ActiveDocs != 2 {
		t.Errorf("step 5: TotalTokens = %d, ActiveDocs = %d; want 400, 2", timeline[5].TotalTokens, timeline[5].ActiveDocs)
	}

	// Verify categories at step 6
	if timeline[5].ByCategory["lang"] != 250 {
		t.Errorf("step 5: ByCategory[lang] = %d; want 250", timeline[5].ByCategory["lang"])
	}
	if timeline[5].ByCategory["root"] != 150 {
		t.Errorf("step 5: ByCategory[root] = %d; want 150", timeline[5].ByCategory["root"])
	}

	// Verify doc summaries
	if len(summaries) != 3 {
		t.Fatalf("got %d summaries; want 3", len(summaries))
	}

	// Sorted by CurrentTokens descending: Go.md (250), AGENTS.md (150), docs/temp.md (0)
	if summaries[0].Path != "docs/lang/Go.md" || summaries[0].CurrentTokens != 250 {
		t.Errorf("summary[0] = %+v; want Go.md with 250 tokens", summaries[0])
	}
	if summaries[1].Path != "AGENTS.md" || summaries[1].CurrentTokens != 150 {
		t.Errorf("summary[1] = %+v; want AGENTS.md with 150 tokens", summaries[1])
	}
	if summaries[2].Path != "docs/temp.md" || summaries[2].CurrentTokens != 0 {
		t.Errorf("summary[2] = %+v; want docs/temp.md with 0 tokens", summaries[2])
	}

	// Percentage calculations (Total active = 400: Go 62.5%, AGENTS 37.5%, temp 0.0%)
	if summaries[0].PctOfTotal != 62.5 {
		t.Errorf("Go.md PctOfTotal = %f; want 62.5", summaries[0].PctOfTotal)
	}
	if summaries[1].PctOfTotal != 37.5 {
		t.Errorf("AGENTS.md PctOfTotal = %f; want 37.5", summaries[1].PctOfTotal)
	}
}

func TestRenderStackedBar(t *testing.T) {
	catTok := map[string]int{
		"lang":      600,
		"practices": 300,
		"studies":   100,
	}
	bar := RenderStackedBar(catTok, 1000, 20, false)
	runes := []rune(bar)
	if len(runes) != 20 {
		t.Fatalf("RenderStackedBar len = %d; want 20", len(runes))
	}

	// Check that lang (█), practices (▓), studies (▒) glyphs are present
	if !strings.Contains(bar, "█") || !strings.Contains(bar, "▓") || !strings.Contains(bar, "▒") {
		t.Errorf("RenderStackedBar missing expected glyphs: %s", bar)
	}

	colorBar := RenderStackedBar(catTok, 1000, 20, true)
	if !strings.Contains(colorBar, "\x1b[36m") {
		t.Errorf("RenderStackedBar(useColor=true) missing ANSI cyan code: %s", colorBar)
	}
}

func TestExtractMultiDocHistoryIntegration(t *testing.T) {
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

	os.MkdirAll(filepath.Join(tmpDir, "docs", "lang"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("# Agents\nSome configuration rules.\n"), 0644)
	runGit("add", "AGENTS.md")
	runGit("commit", "-m", "initial agents")

	os.WriteFile(filepath.Join(tmpDir, "docs", "lang", "Go.md"), []byte("# Go\nConventions for Go code.\n"), 0644)
	runGit("add", "docs/lang/Go.md")
	runGit("commit", "-m", "add go docs")

	res, err := ExtractMultiDocHistory(tmpDir, []string{"AGENTS.md", "docs"})
	if err != nil {
		t.Fatalf("ExtractMultiDocHistory error: %v", err)
	}

	if len(res.ResolvedFiles) != 2 {
		t.Errorf("ResolvedFiles len = %d; want 2", len(res.ResolvedFiles))
	}
	if res.TotalCommits != 2 {
		t.Errorf("TotalCommits = %d; want 2", res.TotalCommits)
	}
	if len(res.Timeline) != 2 {
		t.Fatalf("Timeline len = %d; want 2", len(res.Timeline))
	}

	rendered := RenderMultiDocHistory(res)
	if !strings.Contains(rendered, "Multi-Doc Token Evolution") {
		t.Errorf("RenderMultiDocHistory missing header: %s", rendered)
	}
	if !strings.Contains(rendered, "Category Breakdown Across Milestones") {
		t.Errorf("RenderMultiDocHistory missing category milestones: %s", rendered)
	}
	if !strings.Contains(rendered, "Document Summary") {
		t.Errorf("RenderMultiDocHistory missing document summary: %s", rendered)
	}
}

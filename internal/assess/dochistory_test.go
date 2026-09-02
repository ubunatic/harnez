package assess

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetricsCounters(t *testing.T) {
	doc := `# Title
Some introductory text with four words.

## Section 1
Here is a list of code:
` + "```go\nfmt.Println(\"hello world\")\n```\n" + `
Another heading:
### Sub-heading
Final words.
`
	words := CountWords([]byte(doc))
	if words < 15 {
		t.Errorf("CountWords() = %d; want >= 15", words)
	}

	headings := CountHeadings([]byte(doc))
	if headings != 3 {
		t.Errorf("CountHeadings() = %d; want 3", headings)
	}

	codeBlocks := CountCodeBlocks([]byte(doc))
	if codeBlocks != 1 {
		t.Errorf("CountCodeBlocks() = %d; want 1", codeBlocks)
	}

	metrics := ComputeDocMetrics([]byte(doc))
	if metrics.Headings != 3 || metrics.CodeBlocks != 1 || metrics.EstTokens == 0 {
		t.Errorf("ComputeDocMetrics() unexpected: %+v", metrics)
	}
}

func TestRenderSparkline(t *testing.T) {
	if got := RenderSparkline(nil, 10); got != "" {
		t.Errorf("RenderSparkline(nil) = %q; want empty string", got)
	}

	flat := []int{100, 100, 100}
	if got := RenderSparkline(flat, 10); len([]rune(got)) != 3 {
		t.Errorf("RenderSparkline(flat) len = %d; want 3", len([]rune(got)))
	}

	rising := []int{10, 20, 30, 40, 50, 60, 70, 80}
	spark := RenderSparkline(rising, 10)
	sparkRunes := []rune(spark)
	if len(sparkRunes) != 8 {
		t.Fatalf("RenderSparkline(rising) len = %d; want 8", len(sparkRunes))
	}
	if sparkRunes[0] != SparklineGlyphs[0] {
		t.Errorf("first rune = %c; want %c", sparkRunes[0], SparklineGlyphs[0])
	}
	if sparkRunes[7] != SparklineGlyphs[len(SparklineGlyphs)-1] {
		t.Errorf("last rune = %c; want %c", sparkRunes[7], SparklineGlyphs[len(SparklineGlyphs)-1])
	}

	// MaxWidth truncation
	truncated := RenderSparkline(rising, 4)
	if len([]rune(truncated)) != 4 {
		t.Errorf("RenderSparkline(rising, 4) len = %d; want 4", len([]rune(truncated)))
	}
}

func TestParseGitLogRaw(t *testing.T) {
	rawOutput := []byte(`commit 503b9de6844c548a6d5f9b352eb88382c3ec2838 1787993402 docs(lang): add release and version wiring conventions to Go.md

:100644 100644 2ffc0e48ff65c62b49e96fa3ea23989c97ee6518 a538f4253530f8231d59146a085dd6dfdf600b64 M	docs/lang/Go.md
commit fae8f802dc1b861b676d259ed363d6bec22c8add 1783176757 refactor(docs): restructure docs/ into lang/, other/, proposed/; rename --lang to --doc

:100644 100644 5cc6503ed9f96aeeb659aa01039df57a8b279b61 5cc6503ed9f96aeeb659aa01039df57a8b279b61 R100	docs/src/Go.md	docs/lang/Go.md
commit c6b92866ac60d0fd9a96d7c5263820ee0cdf2d2a 1781092814 upd + add docs, change notif

:000000 100644 0000000000000000000000000000000000000000 b3327ef08f4bf281d097f1a1a08675ac569b8476 A	docs/src/Go.md
`)

	entries, err := parseGitLogRaw(rawOutput)
	if err != nil {
		t.Fatalf("parseGitLogRaw error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("got %d entries; want 3", len(entries))
	}

	if entries[0].commitSHA != "503b9de6844c548a6d5f9b352eb88382c3ec2838" {
		t.Errorf("entry 0 sha = %s", entries[0].commitSHA)
	}
	if entries[1].status != "R100" || entries[1].oldPath != "docs/src/Go.md" || entries[1].newPath != "docs/lang/Go.md" {
		t.Errorf("entry 1 rename parse error: %+v", entries[1])
	}
	if entries[2].status != "A" || entries[2].newPath != "docs/src/Go.md" {
		t.Errorf("entry 2 add parse error: %+v", entries[2])
	}
}

func TestExtractDocHistoryTempRepo(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize git repo
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

	docFile := filepath.Join(tmpDir, "doc.md")
	os.WriteFile(docFile, []byte("# Initial Doc\nLine 1\n"), 0644)
	runGit("add", "doc.md")
	runGit("commit", "-m", "initial doc")

	os.WriteFile(docFile, []byte("# Initial Doc\nLine 1\nLine 2 with more tokens\n"), 0644)
	runGit("add", "doc.md")
	runGit("commit", "-m", "update doc")

	// Renamed doc
	runGit("mv", "doc.md", "guide.md")
	runGit("commit", "-m", "rename to guide")

	// Extract history of guide.md
	res, err := ExtractDocHistory(tmpDir, "guide.md")
	if err != nil {
		t.Fatalf("ExtractDocHistory error: %v", err)
	}

	if res.TotalCommits != 3 {
		t.Errorf("TotalCommits = %d; want 3", res.TotalCommits)
	}
	if len(res.Snapshots) != 3 {
		t.Fatalf("Snapshots len = %d; want 3", len(res.Snapshots))
	}

	// Rename should be tracked
	if res.Snapshots[0].CommitSubject != "initial doc" {
		t.Errorf("first commit subject = %q; want 'initial doc'", res.Snapshots[0].CommitSubject)
	}
	if res.Snapshots[2].CommitSubject != "rename to guide" {
		t.Errorf("last commit subject = %q; want 'rename to guide'", res.Snapshots[2].CommitSubject)
	}
	if res.CacheHits < 1 {
		t.Errorf("CacheHits = %d; want >= 1 (due to identical blob in rename)", res.CacheHits)
	}

	table := RenderDocHistoryTable(res)
	if !strings.Contains(table, "Document Token Evolution: guide.md") {
		t.Errorf("RenderDocHistoryTable missing header: %s", table)
	}
	if !strings.Contains(table, "Sparkline (Tokens):") {
		t.Errorf("RenderDocHistoryTable missing sparkline: %s", table)
	}
}

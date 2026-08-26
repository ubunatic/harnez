package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanDocsEligibilitySymlinkAndSkippedDirs(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	writeCaptureFile(t, source, "docs/Go.md", "same\n")
	writeCaptureFile(t, source, "CLAUDE.md", "agent\n")
	writeCaptureFile(t, root, "clean/AGENTS.md", "agent\n")
	if err := os.MkdirAll(filepath.Join(root, "symlink"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(source, "CLAUDE.md"), filepath.Join(root, "symlink", "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	writeCaptureFile(t, root, "git-only/.git/config", "git\n")
	writeCaptureFile(t, root, "unrelated/README.md", "readme\n")
	writeCaptureFile(t, root, "nested/AGENTS.md", "agent\n")
	writeCaptureFile(t, root, "nested/deeper/AGENTS.md", "agent\n")
	writeCaptureFile(t, root, "clean/docs/Go.md", "same\n")
	writeCaptureFile(t, root, "symlink/docs/Go.md", "same\n")

	report, err := ScanDocs(root, scanTestConfig(source))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report, "Scanned: 3 eligible, 2 skipped") {
		t.Fatalf("summary omitted eligibility result:\n%s", report)
	}
	if strings.Contains(report, "git-only") || strings.Contains(report, "deeper") {
		t.Fatalf("scan escaped immediate eligible children:\n%s", report)
	}
}

func TestScanDocsCleanSummaryAndMissingSuppression(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	writeCaptureFile(t, source, "docs/Go.md", "source\n")
	writeCaptureFile(t, root, "clean/AGENTS.md", "agent\n")
	writeCaptureFile(t, root, "missing/AGENTS.md", "agent\n")
	writeCaptureFile(t, root, "clean/docs/Go.md", "source\n")

	report, err := ScanDocs(root, scanTestConfig(source))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report, "Clean: clean, missing") {
		t.Fatalf("clean repos were not compactly summarized:\n%s", report)
	}
	if strings.Contains(report, "missing") && !strings.Contains(report, "Clean: clean, missing") {
		t.Fatalf("missing-doc detail leaked into summary:\n%s", report)
	}
}

func TestScanDocsSingleRepoDetailedReportSuppressesMissing(t *testing.T) {
	repo := t.TempDir()
	source := t.TempDir()
	writeCaptureFile(t, source, "docs/Go.md", "source\n")
	writeCaptureFile(t, repo, "AGENTS.md", "agent\n")
	writeCaptureFile(t, repo, "docs/Go.md", "local\n")

	report, err := ScanDocs(repo, scanTestConfig(source))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report, "## `docs/Go.md` (changed)") || !strings.Contains(report, "-source") {
		t.Fatalf("single-repo report lacks detailed drift:\n%s", report)
	}
	if strings.Contains(report, "(missing)") || strings.Contains(report, " missing,") {
		t.Fatalf("missing-doc detail leaked into single-repo report:\n%s", report)
	}
}

func TestScanDocsContainerRootWithAgentsMDScansChildren(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	writeCaptureFile(t, source, "docs/Go.md", "source\n")
	writeCaptureFile(t, root, "AGENTS.md", "container root agent doc\n")
	writeCaptureFile(t, root, "clean/AGENTS.md", "clean agent\n")
	writeCaptureFile(t, root, "clean/docs/Go.md", "source\n")
	writeCaptureFile(t, root, "drifted/AGENTS.md", "drifted agent\n")
	writeCaptureFile(t, root, "drifted/docs/Go.md", "local modified\n")
	writeCaptureFile(t, root, "unrelated/README.md", "readme\n")

	report, err := ScanDocs(root, scanTestConfig(source))
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(report, "---") || strings.Contains(report, "# Managed Docs Drift") {
		t.Fatalf("container root was treated as a single repo instead of multi-repo scan:\n%s", report)
	}
	if !strings.Contains(report, "Scanned: 2 eligible, 2 skipped") {
		t.Fatalf("expected 2 eligible and 2 skipped (AGENTS.md file + unrelated dir), got:\n%s", report)
	}
	if !strings.Contains(report, "Clean: clean") {
		t.Fatalf("clean repo summary missing:\n%s", report)
	}
	if !strings.Contains(report, "drifted  docs/Go.md (changed)") {
		t.Fatalf("drift repo summary missing or malformed:\n%s", report)
	}
}

func scanTestConfig(source string) *Config {
	return &Config{
		Dir:  source,
		FS:   os.DirFS(source),
		Docs: []string{"go"},
		AgentsMD: AgentsMD{Languages: map[string]Language{
			"go": {Source: "docs/Go.md", Local: "docs/Go.md"},
		}},
	}
}

package issues

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// againstRepo makes a repo whose main branch holds issues/1099-b.md and issues/1098-kept.md.
func againstRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runTestGit(t, dir, "init", "-q")
	runTestGit(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	issuesDir := filepath.Join(dir, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTicket(t, filepath.Join(issuesDir, "1099-b.md"), "1099", "B")
	writeTicket(t, filepath.Join(issuesDir, "1098-kept.md"), "1098", "Kept")
	runTestGit(t, dir, "add", "-A")
	runTestGit(t, dir, "commit", "-q", "-m", "docs(issues): base")
	return dir
}

func writeTicket(t *testing.T, path, number, title string) {
	t.Helper()
	body := "# " + number + " — " + title + "\n\n**Status**: Open\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLintAgainstNamesBothPathsOnCollision(t *testing.T) {
	dir := againstRepo(t)
	// This checkout adds a different ticket that takes 1099 on main.
	writeTicket(t, filepath.Join(dir, "issues", "1099-a.md"), "1099", "A")

	collisions, ok, err := CheckAgainst(dir, "main")
	if err != nil || !ok {
		t.Fatalf("CheckAgainst = %v, ok=%v, err=%v", collisions, ok, err)
	}
	if len(collisions) != 1 || collisions[0].Local != "issues/1099-a.md" || collisions[0].Remote != "issues/1099-b.md" {
		t.Fatalf("collisions = %+v, want issues/1099-a.md against issues/1099-b.md", collisions)
	}

	var out bytes.Buffer
	err = LintAgainst(&out, dir, "main")
	if err == nil {
		t.Fatal("LintAgainst returned nil on a confirmed collision")
	}
	for _, want := range []string{"issues/1099-a.md", "issues/1099-b.md", "main"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err.Error(), want)
		}
	}
}

func TestLintAgainstCleanBranchPasses(t *testing.T) {
	dir := againstRepo(t)
	// A new ticket with a free number, and a file that already exists on main under its own path.
	writeTicket(t, filepath.Join(dir, "issues", "1100-new.md"), "1100", "New")

	var out bytes.Buffer
	if err := LintAgainst(&out, dir, "main"); err != nil {
		t.Fatalf("LintAgainst on a clean branch: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("unexpected output: %q", out.String())
	}
}

func TestLintAgainstSamePathIsNotACollision(t *testing.T) {
	dir := againstRepo(t)
	// Editing the ticket that main already has is not a new claim on its number.
	writeTicket(t, filepath.Join(dir, "issues", "1099-b.md"), "1099", "B edited")

	if err := LintAgainst(&bytes.Buffer{}, dir, "main"); err != nil {
		t.Fatalf("LintAgainst on an edited existing ticket: %v", err)
	}
}

func TestLintAgainstMissingRefWarnsAndPasses(t *testing.T) {
	dir := againstRepo(t)
	writeTicket(t, filepath.Join(dir, "issues", "1099-a.md"), "1099", "A")

	var out bytes.Buffer
	if err := LintAgainst(&out, dir, "origin/main"); err != nil {
		t.Fatalf("LintAgainst with a missing ref must not fail: %v", err)
	}
	if !strings.Contains(out.String(), "origin/main not found") {
		t.Errorf("output = %q, want a warning that origin/main is not found", out.String())
	}
}

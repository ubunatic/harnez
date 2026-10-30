package issues

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.name=Test", "-c", "user.email=test@example.com"}, args...)
	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# ticket\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// remoteTicketRepo returns a clone whose fetched origin/main holds tickets that
// the clone's working tree does not. Only the first two files should count.
func remoteTicketRepo(t *testing.T) string {
	t.Helper()
	upstream := t.TempDir()
	runTestGit(t, upstream, "init", "-q", "-b", "main")
	writeTestFile(t, filepath.Join(upstream, "issues", "1009-elsewhere.md"))
	writeTestFile(t, filepath.Join(upstream, "issues", "archive", "0500-old.md"))
	writeTestFile(t, filepath.Join(upstream, "issues", "archive", "deep", "0600-nested.md"))
	writeTestFile(t, filepath.Join(upstream, "issues", "notes.md"))
	runTestGit(t, upstream, "add", "issues")
	runTestGit(t, upstream, "commit", "-q", "-m", "tickets")

	dir := t.TempDir()
	runTestGit(t, dir, "init", "-q", "-b", "main")
	writeTestFile(t, filepath.Join(dir, "README.md"))
	runTestGit(t, dir, "add", "README.md")
	runTestGit(t, dir, "commit", "-q", "-m", "initial")
	runTestGit(t, dir, "remote", "add", "origin", upstream)
	runTestGit(t, dir, "fetch", "-q", "origin")
	return dir
}

func TestRemoteTicketFiles(t *testing.T) {
	dir := remoteTicketRepo(t)
	files, err := RemoteTicketFiles(dir)
	if err != nil {
		t.Fatalf("RemoteTicketFiles: %v", err)
	}
	got := map[string]string{}
	for _, f := range files {
		got[f.Ref+":"+f.RelPath] = f.Number
	}
	want := map[string]string{
		"origin/main:1009-elsewhere.md":   "1009",
		"origin/main:archive/0500-old.md": "0500",
	}
	if len(got) != len(want) {
		t.Fatalf("RemoteTicketFiles = %v, want %v", got, want)
	}
	for k, num := range want {
		if got[k] != num {
			t.Errorf("RemoteTicketFiles[%q] = %q, want %q (all: %v)", k, got[k], num, got)
		}
	}

	notRepo, err := RemoteTicketFiles(t.TempDir())
	if err != nil || len(notRepo) != 0 {
		t.Fatalf("RemoteTicketFiles outside a repo = %v, %v; want no files and no error", notRepo, err)
	}
}

func TestReserve_SkipsNumbersOnFetchedRemoteRefs(t *testing.T) {
	repo := remoteTicketRepo(t)
	elsewhere, err := RemoteTicketFiles(repo)
	if err != nil {
		t.Fatalf("RemoteTicketFiles: %v", err)
	}

	baseline := filepath.Join(t.TempDir(), "issues")
	if num, _, err := Reserve(baseline, ReserveOptions{Number: "1009"}); err != nil || num != "1009" {
		t.Fatalf("Reserve without remote refs = %q, %v; want 1009 accepted", num, err)
	}

	issuesDir := filepath.Join(repo, "issues")
	num, file, err := Reserve(issuesDir, ReserveOptions{Elsewhere: elsewhere})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if num != "1010" || file != "1010-reserved.md" {
		t.Fatalf("Reserve = %q, %q; want 1010, 1010-reserved.md", num, file)
	}

	_, _, err = Reserve(issuesDir, ReserveOptions{Number: "1009", Elsewhere: elsewhere})
	if err == nil || !strings.Contains(err.Error(), "already exists: origin/main:issues/1009-elsewhere.md") {
		t.Fatalf("explicit taken number: got %v, want refusal naming origin/main:issues/1009-elsewhere.md", err)
	}
}

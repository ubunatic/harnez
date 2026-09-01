package gitstatus

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParse_CleanUpToDate(t *testing.T) {
	raw := "# branch.oid abc123\n" +
		"# branch.head main\n" +
		"# branch.upstream origin/main\n" +
		"# branch.ab +0 -0\n"
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Branch != "main" || s.Upstream != "origin/main" || s.Ahead != 0 || s.Behind != 0 {
		t.Fatalf("unexpected status: %+v", s)
	}
	if !s.Quiet() {
		t.Errorf("expected quiet, got verbose: %+v", s)
	}
}

func TestParse_AheadOnlyIsQuiet(t *testing.T) {
	raw := "# branch.head main\n" +
		"# branch.upstream origin/main\n" +
		"# branch.ab +3 -0\n"
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Ahead != 3 || s.Behind != 0 {
		t.Fatalf("unexpected ahead/behind: %+v", s)
	}
	if !s.Quiet() {
		t.Errorf("ahead-only with nothing else pending should be quiet, got: %+v", s)
	}
}

func TestParse_BehindIsVerbose(t *testing.T) {
	raw := "# branch.head main\n" +
		"# branch.upstream origin/main\n" +
		"# branch.ab +0 -2\n"
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Behind != 2 {
		t.Fatalf("expected behind=2, got %+v", s)
	}
	if s.Quiet() {
		t.Errorf("behind should be verbose, got quiet: %+v", s)
	}
}

func TestParse_DivergedIsVerbose(t *testing.T) {
	raw := "# branch.head main\n" +
		"# branch.upstream origin/main\n" +
		"# branch.ab +1 -1\n"
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !s.Diverged() {
		t.Errorf("expected diverged")
	}
	if s.Quiet() {
		t.Errorf("diverged should be verbose")
	}
}

func TestParse_DetachedHeadIsVerbose(t *testing.T) {
	raw := "# branch.head (detached)\n"
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !s.Detached {
		t.Errorf("expected detached")
	}
	if s.Quiet() {
		t.Errorf("detached HEAD should be verbose")
	}
}

func TestParse_StagedUnstagedUntracked(t *testing.T) {
	raw := "# branch.head main\n" +
		"1 M. N... 100644 100644 100644 aaaa bbbb staged.go\n" +
		"1 .M N... 100644 100644 100644 aaaa bbbb unstaged.go\n" +
		"1 MM N... 100644 100644 100644 aaaa bbbb both.go\n" +
		"? untracked.txt\n"
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !reflect.DeepEqual(s.StagedFiles, []string{"staged.go", "both.go"}) {
		t.Errorf("StagedFiles = %v", s.StagedFiles)
	}
	if !reflect.DeepEqual(s.UnstagedFiles, []string{"unstaged.go", "both.go"}) {
		t.Errorf("UnstagedFiles = %v", s.UnstagedFiles)
	}
	if !reflect.DeepEqual(s.UntrackedFiles, []string{"untracked.txt"}) {
		t.Errorf("UntrackedFiles = %v", s.UntrackedFiles)
	}
	if s.Quiet() {
		t.Errorf("dirty tree should be verbose")
	}
}

func TestParse_RenamedEntry(t *testing.T) {
	raw := "# branch.head main\n" +
		"2 R. N... 100644 100644 100644 aaaa bbbb R100 new.go\told.go\n"
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !reflect.DeepEqual(s.StagedFiles, []string{"new.go"}) {
		t.Errorf("StagedFiles = %v", s.StagedFiles)
	}
}

func TestParse_ConflictEntry(t *testing.T) {
	raw := "# branch.head main\n" +
		"u UU N... 100644 100644 100644 100644 aaaa bbbb cccc conflict.go\n"
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !reflect.DeepEqual(s.ConflictFiles, []string{"conflict.go"}) {
		t.Errorf("ConflictFiles = %v", s.ConflictFiles)
	}
	if s.Quiet() {
		t.Errorf("conflict should be verbose")
	}
}

// --- real-repo tests -------------------------------------------------

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	return dir
}

func TestCollect_CleanRealRepo(t *testing.T) {
	dir := initRepo(t)
	if err := writeAndCommit(t, dir, "README.md", "hello\n", "initial commit"); err != nil {
		t.Fatal(err)
	}

	s, err := Collect(dir)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if s.Branch != "main" {
		t.Errorf("Branch = %q, want main", s.Branch)
	}
	if !s.Quiet() {
		t.Errorf("expected quiet for freshly-committed repo with no upstream, got: %+v", s)
	}
}

func TestCollect_DirtyRealRepo(t *testing.T) {
	dir := initRepo(t)
	if err := writeAndCommit(t, dir, "README.md", "hello\n", "initial commit"); err != nil {
		t.Fatal(err)
	}

	// unstaged modification
	if err := writeFile(dir, "README.md", "hello again\n"); err != nil {
		t.Fatal(err)
	}
	// untracked file
	if err := writeFile(dir, "new.txt", "new\n"); err != nil {
		t.Fatal(err)
	}
	// staged addition
	if err := writeFile(dir, "staged.txt", "staged\n"); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "staged.txt")

	s, err := Collect(dir)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if s.Quiet() {
		t.Errorf("expected verbose for dirty repo, got quiet: %+v", s)
	}
	if !contains(s.UnstagedFiles, "README.md") {
		t.Errorf("UnstagedFiles = %v, want README.md present", s.UnstagedFiles)
	}
	if !contains(s.UntrackedFiles, "new.txt") {
		t.Errorf("UntrackedFiles = %v, want new.txt present", s.UntrackedFiles)
	}
	if !contains(s.StagedFiles, "staged.txt") {
		t.Errorf("StagedFiles = %v, want staged.txt present", s.StagedFiles)
	}
}

func TestCollect_AheadOfUpstreamOnlyIsQuiet(t *testing.T) {
	remote := t.TempDir()
	runGit(t, remote, "init", "-q", "--bare", "-b", "main")

	dir := initRepo(t)
	if err := writeAndCommit(t, dir, "README.md", "hello\n", "initial commit"); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "remote", "add", "origin", remote)
	runGit(t, dir, "push", "-q", "-u", "origin", "main")

	if err := writeAndCommit(t, dir, "file2.txt", "more\n", "second commit"); err != nil {
		t.Fatal(err)
	}

	s, err := Collect(dir)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if s.Ahead != 1 || s.Behind != 0 {
		t.Fatalf("expected ahead=1 behind=0, got: %+v", s)
	}
	if !s.Quiet() {
		t.Errorf("ahead-of-origin-only should be quiet, got verbose: %+v", s)
	}
}

func TestCollect_BehindUpstreamIsVerbose(t *testing.T) {
	remote := t.TempDir()
	runGit(t, remote, "init", "-q", "--bare", "-b", "main")

	dir := initRepo(t)
	if err := writeAndCommit(t, dir, "README.md", "hello\n", "initial commit"); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "remote", "add", "origin", remote)
	runGit(t, dir, "push", "-q", "-u", "origin", "main")

	// Clone a second checkout, commit there, push -- so `dir` is now behind.
	dir2 := t.TempDir()
	runGit(t, filepath.Dir(dir2), "clone", "-q", remote, dir2)
	runGit(t, dir2, "config", "user.email", "test@example.com")
	runGit(t, dir2, "config", "user.name", "Test")
	if err := writeAndCommit(t, dir2, "file2.txt", "more\n", "second commit"); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir2, "push", "-q")

	runGit(t, dir, "fetch", "-q")

	s, err := Collect(dir)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if s.Behind != 1 {
		t.Fatalf("expected behind=1, got: %+v", s)
	}
	if s.Quiet() {
		t.Errorf("behind-upstream should be verbose, got quiet: %+v", s)
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func writeFile(dir, name, content string) error {
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

func writeAndCommit(t *testing.T, dir, name, content, message string) error {
	t.Helper()
	if err := writeFile(dir, name, content); err != nil {
		return err
	}
	runGit(t, dir, "add", name)
	runGit(t, dir, "commit", "-q", "-m", message)
	return nil
}

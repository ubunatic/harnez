package maintenance

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func rcGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=a@b", "-c", "user.name=t"}, args...)...)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func rcWrite(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func rcTicket(num, title, extra string) string {
	return "# " + num + " — " + title + "\n\n**Status**: Open\n**Priority**: P2 (Medium)\n**Severity**: Minor\n**Category**: Feature\n\n---\n\nbody\n" + extra
}

func rcCommit(t *testing.T, dir, msg string) {
	rcGit(t, dir, "add", "-A")
	rcGit(t, dir, "commit", "-qm", msg)
}

// rcMerged builds main with theirs (002, plus extra numbered tickets) and feat with
// ours (002 self-referencing, 003 referencing 002), then merges main into feat.
func rcMerged(t *testing.T, theirs ...string) string {
	t.Helper()
	dir := t.TempDir()
	rcGit(t, dir, "init", "-q", "-b", "main")
	rcWrite(t, dir, "issues/001-base.md", rcTicket("001", "base", ""))
	rcCommit(t, dir, "base")
	rcGit(t, dir, "checkout", "-qb", "feat")
	rcWrite(t, dir, "issues/002-ours.md", rcTicket("002", "ours", "self 002-ours.md\n"))
	rcWrite(t, dir, "issues/003-other.md", rcTicket("003", "other", "see 002-ours.md\n"))
	rcCommit(t, dir, "ours")
	rcGit(t, dir, "checkout", "-q", "main")
	rcWrite(t, dir, "issues/002-theirs.md", rcTicket("002", "theirs", ""))
	for _, n := range theirs {
		rcWrite(t, dir, "issues/"+n+"-t.md", rcTicket(n, "t"+n, ""))
	}
	rcCommit(t, dir, "theirs")
	rcGit(t, dir, "checkout", "-q", "feat")
	rcGit(t, dir, "merge", "-q", "--no-edit", "main")
	return dir
}

func rcExists(dir, rel string) bool {
	_, err := os.Stat(filepath.Join(dir, rel))
	return err == nil
}

func rcRead(t *testing.T, dir, rel string) string {
	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestReconcileDryRunPlansWithoutChanges(t *testing.T) {
	dir := rcMerged(t)
	before := rcGit(t, dir, "rev-parse", "HEAD")
	var out strings.Builder
	if err := RunIssuesReconcile(&out, dir, "", true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "issues/002-ours.md\t002 -> 004") || !strings.Contains(out.String(), "rewrite references in issues/003-other.md") {
		t.Fatalf("plan: %s", out.String())
	}
	if !rcExists(dir, "issues/002-ours.md") || rcGit(t, dir, "rev-parse", "HEAD") != before {
		t.Fatal("dry-run changed state")
	}
}

func TestReconcileRenumbersOwnedCollisionInOneCommit(t *testing.T) {
	dir := rcMerged(t)
	before := rcGit(t, dir, "rev-parse", "HEAD")
	var out strings.Builder
	if err := RunIssuesReconcile(&out, dir, "", false); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if rcExists(dir, "issues/002-ours.md") || !rcExists(dir, "issues/004-ours.md") || !rcExists(dir, "issues/002-theirs.md") {
		t.Fatal("wrong files after renumber")
	}
	got := rcRead(t, dir, "issues/004-ours.md")
	if !strings.HasPrefix(got, "# 004 — ours") || !strings.Contains(got, "self 004-ours.md") {
		t.Fatalf("header/self reference: %s", got)
	}
	if !strings.Contains(rcRead(t, dir, "issues/003-other.md"), "see 004-ours.md") {
		t.Fatal("reference not rewritten")
	}
	if !strings.Contains(rcRead(t, dir, "issues/README.md"), "004-ours.md") {
		t.Fatal("README not regenerated")
	}
	if n := rcGit(t, dir, "rev-list", "--count", before+"..HEAD"); n != "1" {
		t.Fatalf("commits = %s, want 1", n)
	}
	if m := rcGit(t, dir, "log", "-1", "--format=%s"); m != "docs(issues): reconcile ticket numbers after merge" {
		t.Fatalf("message %q", m)
	}
}

func TestReconcileNoCollisions(t *testing.T) {
	dir := t.TempDir()
	rcGit(t, dir, "init", "-q", "-b", "main")
	rcWrite(t, dir, "issues/001-a.md", rcTicket("001", "a", ""))
	rcCommit(t, dir, "a")
	rcGit(t, dir, "branch", "base")
	var out strings.Builder
	if err := RunIssuesReconcile(&out, dir, "base", false); err != nil || !strings.Contains(out.String(), "no collisions") {
		t.Fatalf("%v %s", err, out.String())
	}
}

func TestReconcileMultipleCollisionsUseNextFreeNumbers(t *testing.T) {
	dir := rcMerged(t, "004")
	// a second owned ticket colliding with upstream's 004
	rcWrite(t, dir, "issues/004-ours2.md", rcTicket("004", "ours2", ""))
	_ = os.Remove(filepath.Join(dir, "issues/004-t.md"))
	rcGit(t, dir, "checkout", "-q", "main", "--", "issues/004-t.md")
	rcCommit(t, dir, "second")
	var out strings.Builder
	if err := RunIssuesReconcile(&out, dir, "main", false); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if !rcExists(dir, "issues/005-ours.md") || !rcExists(dir, "issues/006-ours2.md") {
		t.Fatalf("want 005 and 006:\n%s", out.String())
	}
}

func TestReconcileErrors(t *testing.T) {
	dir := rcMerged(t)
	if err := RunIssuesReconcile(&strings.Builder{}, dir, "no-such-ref", false); err == nil {
		t.Fatal("bad ref must fail")
	}
	rcWrite(t, dir, "issues/003-other.md", "dirty\n")
	if err := RunIssuesReconcile(&strings.Builder{}, dir, "", false); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("dirty tracker: %v", err)
	}
}

func TestReconcileSkipsDeletedOwnedTicket(t *testing.T) {
	dir := rcMerged(t)
	rcGit(t, dir, "rm", "-q", "issues/002-ours.md")
	rcWrite(t, dir, "issues/003-other.md", rcTicket("003", "other", ""))
	rcCommit(t, dir, "drop ours")
	var out strings.Builder
	if err := RunIssuesReconcile(&out, dir, "main", false); err != nil || !strings.Contains(out.String(), "no collisions") {
		t.Fatalf("%v %s", err, out.String())
	}
}

func TestReconcilePullMergesUpstreamAndRenumbers(t *testing.T) {
	src := rcMerged(t)
	_ = src
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	rcGit(t, root, "init", "-q", "--bare", "-b", "main", remote)
	work := filepath.Join(root, "work")
	rcGit(t, root, "clone", "-q", remote, work)
	rcWrite(t, work, "issues/001-base.md", rcTicket("001", "base", ""))
	rcCommit(t, work, "base")
	rcGit(t, work, "push", "-q", "origin", "HEAD:main")
	rcGit(t, work, "branch", "-q", "--set-upstream-to=origin/main")
	rcWrite(t, work, "issues/002-ours.md", rcTicket("002", "ours", ""))
	rcCommit(t, work, "ours")
	other := filepath.Join(root, "other")
	rcGit(t, root, "clone", "-q", remote, other)
	rcWrite(t, other, "issues/002-theirs.md", rcTicket("002", "theirs", ""))
	rcCommit(t, other, "theirs")
	rcGit(t, other, "push", "-q", "origin", "HEAD:main")

	var out strings.Builder
	if err := RunIssuesReconcileOpts(&out, work, ReconcileOptions{Fetch: true}); err != nil || !strings.Contains(out.String(), "002 -> 003") {
		t.Fatalf("fetch plan: %v %s", err, out.String())
	}
	if !rcExists(work, "issues/002-ours.md") || rcExists(work, "issues/002-theirs.md") {
		t.Fatal("--fetch must not merge or renumber")
	}
	out.Reset()
	if err := RunIssuesReconcileOpts(&out, work, ReconcileOptions{Pull: true}); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if !rcExists(work, "issues/003-ours.md") || !rcExists(work, "issues/002-theirs.md") || rcExists(work, "issues/002-ours.md") {
		t.Fatalf("after pull:\n%s", out.String())
	}
	if err := RunIssuesReconcileOpts(&out, work, ReconcileOptions{Pull: true, Ref: "x"}); err == nil {
		t.Fatal("ref with --pull must fail")
	}
}

func TestTrackerDirtyDetectsModifiedFirstLine(t *testing.T) {
	dir := t.TempDir()
	rcGit(t, dir, "init", "-q", "-b", "main")
	rcWrite(t, dir, "issues/001-a.md", rcTicket("001", "a", ""))
	rcCommit(t, dir, "a")
	rcWrite(t, dir, "issues/001-a.md", "changed\n")
	status, err := porcelainStatus(dir)
	if err != nil || !trackerDirty(status) {
		t.Fatalf("modified tracked ticket not detected: %q %v", status, err)
	}
}

func TestReconcileRollsBackWhenClaimFails(t *testing.T) {
	dir := rcMerged(t)
	// An (untracked, empty) directory squats the destination so the O_EXCL claim fails.
	if err := os.Mkdir(filepath.Join(dir, "issues", "004-ours.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	head := rcGit(t, dir, "rev-parse", "HEAD")
	err := RunIssuesReconcile(&strings.Builder{}, dir, "main", false)
	if err == nil {
		t.Fatal("expected failure")
	}
	if rcGit(t, dir, "rev-parse", "HEAD") != head || !rcExists(dir, "issues/002-ours.md") {
		t.Fatalf("not rolled back: %v", err)
	}
	if !strings.Contains(rcRead(t, dir, "issues/003-other.md"), "see 002-ours.md") {
		t.Fatal("references modified despite failure")
	}
	if s := rcGit(t, dir, "status", "--porcelain", "--untracked-files=no"); s != "" {
		t.Fatalf("dirty after rollback: %s", s)
	}
}

func rcRemoteWithConflict(t *testing.T) (work string) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	rcGit(t, root, "init", "-q", "--bare", "-b", "main", remote)
	work = filepath.Join(root, "work")
	rcGit(t, root, "clone", "-q", remote, work)
	rcWrite(t, work, "a.txt", "base\n")
	rcCommit(t, work, "base")
	rcGit(t, work, "push", "-q", "origin", "HEAD:main")
	rcGit(t, work, "branch", "-q", "--set-upstream-to=origin/main")
	rcWrite(t, work, "a.txt", "ours\n")
	rcCommit(t, work, "ours")
	other := filepath.Join(root, "other")
	rcGit(t, root, "clone", "-q", remote, other)
	rcWrite(t, other, "a.txt", "theirs\n")
	rcCommit(t, other, "theirs")
	rcGit(t, other, "push", "-q", "origin", "HEAD:main")
	return work
}

func TestReconcilePullStopsOnConflict(t *testing.T) {
	work := rcRemoteWithConflict(t)
	err := RunIssuesReconcileOpts(&strings.Builder{}, work, ReconcileOptions{Pull: true})
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("want conflict error, got %v", err)
	}
	if _, e := exec.Command("git", "-C", work, "rev-parse", "-q", "--verify", "MERGE_HEAD").Output(); e != nil {
		t.Fatal("merge should be left in progress for the user")
	}
}

func TestReconcilePullRefusesDirtyTrackedTree(t *testing.T) {
	work := rcRemoteWithConflict(t)
	rcWrite(t, work, "a.txt", "dirty\n")
	err := RunIssuesReconcileOpts(&strings.Builder{}, work, ReconcileOptions{Pull: true})
	if err == nil || !strings.Contains(err.Error(), "tracked changes") {
		t.Fatalf("got %v", err)
	}
}

func TestReconcileFetchAndPullExclusive(t *testing.T) {
	if err := RunIssuesReconcileOpts(&strings.Builder{}, t.TempDir(), ReconcileOptions{Fetch: true, Pull: true}); err == nil {
		t.Fatal("expected error")
	}
}

func TestReconcilePullAfterOldMergeAlreadyUpToDate(t *testing.T) {
	dir := rcMerged(t) // HEAD is a merge commit with a collision-free upstream afterwards
	_ = RunIssuesReconcile(&strings.Builder{}, dir, "", false)
	rcGit(t, dir, "remote", "add", "origin", dir)
	rcGit(t, dir, "fetch", "-q", "origin")
	rcGit(t, dir, "branch", "-q", "--set-upstream-to=origin/main")
	var out strings.Builder
	if err := RunIssuesReconcileOpts(&out, dir, ReconcileOptions{Pull: true}); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if !rcExists(dir, "issues/004-ours.md") {
		t.Fatal("renumbered ticket must stay")
	}
}

package prwait

import (
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// openJSON is an open PR in each forge's real shape. mergeable is "true", "false" or "null".
// Forgejo marks AGit PRs with flow 1 and labels the head "<pusher>/<topic>"; GitHub has no
// flow and labels the head "<owner>:<branch>".
func openJSON(kind, mergeable, base string) string {
	return openJSONHead(kind, mergeable, base, "aaa111")
}

func openJSONHead(kind, mergeable, base, head string) string {
	if kind == "github" {
		return fmt.Sprintf(`{"number":78,"state":"open","merged":false,"mergeable":%s,`+
			`"head":{"ref":"issue-78","label":"ubunatic:issue-78","sha":%q,"repo":{"full_name":"ubunatic/harnez"}},`+
			`"base":{"ref":%q,"label":%q}}`, mergeable, head, base, base)
	}
	return fmt.Sprintf(`{"number":78,"state":"open","merged":false,"mergeable":%s,"flow":1,`+
		`"head":{"ref":"refs/pull/78/head","label":"flashcoder/issue-78","sha":%q,"repo":{"full_name":"ubunatic/harnez"}},`+
		`"base":{"ref":%q,"label":%q}}`, mergeable, head, base, base)
}

func runConflictScenario(t *testing.T, kind string, steps []step, git *fakeGit) (Result, *scripted, Options) {
	t.Helper()
	return runFixupScenario(t, kind, steps, git, nil, nil)
}

func runFixupScenario(t *testing.T, kind string, steps []step, git *fakeGit, fix Fixer, progress *strings.Builder) (Result, *scripted, Options) {
	t.Helper()
	s := &scripted{steps: steps}
	srv := httptest.NewServer(s.handler(t))
	t.Cleanup(srv.Close)
	opt := Options{
		Target: Target{Forge: forgeFor(kind, srv.URL), Owner: "ubunatic", Repo: "harnez", Number: 78},
		Poll:   30 * time.Second, Timeout: time.Hour, MaxErrors: 3, Remote: "origin", Branch: "main",
	}
	if progress != nil {
		opt.Progress = progress
	}
	w := newWaiter(NewHTTPFetcher(5*time.Second, nil), &fakeClock{now: time.Unix(0, 0)}, git)
	w.Fix = fix
	return w.Wait(opt), s, opt
}

type fakeFixer struct {
	head string
	err  error
	reqs []FixupRequest
}

func (f *fakeFixer) Fixup(req FixupRequest) (FixupResult, error) {
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return FixupResult{}, f.err
	}
	return FixupResult{Head: f.head, Dest: "refs/for/main/issue-78", Regenerated: req.Files}, nil
}

func TestWaitFixupScenarios(t *testing.T) {
	readme := []string{"issues/README.md"}
	gitDown := conflictAnswer{err: fmt.Errorf("git merge-tree: exit 129")}
	cases := []struct {
		name    string
		polls   [][2]string // mergeable, head per poll; "merged" ends with a merged PR
		git     []conflictAnswer
		fixErr  error
		want    Outcome
		fixes   int
		checks  int
		errHas  string
		pushed  bool
		fetches int
	}{
		{name: "fixup then merge", polls: [][2]string{{"false", "aaa111"}, {"true", "fff999"}, {"merged", ""}},
			git: []conflictAnswer{{files: readme}}, want: Merged, fixes: 1, checks: 1, pushed: true, fetches: 3},
		{name: "fixup fails", polls: [][2]string{{"false", "aaa111"}},
			git: []conflictAnswer{{files: readme}}, fixErr: fmt.Errorf("push rejected"),
			want: Conflicted, fixes: 1, checks: 1, errHas: "fixup failed: push rejected", fetches: 1},
		{name: "conflicts again after the fixup", polls: [][2]string{{"false", "aaa111"}, {"false", "fff999"}},
			git: []conflictAnswer{{files: readme}}, want: Conflicted, fixes: 1, checks: 2,
			errHas: "still conflicting after the fixup pushed fff999", pushed: true, fetches: 2},
		{name: "forge shows the old head for one poll", polls: [][2]string{{"false", "aaa111"}, {"false", "aaa111"}, {"null", "fff999"}, {"merged", ""}},
			git: []conflictAnswer{{files: readme}}, want: Merged, fixes: 1, checks: 1, pushed: true, fetches: 4},
		{name: "head never becomes the fixup", polls: [][2]string{{"false", "aaa111"}, {"false", "aaa111"}, {"false", "aaa111"}},
			git: []conflictAnswer{{files: readme}}, want: Conflicted, fixes: 1, checks: 1,
			errHas: "still shows head aaa111", pushed: true, fetches: 3},
		{name: "a later push by someone else is fine", polls: [][2]string{{"false", "aaa111"}, {"true", "fff999"}, {"true", "bbb222"}, {"merged", ""}},
			git: []conflictAnswer{{files: readme}}, want: Merged, fixes: 1, checks: 1, pushed: true, fetches: 4},
		{name: "an unconfirmed conflict is not fixed", polls: [][2]string{{"false", "aaa111"}, {"false", "aaa111"}},
			git: []conflictAnswer{gitDown}, want: Conflicted, fixes: 0, checks: 2, errHas: "local merge check failed", fetches: 2},
	}
	for _, kind := range forgeKinds {
		for _, c := range cases {
			t.Run(kind+"/"+c.name, func(t *testing.T) {
				var steps []step
				for _, p := range c.polls {
					if p[0] == "merged" {
						steps = append(steps, step{200, prJSON("closed", true)})
						continue
					}
					steps = append(steps, step{200, openJSONHead(kind, p[0], "main", p[1])})
				}
				git := &fakeGit{conflicts: c.git}
				fix := &fakeFixer{head: "fff999", err: c.fixErr}
				var progress strings.Builder
				res, s, _ := runFixupScenario(t, kind, steps, git, fix, &progress)
				if res.Outcome != c.want {
					t.Fatalf("outcome = %d, want %d (err=%v)", res.Outcome, c.want, res.Err)
				}
				if s.calls != c.fetches {
					t.Fatalf("fetches = %d, want %d", s.calls, c.fetches)
				}
				if len(fix.reqs) != c.fixes {
					t.Fatalf("fixups = %d, want %d", len(fix.reqs), c.fixes)
				}
				if len(git.conflictAsk) != c.checks {
					t.Fatalf("local merge checks = %d, want %d", len(git.conflictAsk), c.checks)
				}
				if c.fixes > 0 {
					r := fix.reqs[0]
					if r.Base != "main" || r.Remote != "origin" || r.Info.Head.SHA != "aaa111" || strings.Join(r.Files, ",") != "issues/README.md" {
						t.Fatalf("fixup request = %+v, want the conflicting head, base and files", r)
					}
				}
				if c.errHas == "" && res.Err != nil {
					t.Fatalf("err = %v, want none", res.Err)
				}
				if c.errHas != "" && (res.Err == nil || !strings.Contains(res.Err.Error(), c.errHas)) {
					t.Fatalf("err = %v, want it to contain %q", res.Err, c.errHas)
				}
				line := progress.String()
				if c.pushed != (line != "") {
					t.Fatalf("progress = %q, want a fixup line %t", line, c.pushed)
				}
				if c.pushed {
					for _, want := range []string{"PR 78 fixup", "merged origin/main", "regenerated issues/README.md", "pushed fff999 to refs/for/main/issue-78", "waiting again"} {
						if !strings.Contains(line, want) {
							t.Errorf("progress %q lacks %q", line, want)
						}
					}
				}
			})
		}
	}
}

func TestWaitConflictScenarios(t *testing.T) {
	readme := []string{"issues/README.md"}
	gitDown := conflictAnswer{err: fmt.Errorf("git merge-tree: exit 129: unknown option")}
	merged := step{200, prJSON("closed", true)}
	cases := []struct {
		name      string
		mergeable []string // per poll; a merged step follows when wantMerge
		git       []conflictAnswer
		want      Outcome
		fetches   int
		checks    int
		wantFiles bool
		wantErr   bool
	}{
		{name: "conflicted at once", mergeable: []string{"false"},
			git: []conflictAnswer{{files: readme}}, want: Conflicted, fetches: 1, checks: 1, wantFiles: true},
		{name: "null keeps polling", mergeable: []string{"null", "null"},
			want: Merged, fetches: 3, checks: 0},
		{name: "conflict appears mid-wait", mergeable: []string{"true", "false"},
			git: []conflictAnswer{{files: readme}}, want: Conflicted, fetches: 2, checks: 1, wantFiles: true},
		{name: "unmergeable but locally clean keeps polling", mergeable: []string{"false", "false"},
			git: []conflictAnswer{{}}, want: Merged, fetches: 3, checks: 2},
		{name: "unconfirmed conflict exits on the second poll", mergeable: []string{"false", "false"},
			git: []conflictAnswer{gitDown}, want: Conflicted, fetches: 2, checks: 2, wantErr: true},
		{name: "unconfirmed streak resets on a mergeable poll", mergeable: []string{"false", "true", "false"},
			git: []conflictAnswer{gitDown}, want: Merged, fetches: 4, checks: 2},
	}
	for _, kind := range forgeKinds {
		for _, c := range cases {
			t.Run(kind+"/"+c.name, func(t *testing.T) {
				var steps []step
				for _, m := range c.mergeable {
					steps = append(steps, step{200, openJSON(kind, m, "main")})
				}
				if c.want == Merged {
					steps = append(steps, merged)
				}
				git := &fakeGit{conflicts: c.git}
				res, s, _ := runConflictScenario(t, kind, steps, git)
				if res.Outcome != c.want {
					t.Fatalf("outcome = %d, want %d (err=%v)", res.Outcome, c.want, res.Err)
				}
				if s.calls != c.fetches {
					t.Fatalf("fetches = %d, want %d", s.calls, c.fetches)
				}
				if len(git.conflictAsk) != c.checks {
					t.Fatalf("local merge checks = %d, want %d", len(git.conflictAsk), c.checks)
				}
				if c.checks > 0 && git.conflictAsk[0] != "origin/main+#78@aaa111" {
					t.Fatalf("checked %q, want the PR head merged into origin/main", git.conflictAsk[0])
				}
				if (len(res.Files) > 0) != c.wantFiles {
					t.Fatalf("files = %q, want files %t", res.Files, c.wantFiles)
				}
				if (res.Err != nil) != c.wantErr {
					t.Fatalf("err = %v, want error %t", res.Err, c.wantErr)
				}
				if c.want == Conflicted && res.Outcome.ExitCode() != 7 {
					t.Fatalf("exit code = %d, want 7", res.Outcome.ExitCode())
				}
			})
		}
	}
}

// The conflict check and the summary use the PR's own base, not the default branch.
func TestWaitConflictUsesThePRBase(t *testing.T) {
	for _, kind := range forgeKinds {
		t.Run(kind, func(t *testing.T) {
			git := &fakeGit{conflicts: []conflictAnswer{{files: []string{"go.mod"}}}}
			res, _, opt := runConflictScenario(t, kind, []step{{200, openJSON(kind, "false", "release")}}, git)
			if res.Outcome != Conflicted {
				t.Fatalf("outcome = %d, want Conflicted (err=%v)", res.Outcome, res.Err)
			}
			if git.conflictAsk[0] != "origin/release+#78@aaa111" {
				t.Fatalf("checked %q, want origin/release", git.conflictAsk[0])
			}
			line := res.Summary(opt.Target, opt)
			if !strings.Contains(line, "conflicts with release") || !strings.Contains(line, "merge origin/release") {
				t.Fatalf("summary %q does not name the release base", line)
			}
		})
	}
}

func TestConflictSummaryNamesFilesAndPushTarget(t *testing.T) {
	target := Target{Forge: Forge{Host: "codeberg.org", Kind: "forgejo"}, Owner: "ubunatic", Repo: "harnez", Number: 78}
	opt := Options{Target: target, Remote: "origin", Branch: "main"}
	base := PullRef{Ref: "main"}
	cases := []struct {
		name  string
		info  PullInfo
		files []string
		want  []string
	}{
		{"agit topic", PullInfo{Flow: 1, Head: PullRef{Ref: "refs/pull/78/head", Label: "flashcoder/issue-78"}, Base: base},
			[]string{"issues/README.md", "issues/BACKLOG.md"},
			[]string{"PR 78 has merge conflicts with main", "files=issues/README.md,issues/BACKLOG.md",
				"https://codeberg.org/ubunatic/harnez/pulls/78", "merge origin/main",
				"push as flashcoder: git push origin HEAD:refs/for/main/issue-78"}},
		{"agit topic with a slash", PullInfo{Flow: 1, Head: PullRef{Label: "flashcoder/issue/78-x"}, Base: base}, nil,
			[]string{"files=unknown", "HEAD:refs/for/main/issue/78-x"}},
		{"branch in the same repo", PullInfo{Head: PullRef{Ref: "issue-78", Label: "ubunatic:issue-78", Repo: PullRepo{FullName: "ubunatic/harnez"}}, Base: base}, nil,
			[]string{"push it to branch issue-78"}},
		{"branch in a fork", PullInfo{Head: PullRef{Ref: "issue-78", Repo: PullRepo{FullName: "someone/harnez"}}, Base: base}, nil,
			[]string{"push it to branch issue-78 of someone/harnez"}},
		{"unknown head", PullInfo{Head: PullRef{Ref: "refs/pull/78/head"}, Base: base}, nil,
			[]string{"push it to the PR's head"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			line := Result{Outcome: Conflicted, Info: c.info, Files: c.files}.Summary(target, opt)
			for _, want := range c.want {
				if !strings.Contains(line, want) {
					t.Errorf("summary %q lacks %q", line, want)
				}
			}
		})
	}
}

func TestAGitTopicNeedsFlowAndLabel(t *testing.T) {
	cases := []struct {
		info          PullInfo
		pusher, topic string
		ok            bool
	}{
		{PullInfo{Flow: 1, Head: PullRef{Label: "flashcoder/ubunatic"}}, "flashcoder", "ubunatic", true},
		{PullInfo{Flow: 0, Head: PullRef{Label: "flashcoder/ubunatic"}}, "", "", false},
		{PullInfo{Flow: 1, Head: PullRef{Label: "ubunatic"}}, "", "", false},
		{PullInfo{Flow: 1, Head: PullRef{Label: "/topic"}}, "", "", false},
	}
	for _, c := range cases {
		pusher, topic, ok := c.info.AGitTopic()
		if pusher != c.pusher || topic != c.topic || ok != c.ok {
			t.Errorf("AGitTopic(%+v) = %q, %q, %t; want %q, %q, %t", c.info, pusher, topic, ok, c.pusher, c.topic, c.ok)
		}
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.test"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// forgeRepo is a bare "forge" with main and refs/pull/1/head, plus a clone of it.
// pr changes a.txt or b.txt; main always changes a.txt.
func forgeRepo(t *testing.T, prFile string) (clone, head string) {
	t.Helper()
	root := t.TempDir()
	bare := filepath.Join(root, "origin.git")
	git(t, root, "init", "-q", "--bare", "-b", "main", bare)
	work := filepath.Join(root, "work")
	git(t, root, "init", "-q", "-b", "main", work)
	writeFile(t, filepath.Join(work, "a.txt"), "base\n")
	writeFile(t, filepath.Join(work, "b.txt"), "base\n")
	// A merge driver that always "resolves" a.txt. The forge does not run it, so neither
	// may the local check.
	writeFile(t, filepath.Join(work, ".gitattributes"), "a.txt merge=keep\n")
	git(t, work, "add", ".")
	git(t, work, "commit", "-q", "-m", "base")
	git(t, work, "remote", "add", "origin", bare)
	git(t, work, "push", "-q", "origin", "main")
	git(t, work, "switch", "-q", "-c", "pr")
	writeFile(t, filepath.Join(work, prFile), "pr\n")
	git(t, work, "commit", "-q", "-am", "pr")
	head = git(t, work, "rev-parse", "HEAD")
	git(t, work, "push", "-q", "origin", "HEAD:refs/pull/1/head")
	git(t, work, "switch", "-q", "main")
	writeFile(t, filepath.Join(work, "a.txt"), "main\n")
	git(t, work, "commit", "-q", "-am", "main moves on")
	git(t, work, "push", "-q", "origin", "main")

	clone = filepath.Join(root, "clone")
	git(t, root, "clone", "-q", bare, clone)
	git(t, clone, "config", "merge.keep.driver", "true")
	return clone, head
}

func TestSystemGitConflictsListsFilesAndIgnoresMergeDrivers(t *testing.T) {
	clone, head := forgeRepo(t, "a.txt")
	before := git(t, clone, "status", "--porcelain")

	// Sanity: with the driver, a plain local merge-tree calls the merge clean. That is the trap
	// the check must avoid: the forge would keep saying unmergeable while git says clean.
	git(t, clone, "fetch", "-q", "origin", "refs/pull/1/head")
	git(t, clone, "merge-tree", "--write-tree", "origin/main", head)

	files, err := SystemGit{}.Conflicts(clone, "origin", "main", 1, head)
	if err != nil {
		t.Fatalf("Conflicts: %v", err)
	}
	if strings.Join(files, ",") != "a.txt" {
		t.Fatalf("files = %q, want [a.txt]", files)
	}
	if after := git(t, clone, "status", "--porcelain"); after != before {
		t.Fatalf("Conflicts changed the worktree: %q -> %q", before, after)
	}
}

func TestSystemGitConflictsCleanMerge(t *testing.T) {
	clone, head := forgeRepo(t, "b.txt")
	files, err := SystemGit{}.Conflicts(clone, "origin", "main", 1, head)
	if err != nil || len(files) != 0 {
		t.Fatalf("Conflicts = %q, %v; want a clean merge", files, err)
	}
}

func TestSystemGitConflictsReportsFetchErrors(t *testing.T) {
	clone, head := forgeRepo(t, "a.txt")
	_, err := SystemGit{}.Conflicts(clone, "origin", "main", 99, head)
	if err == nil || !strings.Contains(err.Error(), "refs/pull/99/head") {
		t.Fatalf("err = %v, want the failed fetch of refs/pull/99/head", err)
	}
}

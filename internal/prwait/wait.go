package prwait

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Clock supplies the current time and blocks between polls. Tests inject a fake clock.
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

// SystemClock is the real wall clock.
type SystemClock struct{}

// Now returns the current time.
func (SystemClock) Now() time.Time { return time.Now() }

// Sleep blocks for d.
func (SystemClock) Sleep(d time.Duration) { time.Sleep(d) }

// Outcome is the final state of a wait. Each outcome maps to one exit code.
type Outcome int

const (
	Merged             Outcome = iota // exit 0
	Failed                            // exit 1: local failure such as a git error
	ClosedUnmerged                    // exit 2
	TimedOut                          // exit 3
	APIFailed                         // exit 4
	NotOnBranch                       // exit 5: merged, but --contains sha is not on the default branch
	ContainsUnverified                // exit 6: merged, but neither the forge nor local git could check --contains
	Conflicted                        // exit 7: open, but the head conflicts with the base
)

// unconfirmedConflictPolls is how many polls in a row the forge must call an open PR
// unmergeable before pr wait exits without a local merge check to confirm it.
const unconfirmedConflictPolls = 2

// ExitCode is the process exit code for the outcome.
func (o Outcome) ExitCode() int {
	return int(o)
}

// Options controls one wait.
type Options struct {
	Target    Target
	Poll      time.Duration
	Timeout   time.Duration
	MaxErrors int
	// Contains, when set, is a commit that must be an ancestor of Remote/Branch after a merge.
	Contains string
	Dir      string
	Remote   string
	Branch   string
	// Progress receives a line for each fixup pushed while the wait goes on; nil discards it.
	Progress io.Writer
}

// Fixer resolves a confirmed conflict and pushes the fix to the PR's own head (--fixup).
type Fixer interface {
	Fixup(req FixupRequest) (FixupResult, error)
}

// FixupRequest is a PR whose head conflicts with its base in Files.
type FixupRequest struct {
	Target Target
	Info   PullInfo
	Dir    string
	Remote string
	Base   string
	Files  []string
}

// FixupResult is the merge commit a fixup pushed and where it went.
type FixupResult struct {
	Head        string   // the pushed merge commit; the PR head must become this
	Dest        string   // the pushed ref, e.g. refs/for/main/<topic>
	Regenerated []string // the conflicting files the fixup regenerated
}

// Result is the outcome plus the last pull request state seen.
type Result struct {
	Outcome Outcome
	Info    PullInfo
	Err     error
	// Files are the conflicting paths of a Conflicted result, when local git could list them.
	Files []string
}

// Waiter polls a pull request until it is merged, closed, or the timeout passes.
// Compare is optional; without it --contains uses local git only. Fix is optional; with it
// (--fixup) the first confirmed conflict is fixed and pushed instead of ending the wait.
type Waiter struct {
	Fetch   Fetcher
	Clock   Clock
	Git     GitOps
	Compare Comparer
	Fix     Fixer
}

// Wait polls until a terminal state. The first poll happens immediately.
func (w Waiter) Wait(o Options) Result {
	deadline := w.Clock.Now().Add(o.Timeout)
	consecutive := 0
	var cs conflictState
	var last PullInfo
	for {
		info, err := w.Fetch.Fetch(o.Target)
		if err != nil {
			apiErr, _ := err.(*APIError)
			if apiErr == nil || !apiErr.Transient {
				return Result{Outcome: APIFailed, Info: last, Err: err}
			}
			consecutive++
			if consecutive >= o.MaxErrors {
				return Result{Outcome: APIFailed, Info: last, Err: fmt.Errorf("%d consecutive transient errors: %w", consecutive, err)}
			}
		} else {
			consecutive = 0
			last = info
			switch {
			case info.Merged:
				return w.afterMerge(o, info)
			case info.State == "closed":
				return Result{Outcome: ClosedUnmerged, Info: info}
			case info.State != "open":
				return Result{Outcome: APIFailed, Info: info, Err: fmt.Errorf("unexpected pull request state %q", info.State)}
			}
			if res, done := w.pollOpen(o, info, &cs); done {
				return res
			}
		}
		remaining := deadline.Sub(w.Clock.Now())
		if remaining <= 0 {
			return Result{Outcome: TimedOut, Info: last}
		}
		wait := o.Poll
		if remaining < wait {
			wait = remaining
		}
		w.Clock.Sleep(wait)
	}
}

// conflictState carries the conflict checks of one wait across polls.
type conflictState struct {
	unmergeable int    // polls in a row the forge called the PR unmergeable
	fixed       string // the head a fixup pushed; at most one fixup per wait
	pending     bool   // the forge has not shown fixed as the head yet
	stale       int    // polls since the fixup that still show another head
}

// pollOpen handles one poll of an open PR: it ends the wait on a confirmed conflict, or runs
// the one --fixup the wait allows and keeps waiting.
func (w Waiter) pollOpen(o Options, info PullInfo, cs *conflictState) (Result, bool) {
	if cs.pending && info.Head.SHA == cs.fixed {
		cs.pending = false
	}
	if cs.pending {
		// Forgejo and GitHub update the head on push, so a head that stays old means the
		// push went elsewhere.
		cs.stale++
		if cs.stale < unconfirmedConflictPolls {
			return Result{}, false
		}
		return Result{Outcome: Conflicted, Info: info, Err: fmt.Errorf("fixup pushed %s, but PR %d still shows head %s after %d polls",
			cs.fixed, o.Target.Number, info.Head.SHA, cs.stale)}, true
	}
	if info.Mergeable == nil || *info.Mergeable {
		cs.unmergeable = 0
		return Result{}, false
	}
	cs.unmergeable++
	res, done := w.checkConflict(o, info, cs.unmergeable)
	if !done {
		return Result{}, false
	}
	if cs.fixed != "" {
		res.Err = errors.Join(res.Err, fmt.Errorf("still conflicting after the fixup pushed %s", cs.fixed))
		return res, true
	}
	if w.Fix == nil || len(res.Files) == 0 {
		return res, true
	}
	base := baseBranch(o, info)
	fix, err := w.Fix.Fixup(FixupRequest{Target: o.Target, Info: info, Dir: o.Dir, Remote: o.Remote, Base: base, Files: res.Files})
	if err != nil {
		res.Err = fmt.Errorf("fixup failed: %w", err)
		return res, true
	}
	cs.fixed, cs.pending, cs.unmergeable = fix.Head, true, 0
	if o.Progress != nil {
		fmt.Fprintf(o.Progress, "PR %d fixup: merged %s/%s, regenerated %s, pushed %s to %s; waiting again\n",
			o.Target.Number, o.Remote, base, strings.Join(fix.Regenerated, ","), fix.Head, fix.Dest)
	}
	return Result{}, false
}

// checkConflict decides whether an open PR the forge calls unmergeable really conflicts.
// A local merge of the head into the base is the proof and names the files. A clean local
// merge means the forge is still rechecking or the PR is a work in progress, so polling goes
// on. When git cannot check, the forge's answer counts once it holds for
// unconfirmedConflictPolls polls in a row.
func (w Waiter) checkConflict(o Options, info PullInfo, streak int) (Result, bool) {
	files, err := w.Git.Conflicts(o.Dir, o.Remote, baseBranch(o, info), o.Target.Number, info.Head.SHA)
	if err == nil {
		if len(files) == 0 {
			return Result{}, false
		}
		return Result{Outcome: Conflicted, Info: info, Files: files}, true
	}
	if streak < unconfirmedConflictPolls {
		return Result{}, false
	}
	return Result{Outcome: Conflicted, Info: info,
		Err: fmt.Errorf("forge reported unmergeable %d polls in a row; local merge check failed: %w", streak, err)}, true
}

// baseBranch is the PR's base branch, or the configured default when the forge omits it.
func baseBranch(o Options, info PullInfo) string {
	if info.Base.Ref != "" {
		return info.Base.Ref
	}
	return o.Branch
}

// afterMerge runs the optional --contains check: the commit must be on the default branch.
// The forge API answers first; local git is the fallback. A seen merge never ends in Failed
// (issue 1248): when neither path can answer, the outcome is ContainsUnverified.
func (w Waiter) afterMerge(o Options, info PullInfo) Result {
	if o.Contains == "" {
		return Result{Outcome: Merged, Info: info}
	}
	var apiErr error
	if w.Compare != nil {
		on, err := w.Compare.OnBranch(o.Target, o.Branch, o.Contains)
		if err == nil {
			return containsResult(on, info)
		}
		apiErr = fmt.Errorf("forge compare: %w", err)
	}
	on, gitErr := w.localContains(o)
	if gitErr == nil {
		return containsResult(on, info)
	}
	return Result{Outcome: ContainsUnverified, Info: info, Err: errors.Join(apiErr, gitErr)}
}

func (w Waiter) localContains(o Options) (bool, error) {
	if err := w.Git.FetchRemote(o.Dir, o.Remote); err != nil {
		return false, err
	}
	return w.Git.IsAncestor(o.Dir, o.Contains, o.Remote+"/"+o.Branch)
}

func containsResult(on bool, info PullInfo) Result {
	if !on {
		return Result{Outcome: NotOnBranch, Info: info}
	}
	return Result{Outcome: Merged, Info: info}
}

// Summary is the one line printed for a finished wait. It never contains a token.
func (r Result) Summary(t Target, o Options) string {
	page := t.PageURL()
	switch r.Outcome {
	case Merged:
		line := fmt.Sprintf("PR %d merged: state=%s merged=%t merge_commit=%s url=%s",
			t.Number, r.Info.State, r.Info.Merged, r.Info.MergeCommitSHA, page)
		if o.Contains != "" {
			line += fmt.Sprintf(" contains=%s on %s/%s", o.Contains, o.Remote, o.Branch)
		}
		return line
	case ClosedUnmerged:
		return fmt.Sprintf("PR %d closed without merge: state=%s merged=false url=%s", t.Number, r.Info.State, page)
	case TimedOut:
		return fmt.Sprintf("PR %d still open after %s: state=%s url=%s", t.Number, o.Timeout, stateOrUnknown(r.Info.State), page)
	case NotOnBranch:
		return fmt.Sprintf("PR %d merged as %s, but %s is not on %s/%s: url=%s",
			t.Number, r.Info.MergeCommitSHA, o.Contains, o.Remote, o.Branch, page)
	case ContainsUnverified:
		return fmt.Sprintf("PR %d merged as %s, but whether %s is on %s/%s is unverified: url=%s",
			t.Number, r.Info.MergeCommitSHA, o.Contains, o.Remote, o.Branch, page)
	case Conflicted:
		return conflictSummary(t, o, r)
	default:
		return ""
	}
}

// conflictSummary names the base, the conflicting files and where a fix must be pushed so it
// updates this PR instead of opening another.
func conflictSummary(t Target, o Options, r Result) string {
	base := baseBranch(o, r.Info)
	files := "unknown"
	if len(r.Files) > 0 {
		files = strings.Join(r.Files, ",")
	}
	line := fmt.Sprintf("PR %d has merge conflicts with %s: files=%s url=%s; merge %s/%s into the PR head and ",
		t.Number, base, files, t.PageURL(), o.Remote, base)
	if pusher, topic, ok := r.Info.AGitTopic(); ok {
		return line + fmt.Sprintf("push as %s: git push %s HEAD:refs/for/%s/%s", pusher, o.Remote, base, topic)
	}
	if ref := r.Info.Head.Ref; ref != "" && !strings.HasPrefix(ref, "refs/") {
		if repo := r.Info.Head.Repo.FullName; repo != "" && repo != t.Owner+"/"+t.Repo {
			return line + fmt.Sprintf("push it to branch %s of %s", ref, repo)
		}
		return line + fmt.Sprintf("push it to branch %s", ref)
	}
	return line + "push it to the PR's head"
}

func stateOrUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

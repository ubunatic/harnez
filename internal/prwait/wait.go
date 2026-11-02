package prwait

import (
	"fmt"
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
	Merged         Outcome = iota // exit 0
	Failed                        // exit 1: local failure such as a git error
	ClosedUnmerged                // exit 2
	TimedOut                      // exit 3
	APIFailed                     // exit 4
	NotOnBranch                   // exit 5: merged, but --contains sha is not on the default branch
)

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
}

// Result is the outcome plus the last pull request state seen.
type Result struct {
	Outcome Outcome
	Info    PullInfo
	Err     error
}

// Waiter polls a pull request until it is merged, closed, or the timeout passes.
type Waiter struct {
	Fetch Fetcher
	Clock Clock
	Git   GitOps
}

// Wait polls until a terminal state. The first poll happens immediately.
func (w Waiter) Wait(o Options) Result {
	deadline := w.Clock.Now().Add(o.Timeout)
	consecutive := 0
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

// afterMerge runs the optional --contains check: the commit must be on the default branch.
func (w Waiter) afterMerge(o Options, info PullInfo) Result {
	if o.Contains == "" {
		return Result{Outcome: Merged, Info: info}
	}
	if err := w.Git.FetchRemote(o.Dir, o.Remote); err != nil {
		return Result{Outcome: Failed, Info: info, Err: err}
	}
	ok, err := w.Git.IsAncestor(o.Dir, o.Contains, o.Remote+"/"+o.Branch)
	if err != nil {
		return Result{Outcome: Failed, Info: info, Err: err}
	}
	if !ok {
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
	default:
		return ""
	}
}

func stateOrUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

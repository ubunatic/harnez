package prwait

import (
	"fmt"
	"strings"
	"time"
)

// Request is one `harnez pr wait` invocation, before the spec defaults are applied.
type Request struct {
	Arg      string        // pull request number, or a pull request page URL
	Remote   string        // git remote name; empty uses the spec's default_remote
	Dir      string        // repository directory; empty means the current directory
	Timeout  time.Duration // zero uses the spec's default_timeout
	Contains string        // optional commit that must be on the default branch after a merge
}

// Deps are the side effects Run needs. Tests inject fakes; the command uses SystemDeps.
type Deps struct {
	Fetch Fetcher
	Clock Clock
	Git   GitOps
}

// Report is what the command prints and turns into an exit code.
type Report struct {
	Outcome Outcome
	Line    string // one summary line for stdout; empty when there is nothing to report
	Err     error  // detail for stderr; never contains a token
}

// SystemDeps wires the real HTTP fetcher, wall clock and git binary, with tokens read from getenv.
func SystemDeps(spec Spec, getenv func(string) string) Deps {
	ts := SystemTokenSource(getenv)
	return Deps{
		Fetch: NewHTTPFetcher(spec.RequestTimeout, ts.For),
		Clock: SystemClock{},
		Git:   SystemGit{},
	}
}

// Run resolves the target, polls until a terminal state, and reports the outcome.
func Run(spec Spec, deps Deps, req Request) Report {
	if req.Timeout < 0 {
		return Report{Outcome: Failed, Err: fmt.Errorf("--timeout must not be negative")}
	}
	remote := req.Remote
	if remote == "" {
		remote = spec.DefaultRemote
	}
	dir := req.Dir
	if dir == "" {
		dir = "."
	}
	remoteURL := ""
	if !strings.Contains(req.Arg, "://") {
		u, err := deps.Git.RemoteURL(dir, remote)
		if err != nil {
			return Report{Outcome: Failed, Err: err}
		}
		remoteURL = u
	}
	target, err := ResolveTarget(spec, req.Arg, remoteURL)
	if err != nil {
		return Report{Outcome: Failed, Err: err}
	}
	timeout := req.Timeout
	if timeout == 0 {
		timeout = spec.DefaultTimeout
	}
	opt := Options{
		Target:    target,
		Poll:      spec.PollInterval,
		Timeout:   timeout,
		MaxErrors: spec.MaxConsecutiveErrors,
		Contains:  req.Contains,
		Dir:       dir,
		Remote:    remote,
		Branch:    spec.DefaultBranch,
	}
	res := Waiter(deps).Wait(opt)
	return Report{Outcome: res.Outcome, Line: res.Summary(target, opt), Err: res.Err}
}

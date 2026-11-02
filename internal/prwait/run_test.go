package prwait

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// fakeFetcher answers every Fetch from fn, so Run tests never touch the network.
type fakeFetcher struct {
	fn    func(Target) (PullInfo, error)
	calls int
}

func (f *fakeFetcher) Fetch(t Target) (PullInfo, error) {
	f.calls++
	return f.fn(t)
}

func testSpec() Spec {
	return Spec{
		DefaultRemote:        "origin",
		DefaultBranch:        "main",
		PollInterval:         30 * time.Second,
		DefaultTimeout:       12 * time.Hour,
		RequestTimeout:       time.Second,
		MaxConsecutiveErrors: 3,
		Forges: []Forge{
			{Host: "codeberg.org", Kind: "forgejo", APIBase: "https://codeberg.org/api/v1"},
			{Host: "github.com", Kind: "github", APIBase: "https://api.github.com"},
		},
	}
}

func mergedInfo(Target) (PullInfo, error) {
	return PullInfo{Number: 78, State: "closed", Merged: true, MergeCommitSHA: "d2044b5a3915", HTMLURL: "https://codeberg.org/ubunatic/harnez/pulls/78"}, nil
}

func TestRunNumberTakesForgeFromRemote(t *testing.T) {
	git := &fakeGit{url: "ssh://git@codeberg.org/ubunatic/harnez.git"}
	fetch := &fakeFetcher{fn: mergedInfo}
	rep := Run(testSpec(), Deps{Fetch: fetch, Clock: &fakeClock{}, Git: git}, Request{Arg: "78"})
	if rep.Outcome != Merged || rep.Err != nil {
		t.Fatalf("outcome = %d err = %v", rep.Outcome, rep.Err)
	}
	if git.asked != 1 {
		t.Fatalf("remote URL asked %d times, want 1", git.asked)
	}
	if !strings.HasPrefix(rep.Line, "PR 78 merged: state=closed merged=true merge_commit=d2044b5a3915") {
		t.Fatalf("line = %q", rep.Line)
	}
}

func TestRunURLSkipsRemoteLookup(t *testing.T) {
	git := &fakeGit{urlErr: fmt.Errorf("no remote")}
	fetch := &fakeFetcher{fn: mergedInfo}
	rep := Run(testSpec(), Deps{Fetch: fetch, Clock: &fakeClock{}, Git: git},
		Request{Arg: "https://codeberg.org/ubunatic/harnez/pulls/78"})
	if rep.Outcome != Merged {
		t.Fatalf("outcome = %d err = %v", rep.Outcome, rep.Err)
	}
	if git.asked != 0 {
		t.Fatalf("a URL argument must not read the remote, asked %d times", git.asked)
	}
}

func TestRunTimeoutDefaultsAndOverride(t *testing.T) {
	open := func(Target) (PullInfo, error) { return PullInfo{State: "open"}, nil }

	clock := &fakeClock{}
	start := clock.Now()
	rep := Run(testSpec(), Deps{Fetch: &fakeFetcher{fn: open}, Clock: clock, Git: &fakeGit{url: "https://codeberg.org/ubunatic/harnez.git"}},
		Request{Arg: "78", Timeout: 60 * time.Second})
	if rep.Outcome != TimedOut {
		t.Fatalf("outcome = %d, want TimedOut", rep.Outcome)
	}
	if waited := clock.Now().Sub(start); waited != 60*time.Second {
		t.Fatalf("waited %v, want the 60s override", waited)
	}
	if !strings.Contains(rep.Line, "still open after 1m0s") {
		t.Fatalf("line = %q", rep.Line)
	}
}

func TestRunDefaultTimeoutFromSpec(t *testing.T) {
	clock := &fakeClock{}
	start := clock.Now()
	spec := testSpec()
	spec.DefaultTimeout = 90 * time.Second
	open := func(Target) (PullInfo, error) { return PullInfo{State: "open"}, nil }
	rep := Run(spec, Deps{Fetch: &fakeFetcher{fn: open}, Clock: clock, Git: &fakeGit{url: "https://codeberg.org/ubunatic/harnez.git"}}, Request{Arg: "78"})
	if waited := clock.Now().Sub(start); rep.Outcome != TimedOut || waited != 90*time.Second {
		t.Fatalf("outcome = %d waited %v, want TimedOut after the spec's 90s", rep.Outcome, waited)
	}
}

func TestRunUsageAndConfigErrorsExitOne(t *testing.T) {
	cases := []struct {
		name string
		git  *fakeGit
		req  Request
	}{
		{"bad number", &fakeGit{url: "https://codeberg.org/o/r.git"}, Request{Arg: "abc"}},
		{"missing remote", &fakeGit{urlErr: fmt.Errorf("no such remote")}, Request{Arg: "78", Remote: "nope"}},
		{"unconfigured host", &fakeGit{url: "https://gitlab.example/o/r.git"}, Request{Arg: "78"}},
		{"negative timeout", &fakeGit{url: "https://codeberg.org/o/r.git"}, Request{Arg: "78", Timeout: -time.Second}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fetch := &fakeFetcher{fn: mergedInfo}
			rep := Run(testSpec(), Deps{Fetch: fetch, Clock: &fakeClock{}, Git: c.git}, c.req)
			if rep.Outcome != Failed || rep.Err == nil {
				t.Fatalf("outcome = %d err = %v, want Failed with an error", rep.Outcome, rep.Err)
			}
			if fetch.calls != 0 {
				t.Fatalf("made %d API calls for an invalid request", fetch.calls)
			}
		})
	}
}

func TestRunContainsNotOnDefaultBranchExitsFive(t *testing.T) {
	git := &fakeGit{url: "https://codeberg.org/ubunatic/harnez.git", ancestor: false}
	rep := Run(testSpec(), Deps{Fetch: &fakeFetcher{fn: mergedInfo}, Clock: &fakeClock{}, Git: git},
		Request{Arg: "78", Contains: "d2044b5a3915"})
	if rep.Outcome != NotOnBranch || rep.Outcome.ExitCode() != 5 {
		t.Fatalf("outcome = %d, want NotOnBranch (exit 5)", rep.Outcome)
	}
	if len(git.checked) != 1 || git.checked[0] != "d2044b5a3915@origin/main" {
		t.Fatalf("checked %v, want the sha against origin/main", git.checked)
	}
}

func TestExitCodesMatchTheContract(t *testing.T) {
	want := map[Outcome]int{Merged: 0, Failed: 1, ClosedUnmerged: 2, TimedOut: 3, APIFailed: 4, NotOnBranch: 5}
	for o, code := range want {
		if o.ExitCode() != code {
			t.Errorf("outcome %d exit = %d, want %d", o, o.ExitCode(), code)
		}
	}
}

package prwait

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeClock advances only when Sleep is called, so no test waits in real time.
type fakeClock struct {
	now   time.Time
	slept []time.Duration
}

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) Sleep(d time.Duration) {
	c.slept = append(c.slept, d)
	c.now = c.now.Add(d)
}

type fakeGit struct {
	fetchErr  error
	ancestor  bool
	ancestorE error
	fetched   int
	checked   []string
}

func (g *fakeGit) RemoteURL(string, string) (string, error) { return "", nil }

func (g *fakeGit) FetchRemote(string, string) error {
	g.fetched++
	return g.fetchErr
}

func (g *fakeGit) IsAncestor(_ string, sha, ref string) (bool, error) {
	g.checked = append(g.checked, sha+"@"+ref)
	return g.ancestor, g.ancestorE
}

// step is one scripted HTTP response. The last step repeats once the script is exhausted.
type step struct {
	status int
	body   string
}

type scripted struct {
	mu    sync.Mutex
	steps []step
	calls int
	auth  []string
}

func (s *scripted) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/repos/ubunatic/harnez/pulls/78") {
			t.Errorf("unexpected request path %q", r.URL.Path)
		}
		s.mu.Lock()
		i := s.calls
		s.calls++
		s.auth = append(s.auth, r.Header.Get("Authorization"))
		s.mu.Unlock()
		if i >= len(s.steps) {
			i = len(s.steps) - 1
		}
		st := s.steps[i]
		w.WriteHeader(st.status)
		_, _ = io.WriteString(w, st.body)
	}
}

func prJSON(state string, merged bool) string {
	return fmt.Sprintf(`{"number":78,"state":%q,"merged":%t,"merged_at":null,"merge_commit_sha":"d2044b5a3915","html_url":"https://example.test/pulls/78"}`, state, merged)
}

// forgeFor builds the spec entry a test server stands in for, in the real API shape of each kind.
func forgeFor(kind, base string) Forge {
	if kind == "github" {
		return Forge{Host: "github.com", Kind: "github", APIBase: base}
	}
	return Forge{Host: "codeberg.org", Kind: "forgejo", APIBase: base + "/api/v1"}
}

var forgeKinds = []string{"forgejo", "github"}

func newWaiter(fetcher Fetcher, clock *fakeClock, git GitOps) Waiter {
	return Waiter{Fetch: fetcher, Clock: clock, Git: git}
}

func runScenario(t *testing.T, kind string, steps []step, opt Options) (Result, *scripted, *fakeClock) {
	t.Helper()
	s := &scripted{steps: steps}
	srv := httptest.NewServer(s.handler(t))
	t.Cleanup(srv.Close)
	clock := &fakeClock{now: time.Unix(0, 0)}
	target := Target{Forge: forgeFor(kind, srv.URL), Owner: "ubunatic", Repo: "harnez", Number: 78}
	opt.Target = target
	if opt.Poll == 0 {
		opt.Poll = 30 * time.Second
	}
	if opt.MaxErrors == 0 {
		opt.MaxErrors = 3
	}
	if opt.Remote == "" {
		opt.Remote = "origin"
	}
	if opt.Branch == "" {
		opt.Branch = "main"
	}
	client := NewHTTPFetcher(5*time.Second, nil)
	res := newWaiter(client, clock, &fakeGit{}).Wait(opt)
	return res, s, clock
}

func TestWaitScenarios(t *testing.T) {
	cases := []struct {
		name      string
		steps     []step
		timeout   time.Duration
		maxErrors int
		want      Outcome
		fetches   int
	}{
		{
			name:    "merged at once",
			steps:   []step{{200, prJSON("closed", true)}},
			timeout: time.Hour, want: Merged, fetches: 1,
		},
		{
			name:    "closed without merge",
			steps:   []step{{200, prJSON("closed", false)}},
			timeout: time.Hour, want: ClosedUnmerged, fetches: 1,
		},
		{
			name:    "open until merged",
			steps:   []step{{200, prJSON("open", false)}, {200, prJSON("open", false)}, {200, prJSON("closed", true)}},
			timeout: time.Hour, want: Merged, fetches: 3,
		},
		{
			name:    "open until timeout",
			steps:   []step{{200, prJSON("open", false)}},
			timeout: 60 * time.Second, want: TimedOut, fetches: 3,
		},
		{
			name:    "404 is not retried",
			steps:   []step{{404, `{"message":"Not Found"}`}},
			timeout: time.Hour, want: APIFailed, fetches: 1,
		},
		{
			name:    "401 is not retried",
			steps:   []step{{401, `{"message":"bad credentials"}`}},
			timeout: time.Hour, want: APIFailed, fetches: 1,
		},
		{
			name:    "transient error then success",
			steps:   []step{{500, "oops"}, {200, prJSON("closed", true)}},
			timeout: time.Hour, want: Merged, fetches: 2,
		},
		{
			name:      "transient errors exhaust the budget",
			steps:     []step{{503, "busy"}},
			timeout:   time.Hour,
			maxErrors: 3, want: APIFailed, fetches: 3,
		},
		{
			name:    "undecodable body is an API error",
			steps:   []step{{200, "<html>"}},
			timeout: time.Hour, want: APIFailed, fetches: 1,
		},
	}
	for _, kind := range forgeKinds {
		for _, c := range cases {
			t.Run(kind+"/"+c.name, func(t *testing.T) {
				opt := Options{Timeout: c.timeout, MaxErrors: c.maxErrors}
				res, s, _ := runScenario(t, kind, c.steps, opt)
				if res.Outcome != c.want {
					t.Fatalf("outcome = %d, want %d (err=%v)", res.Outcome, c.want, res.Err)
				}
				if s.calls != c.fetches {
					t.Fatalf("fetches = %d, want %d", s.calls, c.fetches)
				}
			})
		}
	}
}

func TestWaitOpenUntilTimeoutSleepsBetweenPolls(t *testing.T) {
	res, _, clock := runScenario(t, "forgejo", []step{{200, prJSON("open", false)}},
		Options{Timeout: 60 * time.Second, Poll: 30 * time.Second})
	if res.Outcome != TimedOut {
		t.Fatalf("outcome = %d, want TimedOut", res.Outcome)
	}
	if len(clock.slept) != 2 || clock.slept[0] != 30*time.Second || clock.slept[1] != 30*time.Second {
		t.Fatalf("slept = %v, want two 30s polls", clock.slept)
	}
}

func TestWaitTransportFailureIsTransient(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	base := srv.URL
	srv.Close() // nothing listens any more: every request fails at the transport
	clock := &fakeClock{now: time.Unix(0, 0)}
	target := Target{Forge: forgeFor("forgejo", base), Owner: "ubunatic", Repo: "harnez", Number: 78}
	res := newWaiter(NewHTTPFetcher(time.Second, nil), clock, &fakeGit{}).Wait(
		Options{Target: target, Poll: time.Second, Timeout: time.Hour, MaxErrors: 2, Remote: "origin", Branch: "main"})
	if res.Outcome != APIFailed {
		t.Fatalf("outcome = %d, want APIFailed", res.Outcome)
	}
	if !strings.Contains(res.Err.Error(), "consecutive transient errors") {
		t.Fatalf("err = %v, want the transient budget message", res.Err)
	}
}

func TestWaitContainsChecksDefaultBranch(t *testing.T) {
	merged := []step{{200, prJSON("closed", true)}}
	cases := []struct {
		name     string
		git      *fakeGit
		want     Outcome
		wantFetc int
	}{
		{"ancestor of default branch", &fakeGit{ancestor: true}, Merged, 1},
		{"not on default branch", &fakeGit{ancestor: false}, NotOnBranch, 1},
		{"fetch fails", &fakeGit{fetchErr: fmt.Errorf("network down")}, Failed, 1},
		{"ancestry check errors", &fakeGit{ancestorE: fmt.Errorf("bad object")}, Failed, 1},
	}
	for _, kind := range forgeKinds {
		for _, c := range cases {
			t.Run(kind+"/"+c.name, func(t *testing.T) {
				s := &scripted{steps: merged}
				srv := httptest.NewServer(s.handler(t))
				t.Cleanup(srv.Close)
				target := Target{Forge: forgeFor(kind, srv.URL), Owner: "ubunatic", Repo: "harnez", Number: 78}
				opt := Options{Target: target, Poll: time.Second, Timeout: time.Hour, MaxErrors: 3,
					Contains: "d2044b5a3915", Remote: "origin", Branch: "main"}
				res := newWaiter(NewHTTPFetcher(time.Second, nil), &fakeClock{}, c.git).Wait(opt)
				if res.Outcome != c.want {
					t.Fatalf("outcome = %d, want %d (err=%v)", res.Outcome, c.want, res.Err)
				}
				if len(c.git.checked) > 0 && c.git.checked[0] != "d2044b5a3915@origin/main" {
					t.Fatalf("checked %q, want the sha against origin/main", c.git.checked[0])
				}
			})
		}
	}
}

func TestWaitSummaryNamesPRStateAndMergeCommit(t *testing.T) {
	res, _, _ := runScenario(t, "forgejo", []step{{200, prJSON("closed", true)}}, Options{Timeout: time.Hour})
	target := Target{Forge: Forge{Host: "codeberg.org", Kind: "forgejo"}, Owner: "ubunatic", Repo: "harnez", Number: 78}
	line := res.Summary(target, Options{Timeout: time.Hour})
	for _, want := range []string{"PR 78 merged", "state=closed", "merged=true", "merge_commit=d2044b5a3915", "https://codeberg.org/ubunatic/harnez/pulls/78"} {
		if !strings.Contains(line, want) {
			t.Errorf("summary %q lacks %q", line, want)
		}
	}
}

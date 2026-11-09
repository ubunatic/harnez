package forge

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// issueServer serves pages of issues: page p holds the items in pages[p-1].
func issueServer(t *testing.T, pages []string, seen *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.URL.String())
		var p int
		_, _ = fmt.Sscanf(r.URL.Query().Get("page"), "%d", &p) // a parse failure leaves the target at its zero value
		if p < 1 || p > len(pages) {
			fmt.Fprint(w, "[]")
			return
		}
		fmt.Fprint(w, pages[p-1])
	}))
}

func TestIssuesForgejoQueryAndPRFilter(t *testing.T) {
	var seen []string
	srv := issueServer(t, []string{
		`[{"number":3,"title":"a","state":"open"},{"number":2,"title":"pr","pull_request":{}}]`,
		`[{"number":1,"title":"b","state":"closed"}]`,
	}, &seen)
	defer srv.Close()
	l := NewPullLister(testScan, nil)
	got, truncated, err := l.Issues(Forge{Host: "x", Kind: "forgejo", APIBase: srv.URL}, "o", "r", "all", testScan)
	if err != nil || truncated {
		t.Fatalf("Issues: %v truncated=%v", err, truncated)
	}
	if len(got) != 2 || got[0].Number != 3 || got[1].Number != 1 {
		t.Fatalf("want issues 3 and 1 without the PR, got %+v", got)
	}
	first := seen[0]
	for _, want := range []string{"/repos/o/r/issues", "state=all", "type=issues", "limit=2", "page=1"} {
		if !strings.Contains(first, want) {
			t.Errorf("request %q missing %q", first, want)
		}
	}
	if strings.Contains(first, "q=") {
		t.Errorf("request %q must not send a server search", first)
	}
}

// TestIssuesStateParam checks that the state argument reaches the query as given, and that
// an empty state means all states.
func TestIssuesStateParam(t *testing.T) {
	for _, c := range []struct{ state, want string }{{"open", "open"}, {"closed", "closed"}, {"", "all"}} {
		var seen []string
		srv := issueServer(t, []string{`[]`}, &seen)
		l := NewPullLister(testScan, nil)
		_, _, err := l.Issues(Forge{Host: "x", Kind: "forgejo", APIBase: srv.URL}, "o", "r", c.state, testScan)
		srv.Close()
		if err != nil {
			t.Fatalf("state %q: %v", c.state, err)
		}
		u, err := url.Parse(seen[0])
		if err != nil || u.Query().Get("state") != c.want {
			t.Errorf("state %q: request %q, want state=%s", c.state, seen[0], c.want)
		}
	}
}

func TestIssuesGitHubPagingAndTruncation(t *testing.T) {
	var seen []string
	full := `[{"number":9,"title":"x"},{"number":8,"title":"y"}]`
	srv := issueServer(t, []string{full, full, full, full, full}, &seen)
	defer srv.Close()
	l := NewPullLister(testScan, nil)
	got, truncated, err := l.Issues(Forge{Host: "github.com", Kind: "github", APIBase: srv.URL}, "o", "r", "", testScan)
	if err != nil {
		t.Fatal(err)
	}
	if !truncated || len(got) != 8 || len(seen) != testScan.MaxPages {
		t.Fatalf("want 8 issues from %d pages and truncated, got %d issues, %d requests, truncated=%v", testScan.MaxPages, len(got), len(seen), truncated)
	}
	if !strings.Contains(seen[0], "per_page=2") || strings.Contains(seen[0], "type=issues") {
		t.Errorf("GitHub request %q: want per_page and no type param", seen[0])
	}
}

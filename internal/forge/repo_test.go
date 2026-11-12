package forge

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRepoProbeReportsRepoReachability checks the repo endpoint probe that `issues show`
// uses after an issue 404 (issue 1199): 200 means the repo is reachable, 404 is an APIError.
func TestRepoProbeReportsRepoReachability(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path)
		if r.URL.Path == "/repos/o/r" {
			_, _ = w.Write([]byte(`{"name":"r"}`))
			return
		}
		http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()
	fg := Forge{Host: "x", Kind: "forgejo", APIBase: srv.URL}
	l := NewPullLister(testScan, nil)
	if err := l.Repo(fg, "o", "r"); err != nil {
		t.Fatalf("reachable repo: %v", err)
	}
	err := l.Repo(fg, "o", "missing")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
		t.Fatalf("missing repo: want 404 APIError, got %v", err)
	}
	if len(seen) != 2 || !strings.HasSuffix(seen[1], "/repos/o/missing") {
		t.Fatalf("want one probe per call on the repo path, saw %q", seen)
	}
}

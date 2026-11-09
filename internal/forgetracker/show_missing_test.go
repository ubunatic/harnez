package forgetracker

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/forge"
)

// Repo answers the repo probe as missing: sample() has no repo behind it, so an issue 404
// keeps its original error. Tests that need a reachable repo use repoSource.
func (f *fakeSource) Repo(forge.Forge, string, string) error {
	return &forge.APIError{Status: 404, Detail: "repo"}
}

// repoSource is sample() with a chosen repo probe result.
type repoSource struct {
	*fakeSource
	repoErr error
}

func (r repoSource) Repo(forge.Forge, string, string) error { return r.repoErr }

// TestShowMissingIssueOnReachableRepo (issue 1199): a 404 on the issue with a reachable repo
// reports not found. The result must not be a forge.APIError, or the token hint would apply.
func TestShowMissingIssueOnReachableRepo(t *testing.T) {
	err := Show(&bytes.Buffer{}, repoSource{fakeSource: sample()}, target, "99", false)
	want := "issue #99 not found on codeberg.org/o/r"
	if err == nil || err.Error() != want {
		t.Fatalf("want %q, got %v", want, err)
	}
	var apiErr *forge.APIError
	if errors.As(err, &apiErr) {
		t.Fatalf("not-found must not wrap an APIError: %v", err)
	}
}

// TestShowMissingIssueKeepsErrorWhenRepoProbeIsNot200 (issue 1199): a repo 404, or any other
// probe failure, keeps the original issue 404 so the token hint still applies.
func TestShowMissingIssueKeepsErrorWhenRepoProbeIsNot200(t *testing.T) {
	for _, probe := range []error{
		&forge.APIError{Status: http.StatusNotFound, Detail: "repo"},
		&forge.APIError{Status: http.StatusInternalServerError, Detail: "repo"},
		&forge.APIError{Status: http.StatusUnauthorized, Detail: "repo"},
	} {
		err := Show(&bytes.Buffer{}, repoSource{fakeSource: sample(), repoErr: probe}, target, "99", false)
		var apiErr *forge.APIError
		if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
			t.Fatalf("probe %v: want the original 404 APIError, got %v", probe, err)
		}
		if strings.Contains(err.Error(), "not found on") {
			t.Fatalf("probe %v: must not claim the repo is reachable: %v", probe, err)
		}
	}
}

// TestShowProbesRepoThroughForgeReader (issue 1199) runs the real forge reader against a mock
// forge: the issue is 404 in both cases, and only the repo probe status differs.
func TestShowProbesRepoThroughForgeReader(t *testing.T) {
	for _, c := range []struct {
		name       string
		repoStatus int
		wantNotFnd bool
	}{
		{"repo reachable", http.StatusOK, true},
		{"repo missing", http.StatusNotFound, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/repos/o/r/issues/99", func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
			})
			mux.HandleFunc("/repos/o/r", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(c.repoStatus)
				fmt.Fprint(w, `{"name":"r"}`)
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()
			tg := Target{Forge: forge.Forge{Host: "codeberg.org", Kind: "forgejo", APIBase: srv.URL}, Owner: "o", Repo: "r"}
			src := forge.NewPullLister(forge.ScanOptions{PageSize: 50, MaxPages: 4}, nil)
			err := Show(&bytes.Buffer{}, src, tg, "99", false)
			if err == nil {
				t.Fatal("want an error for a missing issue")
			}
			found := strings.Contains(err.Error(), "issue #99 not found on codeberg.org/o/r")
			if found != c.wantNotFnd {
				t.Fatalf("not-found message = %v, want %v: %v", found, c.wantNotFnd, err)
			}
			var apiErr *forge.APIError
			if !c.wantNotFnd && (!errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound) {
				t.Fatalf("missing repo: want the 404 APIError for the token hint, got %v", err)
			}
		})
	}
}

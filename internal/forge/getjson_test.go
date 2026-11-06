package forge

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// sentinelToken must never appear in an error or a log; the tests assert that.
const sentinelToken = "tok-sentinel-1138"

// fakeAuth answers each request from answer, keyed by the Authorization header and the page.
// It records every Authorization header in arrival order.
type fakeAuth struct {
	answer func(auth, page string) (int, string)
	auth   []string
}

func (f *fakeAuth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a := r.Header.Get("Authorization")
	f.auth = append(f.auth, a)
	status, body := f.answer(a, r.URL.Query().Get("page"))
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// tokenOnly answers 403 to the token and 200 with body to anonymous reads.
func tokenOnly(body string) func(string, string) (int, string) {
	return func(auth, _ string) (int, string) {
		if auth != "" {
			return http.StatusForbidden, `{"message":"forbidden"}`
		}
		return http.StatusOK, body
	}
}

func newAuthServer(t *testing.T, f *fakeAuth) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return srv
}

func forgejoAt(url string) Forge {
	return Forge{Host: "codeberg.org", Kind: "forgejo", APIBase: url}
}

func TestGetJSONRetriesPublicReadAnonymously(t *testing.T) {
	f := &fakeAuth{answer: tokenOnly(`{"number":7,"title":"public"}`)}
	srv := newAuthServer(t, f)
	l := NewPullLister(testScan, func(Forge) string { return sentinelToken })

	is, err := l.Issue(forgejoAt(srv.URL), "ubunatic", "harnez", 7)
	if err != nil {
		t.Fatalf("anonymous retry must succeed, got %v", err)
	}
	if is.Title != "public" {
		t.Fatalf("title = %q, want the anonymous answer", is.Title)
	}
	if len(f.auth) != 2 || f.auth[0] != "token "+sentinelToken || f.auth[1] != "" {
		t.Fatalf("auth sequence = %q, want [token, \"\"]", f.auth)
	}
}

func TestGetJSONBothForbiddenNamesScopeAndHidesToken(t *testing.T) {
	f := &fakeAuth{answer: func(string, string) (int, string) {
		return http.StatusForbidden, `{"message":"forbidden"}`
	}}
	srv := newAuthServer(t, f)
	l := NewPullLister(testScan, func(Forge) string { return sentinelToken })

	_, err := l.Issue(forgejoAt(srv.URL), "ubunatic", "harnez", 7)
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Status != http.StatusForbidden {
		t.Fatalf("want 403 APIError, got %T %v", err, err)
	}
	for _, want := range []string{"lacks a scope", "harnez forge token rotate", "harnez forge token import"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q must contain %q", err.Error(), want)
		}
	}
	if strings.Contains(err.Error(), sentinelToken) {
		t.Fatalf("token leaked into error: %q", err.Error())
	}
	if len(f.auth) != 2 {
		t.Fatalf("want exactly 2 requests (token, then anonymous), got %d", len(f.auth))
	}
}

func TestGetJSONGitHubScopeHintNamesGitHubToken(t *testing.T) {
	f := &fakeAuth{answer: func(string, string) (int, string) {
		return http.StatusForbidden, `{"message":"forbidden"}`
	}}
	srv := newAuthServer(t, f)
	gh := Forge{Host: "github.com", Kind: "github", APIBase: srv.URL, TokenEnv: []string{"GITHUB_TOKEN"}}
	l := NewPullLister(testScan, func(Forge) string { return sentinelToken })

	_, err := l.Issue(gh, "ubunatic", "harnez", 7)
	if err == nil || !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Fatalf("GitHub scope hint must name GITHUB_TOKEN, got %v", err)
	}
	if strings.Contains(err.Error(), "forge token") {
		t.Fatalf("rotate/import are Forgejo-only, got %q", err.Error())
	}
}

func TestGetJSONNoTokenForbiddenIsNotRetried(t *testing.T) {
	f := &fakeAuth{answer: func(string, string) (int, string) {
		return http.StatusForbidden, `{"message":"forbidden"}`
	}}
	srv := newAuthServer(t, f)
	l := NewPullLister(testScan, func(Forge) string { return "" })

	_, err := l.Issue(forgejoAt(srv.URL), "ubunatic", "harnez", 7)
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("want HTTP 403 error, got %v", err)
	}
	if len(f.auth) != 1 {
		t.Fatalf("no token sent: want 1 request, got %d", len(f.auth))
	}
}

func TestGetJSONNotFoundWithTokenIsNotRetried(t *testing.T) {
	f := &fakeAuth{answer: func(string, string) (int, string) {
		return http.StatusNotFound, `{"message":"not found"}`
	}}
	srv := newAuthServer(t, f)
	l := NewPullLister(testScan, func(Forge) string { return sentinelToken })

	_, err := l.Issue(forgejoAt(srv.URL), "ubunatic", "harnez", 7)
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("want HTTP 404 error, got %v", err)
	}
	if len(f.auth) != 1 {
		t.Fatalf("404 must not retry: want 1 request, got %d", len(f.auth))
	}
}

func TestIssuesRetryIsPerPage(t *testing.T) {
	// Page 1 reads with the token; page 2 is 403 with the token and is retried anonymously.
	f := &fakeAuth{answer: func(auth, page string) (int, string) {
		if page == "1" {
			return http.StatusOK, `[{"number":1,"title":"a"},{"number":2,"title":"b"}]`
		}
		if auth != "" {
			return http.StatusForbidden, `{"message":"forbidden"}`
		}
		return http.StatusOK, `[]`
	}}
	srv := newAuthServer(t, f)
	l := NewPullLister(testScan, func(Forge) string { return sentinelToken })

	issues, truncated, err := l.Issues(forgejoAt(srv.URL), "ubunatic", "harnez", "all", testScan)
	if err != nil {
		t.Fatalf("paged read must succeed, got %v", err)
	}
	if truncated || len(issues) != 2 {
		t.Fatalf("issues = %d truncated=%v, want 2 and false", len(issues), truncated)
	}
	if len(f.auth) != 3 || f.auth[2] != "" {
		t.Fatalf("auth sequence = %q, want token, token, anonymous", f.auth)
	}
}

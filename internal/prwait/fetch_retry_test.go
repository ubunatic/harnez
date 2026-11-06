package prwait

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const fetchSentinel = "tok-sentinel-1138"

// A token that lacks a scope gets 403 on a public pull request; pr wait must retry anonymously.
func TestFetchRetriesPublicPullAnonymously(t *testing.T) {
	var auths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a := r.Header.Get("Authorization")
		auths = append(auths, a)
		if a != "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"number":3,"state":"closed","merged":true}`))
	}))
	defer srv.Close()

	f := NewHTTPFetcher(2*time.Second, func(Forge) string { return fetchSentinel })
	tg := Target{Forge: Forge{Host: "codeberg.org", Kind: "forgejo", APIBase: srv.URL}, Owner: "ubunatic", Repo: "harnez", Number: 3}
	info, err := f.Fetch(tg)
	if err != nil {
		t.Fatalf("anonymous retry must succeed, got %v", err)
	}
	if !info.Merged {
		t.Fatalf("merged = false, want the anonymous answer: %+v", info)
	}
	if len(auths) != 2 || auths[1] != "" {
		t.Fatalf("auth sequence = %q, want [token, \"\"]", auths)
	}
}

func TestFetchBothForbiddenNamesScopeAndHidesToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	f := NewHTTPFetcher(2*time.Second, func(Forge) string { return fetchSentinel })
	tg := Target{Forge: Forge{Host: "codeberg.org", Kind: "forgejo", APIBase: srv.URL}, Owner: "ubunatic", Repo: "harnez", Number: 3}
	_, err := f.Fetch(tg)
	if err == nil || !strings.Contains(err.Error(), "lacks a scope") || !strings.Contains(err.Error(), "harnez forge token rotate") {
		t.Fatalf("want scope hint naming rotate, got %v", err)
	}
	if strings.Contains(err.Error(), fetchSentinel) {
		t.Fatalf("token leaked into error: %q", err.Error())
	}
}

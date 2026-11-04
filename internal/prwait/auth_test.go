package prwait

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const secret = "tok-SECRET-123456"

func TestTokenSourceOrder(t *testing.T) {
	env := map[string]string{"CODEBERG_TOKEN": "", "FORGEJO_TOKEN": "fj-token", "GITHUB_TOKEN": "gh-env"}
	getenv := func(k string) string { return env[k] }
	ghCalled := false
	ts := TokenSource{Getenv: getenv, GHToken: func() (string, error) { ghCalled = true; return "gh-cli\n", nil }}

	forgejo := Forge{Kind: "forgejo", TokenEnv: []string{"CODEBERG_TOKEN", "FORGEJO_TOKEN"}}
	if got := ts.For(forgejo); got != "fj-token" {
		t.Errorf("forgejo token = %q, want the first non-empty env var", got)
	}
	if ghCalled {
		t.Error("gh must not be consulted for a Forgejo forge")
	}

	github := Forge{Kind: "github", TokenEnv: []string{"GITHUB_TOKEN"}}
	if got := ts.For(github); got != "gh-env" {
		t.Errorf("github token = %q, want the env var", got)
	}

	env["GITHUB_TOKEN"] = ""
	if got := ts.For(github); got != "gh-cli" {
		t.Errorf("github token = %q, want the gh CLI fallback", got)
	}
}

// The token stored by `harnez forge token create` wins over env vars, for Forgejo only (1118).
func TestTokenSourceStoredFirst(t *testing.T) {
	stored := map[string]string{"codeberg.org": "stored-tok"}
	ts := TokenSource{
		Getenv:  func(k string) string { return "env-" + k },
		GHToken: func() (string, error) { return "", errors.New("unused") },
		Stored:  func(h string) string { return stored[h] },
	}
	cb := Forge{Host: "codeberg.org", Kind: "forgejo", TokenEnv: []string{"CODEBERG_TOKEN"}}
	if got := ts.For(cb); got != "stored-tok" {
		t.Errorf("forgejo token = %q, want the stored token", got)
	}
	delete(stored, "codeberg.org")
	if got := ts.For(cb); got != "env-CODEBERG_TOKEN" {
		t.Errorf("forgejo token = %q, want the env fallback", got)
	}
	stored["github.com"] = "nope"
	if got := ts.For(Forge{Host: "github.com", Kind: "github", TokenEnv: []string{"GITHUB_TOKEN"}}); got != "env-GITHUB_TOKEN" {
		t.Errorf("github token = %q, stored tokens are Forgejo-only", got)
	}
}

func TestTokenSourceNoneIsEmpty(t *testing.T) {
	ts := TokenSource{
		Getenv:  func(string) string { return "" },
		GHToken: func() (string, error) { return "", errors.New("gh: not installed") },
	}
	if got := ts.For(Forge{Kind: "github", TokenEnv: []string{"GITHUB_TOKEN"}}); got != "" {
		t.Errorf("token = %q, want none", got)
	}
}

// TestAuthHeaderAndNoTokenLeak checks each forge sends its own auth scheme, and that the token
// appears in neither the summary line nor any error, even when the request fails.
func TestAuthHeaderAndNoTokenLeak(t *testing.T) {
	cases := []struct {
		kind, wantAuth string
	}{
		{"forgejo", "token " + secret},
		{"github", "Bearer " + secret},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			s := &scripted{steps: []step{{401, `{"message":"no"}`}}}
			srv := httptest.NewServer(s.handler(t))
			t.Cleanup(srv.Close)
			target := Target{Forge: forgeFor(c.kind, srv.URL), Owner: "ubunatic", Repo: "harnez", Number: 78}
			fetcher := NewHTTPFetcher(5*time.Second, func(Forge) string { return secret })
			res := newWaiter(fetcher, &fakeClock{}, &fakeGit{}).Wait(
				Options{Target: target, Poll: time.Second, Timeout: time.Hour, MaxErrors: 3, Remote: "origin", Branch: "main"})

			if len(s.auth) != 1 || s.auth[0] != c.wantAuth {
				t.Fatalf("Authorization = %q, want %q", s.auth, c.wantAuth)
			}
			line := res.Summary(target, Options{Timeout: time.Hour})
			msgs := []string{line}
			if res.Err != nil {
				msgs = append(msgs, res.Err.Error())
			}
			for _, m := range msgs {
				if strings.Contains(m, secret) {
					t.Fatalf("token leaked into output: %q", m)
				}
			}
		})
	}
}

func TestNoTokenMeansNoAuthHeader(t *testing.T) {
	s := &scripted{steps: []step{{200, prJSON("closed", true)}}}
	srv := httptest.NewServer(s.handler(t))
	t.Cleanup(srv.Close)
	target := Target{Forge: forgeFor("forgejo", srv.URL), Owner: "ubunatic", Repo: "harnez", Number: 78}
	fetcher := NewHTTPFetcher(5*time.Second, func(Forge) string { return "" })
	if _, err := fetcher.Fetch(target); err != nil {
		t.Fatal(err)
	}
	if s.auth[0] != "" {
		t.Fatalf("Authorization = %q, want none for a public read", s.auth[0])
	}
}

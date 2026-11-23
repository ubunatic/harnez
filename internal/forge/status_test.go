package forge

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckStatusForgejo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		switch {
		case auth == "token read-secret" && r.URL.Path == "/api/v1/user":
			w.WriteHeader(http.StatusForbidden)
		case auth == "token read-secret" && r.URL.Path == "/api/v1/user/repos":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		case auth == "token bad-write":
			w.WriteHeader(http.StatusUnauthorized)
		case auth == "token env-secret" && r.URL.Path == "/api/v1/user":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"login":"ubunatic"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	fg := Forge{
		Host:     "codeberg.org",
		Kind:     "forgejo",
		APIBase:  srv.URL + "/api/v1",
		TokenEnv: []string{"CODEBERG_TOKEN", "FORGEJO_TOKEN"},
		IssueCLI: []string{"tea", "fj"},
	}

	rep := CheckStatus(srv.Client(), fg, StatusOptions{
		ReadStored: func(string) (string, string, bool) {
			return "read-secret", "harnez-read-codeberg", true
		},
		WriteStored: func(string) (string, string, bool) {
			return "bad-write", "harnez-write-codeberg", true
		},
		Getenv: func(k string) string {
			if k == "CODEBERG_TOKEN" {
				return "env-secret"
			}
			return ""
		},
	})

	if rep.EffectiveRead != "stored read token" || !rep.EffectiveReadOK {
		t.Fatalf("effective read = %q (ok=%v), want stored read token (ok=true)", rep.EffectiveRead, rep.EffectiveReadOK)
	}
	if rep.EffectiveWrite != "stored write token" || rep.EffectiveWriteOK {
		t.Fatalf("effective write = %q (ok=%v), want stored write token (ok=false)", rep.EffectiveWrite, rep.EffectiveWriteOK)
	}

	var buf bytes.Buffer
	FormatStatus(&buf, rep)
	out := buf.String()
	for _, secret := range []string{"read-secret", "bad-write", "env-secret"} {
		if strings.Contains(out, secret) {
			t.Fatalf("output leaked secret %q:\n%s", secret, out)
		}
	}
	for _, want := range []string{
		"stored read token (harnez-read-codeberg)",
		"OK (read:repository OK (no read:user scope)) [effective:read]",
		"stored write token (harnez-write-codeberg)",
		"FAIL (HTTP 401 unauthorized (token rejected or revoked)) [effective:write]",
		"env CODEBERG_TOKEN",
		"OK (authenticated as ubunatic)",
		"effective read:  stored read token (OK)",
		"effective write: stored write token (FAIL)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestCheckStatusForgejoEnvFallbackAndRepoProbe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/repos/ubunatic/harnez/issues" && r.Header.Get("Authorization") == "token env-tok" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	fg := Forge{
		Host:     "codeberg.org",
		Kind:     "forgejo",
		APIBase:  srv.URL + "/api/v1",
		TokenEnv: []string{"CODEBERG_TOKEN", "FORGEJO_TOKEN"},
		IssueCLI: []string{"tea", "fj"},
	}

	rep := CheckStatus(srv.Client(), fg, StatusOptions{
		Repo: "ubunatic/harnez",
		Getenv: func(k string) string {
			if k == "FORGEJO_TOKEN" {
				return "env-tok"
			}
			return ""
		},
	})

	if rep.EffectiveRead != "env FORGEJO_TOKEN" || !rep.EffectiveReadOK {
		t.Fatalf("effective read = %q (ok=%v), want env FORGEJO_TOKEN (ok=true)", rep.EffectiveRead, rep.EffectiveReadOK)
	}
	if rep.EffectiveWrite != "env FORGEJO_TOKEN" || !rep.EffectiveWriteOK {
		t.Fatalf("effective write = %q (ok=%v), want env FORGEJO_TOKEN (ok=true)", rep.EffectiveWrite, rep.EffectiveWriteOK)
	}
}

func TestCheckStatusGitHub(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user" && r.Header.Get("Authorization") == "Bearer gh-tok" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"login":"octocat"}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	fg := Forge{
		Host:     "github.com",
		Kind:     "github",
		APIBase:  srv.URL,
		TokenEnv: []string{"GITHUB_TOKEN"},
		IssueCLI: []string{"gh"},
	}

	rep := CheckStatus(srv.Client(), fg, StatusOptions{
		GHToken: func() (string, error) { return "gh-tok", nil },
	})

	if rep.EffectiveRead != "gh auth token" || !rep.EffectiveReadOK {
		t.Fatalf("effective read = %q (ok=%v), want gh auth token (ok=true)", rep.EffectiveRead, rep.EffectiveReadOK)
	}
	if rep.EffectiveWrite != "gh auth token" || !rep.EffectiveWriteOK {
		t.Fatalf("effective write = %q (ok=%v), want gh auth token (ok=true)", rep.EffectiveWrite, rep.EffectiveWriteOK)
	}
}

package forgetracker

import (
	"net/http"
	"testing"

	"ubunatic.com/harnez/internal/forge"
)

func TestWriteSelection(t *testing.T) {
	client := &http.Client{}
	lookup := func(host string) string {
		if host == "codeberg.org" {
			return "stored-write"
		}
		return ""
	}
	noEnv := func(string) string { return "" }
	target := Target{Forge: forge.Forge{Kind: "forgejo", Host: "codeberg.org"}}
	w, ok := WriteAPI(target, client, lookup, noEnv)
	if !ok || w.Token != "stored-write" || w.Client != client || w.Source != "" {
		t.Fatalf("writer=%+v selected=%v", w, ok)
	}
	target.Forge.Host = "missing"
	if _, ok := WriteAPI(target, client, lookup, noEnv); ok {
		t.Fatal("selected without token")
	}
	target.Forge.Kind = "github"
	target.Forge.Host = "codeberg.org"
	if _, ok := WriteAPI(target, client, lookup, noEnv); ok {
		t.Fatal("selected github")
	}
}

// Issue 1247: without a stored write token, the first set token_env variable is used,
// and the stored token still wins when both exist.
func TestWriteSelectionEnvFallback(t *testing.T) {
	client := &http.Client{}
	none := func(string) string { return "" }
	env := map[string]string{"CODEBERG_TOKEN": " ", "FORGEJO_TOKEN": "env-write"}
	getenv := func(k string) string { return env[k] }
	f := forge.Forge{Kind: "forgejo", Host: "codeberg.org", TokenEnv: []string{"CODEBERG_TOKEN", "FORGEJO_TOKEN"}}

	w, ok := WriteAPI(Target{Forge: f}, client, none, getenv)
	if !ok || w.Token != "env-write" || w.Source != "FORGEJO_TOKEN" || w.Client != client {
		t.Fatalf("env fallback: writer=%+v selected=%v", w, ok)
	}
	if got := WriteEnvName(f, getenv); got != "FORGEJO_TOKEN" {
		t.Fatalf("WriteEnvName = %q, want FORGEJO_TOKEN (blank CODEBERG_TOKEN skipped)", got)
	}
	w, ok = WriteAPI(Target{Forge: f}, client, func(string) string { return "stored-write" }, getenv)
	if !ok || w.Token != "stored-write" || w.Source != "" {
		t.Fatalf("stored must win over env: writer=%+v selected=%v", w, ok)
	}
	if _, ok := WriteAPI(Target{Forge: f}, client, none, none); ok {
		t.Fatal("selected with neither stored nor env token")
	}
	gh := forge.Forge{Kind: "github", Host: "github.com", TokenEnv: []string{"GITHUB_TOKEN"}}
	ghEnv := func(string) string { return "gh-env" }
	if _, ok := WriteAPI(Target{Forge: gh}, client, none, ghEnv); ok || WriteEnvName(gh, ghEnv) != "" {
		t.Fatal("GitHub must not select an env write token")
	}
}

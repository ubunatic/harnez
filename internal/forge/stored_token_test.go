package forge

import (
	"testing"
	"time"

	"ubunatic.com/harnez/internal/forgetoken"
)

// The real entry point reads a token stored by `harnez forge token create` before the env vars
// (issue 1118).
func TestSystemTokenSourceReadsStoredToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	getenv := func(k string) string { return "env-" + k }
	cb := Forge{Host: "codeberg.org", Kind: "forgejo", TokenEnv: []string{"CODEBERG_TOKEN"}}
	if got := SystemTokenSource(getenv).For(cb); got != "env-CODEBERG_TOKEN" {
		t.Fatalf("no stored token: got %q, want the env var", got)
	}
	sp, err := forgetoken.LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := forgetoken.DefaultDir(sp.TokenDir)
	if err != nil {
		t.Fatal(err)
	}
	st := forgetoken.Store{Dir: dir}
	if _, err := st.Save(forgetoken.Meta{Host: "codeberg.org", User: "uwe", Name: "n", Created: time.Now()}, "stored-secret"); err != nil {
		t.Fatal(err)
	}
	if got := SystemTokenSource(getenv).For(cb); got != "stored-secret" {
		t.Fatalf("stored token: got %q", got)
	}
}

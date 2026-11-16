package main

import (
	"bytes"
	"strings"
	"testing"
	"ubunatic.com/harnez/internal/forgetoken"
)

func TestParseImportedToken(t *testing.T) {
	if tok, err := parseImportedToken([]byte("  abc123def\n")); err != nil || tok != "abc123def" {
		t.Fatalf("bare token: %q %v", tok, err)
	}
	for _, in := range []string{"", "\n", `{"user":"u","token":"s3cret"}`, "two words"} {
		_, err := parseImportedToken([]byte(in))
		if err == nil || strings.Contains(err.Error(), "s3cret") {
			t.Fatalf("%q: err = %v", in, err)
		}
	}
}

// import reads a bare token from stdin and stores it; nothing is echoed (issue 1118).
func TestForgeTokenImportFromStdin(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// Hide secret-tool: the real keyring holds the owner's live token under the same attributes.
	t.Setenv("PATH", t.TempDir())
	var out bytes.Buffer
	cmd := newForgeTokenCmd()
	cmd.SetArgs([]string{"import", "--user", "uwe", "--name", "manual", "-"})
	cmd.SetIn(strings.NewReader("tok-SECRET-1\n"))
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "tok-SECRET-1") || !strings.Contains(out.String(), "imported: manual for uwe@codeberg.org") {
		t.Fatalf("out = %q", out.String())
	}
	_, st, err := forgeTokenStore()
	if err != nil {
		t.Fatal(err)
	}
	if st.Keyring != nil || st.Secret("codeberg.org") != "tok-SECRET-1" {
		t.Fatal("token not stored in the temp file store")
	}
}

// rotate and revoke refuse before any prompt when nothing is stored; status never prompts
// (issue 1118). No terminal is needed for any of these.
func TestForgeTokenWiringWithoutStoredToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, args := range [][]string{{"rotate"}, {"revoke"}, {"revoke", "--local-only"}} {
		cmd := newForgeTokenCmd()
		cmd.SetArgs(args)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "no token for codeberg.org is stored") {
			t.Fatalf("%v: err = %v", args, err)
		}
	}
	var out bytes.Buffer
	cmd := newForgeTokenCmd()
	cmd.SetArgs([]string{"status"})
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil || out.String() != "no token stored for codeberg.org\nwrite token: none (harnez forge token create --write)\n" {
		t.Fatalf("status: %v %q", err, out.String())
	}
	cmd = newForgeTokenCmd()
	cmd.SetArgs([]string{"create", "--host", "github.com"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "not a Forgejo host") {
		t.Fatalf("github host: %v", err)
	}
}

func TestWriteTokenCommands(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	var out bytes.Buffer
	cmd := newForgeTokenCmd()
	cmd.SetArgs([]string{"import", "--write", "--user", "u", "-"})
	cmd.SetIn(strings.NewReader("write-secret"))
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "write-secret") {
		t.Fatal("secret exposed")
	}
	if forgetoken.LookupWrite("codeberg.org") != "write-secret" || forgetoken.Lookup("codeberg.org") != "" {
		t.Fatal("wrong store")
	}
	cmd = newForgeTokenCmd()
	cmd.SetArgs([]string{"revoke", "--write", "--local-only"})
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestWriteTokenIsolation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	_, read, err := forgeTokenStoreFor(false)
	if err != nil {
		t.Fatal(err)
	}
	_, write, err := forgeTokenStoreFor(true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := read.Save(forgetoken.Meta{Host: "codeberg.org", User: "u"}, "read-secret"); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := newForgeTokenCmd()
	cmd.SetArgs([]string{"status"})
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil || !strings.Contains(out.String(), "write token: none (harnez forge token create --write)") {
		t.Fatalf("%s: %v", out.String(), err)
	}
	if _, err := write.Save(forgetoken.Meta{Host: "codeberg.org", User: "u"}, "write-secret"); err != nil {
		t.Fatal(err)
	}
	if forgetoken.Lookup("codeberg.org") != "read-secret" || forgetoken.LookupWrite("codeberg.org") != "write-secret" {
		t.Fatal("lookup isolation")
	}
	cmd = newForgeTokenCmd()
	cmd.SetArgs([]string{"revoke", "--write", "--local-only"})
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil || forgetoken.Lookup("codeberg.org") != "read-secret" {
		t.Fatalf("revoke: %v", err)
	}
	cmd = newForgeTokenCmd()
	cmd.SetArgs([]string{"import", "--write", "--user", "u", "-"})
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&out)
	if err := cmd.Execute(); err == nil {
		t.Fatal("empty import accepted")
	}
	if strings.Contains(out.String(), "read-secret") || strings.Contains(out.String(), "write-secret") {
		t.Fatal("secret exposed")
	}
}

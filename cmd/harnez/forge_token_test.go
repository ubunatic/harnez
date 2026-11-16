package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
	"ubunatic.com/harnez/internal/forgetoken"
)

type fakeTokenTTY struct{}

func (fakeTokenTTY) close()                      {}
func (fakeTokenTTY) line(string) (string, error) { return "", nil }
func (fakeTokenTTY) login(_ string, user string) (forgetoken.Login, error) {
	return forgetoken.Login{User: user, Password: "pw"}, nil
}

func TestForgeTokenCommandIsolation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	var posted []struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var body struct {
				Name   string   `json:"name"`
				Scopes []string `json:"scopes"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			posted = append(posted, body)
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"id":%d,"name":%q,"sha1":"created-secret"}`, len(posted), body.Name)
			return
		}
		if r.Method != "DELETE" {
			t.Errorf("unexpected %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	oldTTY, oldClient := forgeTokenOpenTTY, forgeTokenAPIBase
	forgeTokenOpenTTY = func(string) (tokenTTY, error) { return fakeTokenTTY{}, nil }
	forgeTokenAPIBase = func(string) (string, time.Duration, error) { return srv.URL, time.Second, nil }
	t.Cleanup(func() { forgeTokenOpenTTY, forgeTokenAPIBase = oldTTY, oldClient })
	_, read, _ := forgeTokenStoreFor(false)
	_, write, _ := forgeTokenStoreFor(true)
	readMeta := forgetoken.Meta{Host: "codeberg.org", User: "u", Name: "read-old", ID: 77}
	writeMeta := forgetoken.Meta{Host: "codeberg.org", User: "u", Name: "write-old", ID: 78}
	run := func(args ...string) {
		t.Helper()
		cmd := newForgeTokenCmd()
		cmd.SetArgs(args)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := read.Save(readMeta, "read-secret"); err != nil {
		t.Fatal(err)
	}
	run("create", "--write", "--user", "u")
	if len(posted) != 1 || !strings.HasPrefix(posted[0].Name, "harnez-write-") || !reflect.DeepEqual(posted[0].Scopes, []string{"read:repository", "write:issue"}) || write.Secret("codeberg.org") != "created-secret" {
		t.Fatalf("create: %+v", posted)
	}
	check := func(st forgetoken.Store, want forgetoken.Meta, secret string) {
		t.Helper()
		got, ok, err := st.Meta("codeberg.org")
		if err != nil || !ok || !reflect.DeepEqual(got, want) || st.Secret("codeberg.org") != secret {
			t.Fatalf("other store changed: %+v %v", got, err)
		}
	}
	rm, _, _ := read.Meta("codeberg.org")
	run("rotate", "--write")
	check(read, rm, "read-secret")
	run("revoke", "--write")
	check(read, rm, "read-secret")
	if _, err := write.Save(writeMeta, "write-secret"); err != nil {
		t.Fatal(err)
	}
	wm, _, _ := write.Meta("codeberg.org")
	run("rotate")
	check(write, wm, "write-secret")
	run("revoke")
	check(write, wm, "write-secret")
}

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

func TestForgeTokenStatusWriteMetadata(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	_, st, err := forgeTokenStoreFor(true)
	if err != nil {
		t.Fatal(err)
	}
	m := forgetoken.Meta{Host: "codeberg.org", User: "u", Name: "write-name", Scopes: []string{"read:repository", "write:issue"}, Created: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	if _, err := st.Save(m, "super-secret"); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := newForgeTokenCmd()
	cmd.SetArgs([]string{"status"})
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("no token stored for codeberg.org\nwrite token:\nstored: write-name for u@codeberg.org\n  scopes: read:repository, write:issue\n  store:  %s/codeberg.org.token (0600)\n  created: 2026-01-02T03:04:05Z\n", st.Dir)
	if out.String() != want {
		t.Fatalf("got %q want %q", out.String(), want)
	}
}

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

func (fakeTokenTTY) close()                        {}
func (fakeTokenTTY) line(string) (string, error)   { return "", nil }
func (fakeTokenTTY) secret(string) (string, error) { return "", nil }
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

type scriptedTokenTTY struct {
	lines       []string
	secrets     []string
	prompts     []string
	secretCalls int
}

func (s *scriptedTokenTTY) close() {}
func (s *scriptedTokenTTY) line(p string) (string, error) {
	s.prompts = append(s.prompts, p)
	if len(s.lines) == 0 {
		return "", nil
	}
	v := s.lines[0]
	s.lines = s.lines[1:]
	return v, nil
}
func (s *scriptedTokenTTY) secret(p string) (string, error) {
	s.prompts = append(s.prompts, p)
	s.secretCalls++
	if len(s.secrets) == 0 {
		return "", nil
	}
	v := s.secrets[0]
	s.secrets = s.secrets[1:]
	return v, nil
}
func (s *scriptedTokenTTY) login(_ string, user string) (forgetoken.Login, error) {
	return forgetoken.Login{User: user, Password: "pw"}, nil
}

func TestForgeTokenImportWizard(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	var checkedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()
	oldTTY, oldAPI := forgeTokenOpenTTY, forgeTokenAPIBase
	tty := &scriptedTokenTTY{
		lines:   []string{"", "write", "uwe", "wiz-write", "ubunatic/harnez"},
		secrets: []string{"wiz-SECRET-1"},
	}
	forgeTokenOpenTTY = func(string) (tokenTTY, error) { return tty, nil }
	forgeTokenAPIBase = func(string) (string, time.Duration, error) { return srv.URL, time.Second, nil }
	t.Cleanup(func() { forgeTokenOpenTTY, forgeTokenAPIBase = oldTTY, oldAPI })

	var out bytes.Buffer
	cmd := newForgeTokenCmd()
	cmd.SetArgs([]string{"import", "--wizard"})
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "wiz-SECRET-1") {
		t.Fatal("secret exposed in output")
	}
	if !strings.Contains(out.String(), "imported: wiz-write for uwe@codeberg.org") || !strings.Contains(out.String(), "check:  read ubunatic/harnez issues OK") {
		t.Fatalf("out = %q", out.String())
	}
	if checkedPath != "/repos/ubunatic/harnez/issues" {
		t.Fatalf("checkedPath = %q", checkedPath)
	}
	if forgetoken.LookupWrite("codeberg.org") != "wiz-SECRET-1" || forgetoken.Lookup("codeberg.org") != "" {
		t.Fatal("token not stored in write store")
	}

	// Non-wizard without file arg still fails.
	cmd = newForgeTokenCmd()
	cmd.SetArgs([]string{"import"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "accepts 1 arg(s)") {
		t.Fatalf("expected 1 arg error, got %v", err)
	}

	// Wizard with existing token for same kind fails before prompting for user/secret.
	tty2 := &scriptedTokenTTY{lines: []string{"", "write"}}
	forgeTokenOpenTTY = func(string) (tokenTTY, error) { return tty2, nil }
	cmd = newForgeTokenCmd()
	cmd.SetArgs([]string{"import", "--wizard"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "already stored") || !strings.Contains(err.Error(), "revoke --write") {
		t.Fatalf("expected already stored --write error, got %v", err)
	}
	if len(tty2.prompts) != 2 {
		t.Fatalf("expected early exit after 2 prompts, got %v", tty2.prompts)
	}

	// Wizard with stdin '-' and flag defaults reads token from stdin without calling tty.secret.
	tty3 := &scriptedTokenTTY{lines: []string{"", "", "", "", ""}}
	forgeTokenOpenTTY = func(string) (tokenTTY, error) { return tty3, nil }
	var outStdin bytes.Buffer
	cmd = newForgeTokenCmd()
	cmd.SetArgs([]string{"import", "--wizard", "--user", "uwe", "--name", "wiz-read", "-"})
	cmd.SetIn(strings.NewReader("stdin-SECRET-1\n"))
	cmd.SetOut(&outStdin)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if tty3.secretCalls != 0 {
		t.Fatalf("expected tty.secret not to be called with '-', got %d", tty3.secretCalls)
	}
	if strings.Contains(outStdin.String(), "stdin-SECRET-1") || !strings.Contains(outStdin.String(), "imported: wiz-read for uwe@codeberg.org") {
		t.Fatalf("outStdin = %q", outStdin.String())
	}
	if forgetoken.Lookup("codeberg.org") != "stdin-SECRET-1" {
		t.Fatal("stdin token not stored in read store")
	}
}

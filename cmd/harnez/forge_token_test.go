package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

// import reads the token from --from NAME, from a hidden terminal prompt when no source is
// given, and from that prompt for "-" when stdin is a terminal. No source ever reaches the
// output or an error message.
func TestForgeTokenImportSources(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // hide secret-tool: the real keyring holds live tokens
	oldTTY, oldIsTerm := forgeTokenOpenTTY, importStdinIsTerminal
	t.Cleanup(func() { forgeTokenOpenTTY, importStdinIsTerminal = oldTTY, oldIsTerm })
	run := func(t *testing.T, tty *scriptedTokenTTY, stdin string, args ...string) (string, error) {
		t.Helper()
		forgeTokenOpenTTY = func(string) (tokenTTY, error) { return tty, nil }
		var out bytes.Buffer
		cmd := newForgeTokenCmd()
		cmd.SetArgs(append([]string{"import"}, args...))
		cmd.SetIn(strings.NewReader(stdin))
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		err := cmd.Execute()
		return out.String(), err
	}
	stored := func(t *testing.T, write bool) string {
		t.Helper()
		_, st, err := forgeTokenStoreFor(write)
		if err != nil {
			t.Fatal(err)
		}
		return st.Secret("codeberg.org")
	}

	t.Run("from env var into the write store", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("HARNEZ_TEST_IMPORT_TOKEN", " env-SECRET-1\n")
		tty := &scriptedTokenTTY{}
		out, err := run(t, tty, "", "--write", "--user", "u", "--from", "HARNEZ_TEST_IMPORT_TOKEN")
		if err != nil || strings.Contains(out, "env-SECRET-1") || tty.secretCalls != 0 {
			t.Fatalf("err=%v out=%q prompts=%d", err, out, tty.secretCalls)
		}
		if stored(t, true) != "env-SECRET-1" || stored(t, false) != "" {
			t.Fatal("--from --write must fill only the write store")
		}
	})

	t.Run("from env var errors name the variable, never the value", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("HARNEZ_TEST_IMPORT_EMPTY", "  ")
		t.Setenv("HARNEZ_TEST_IMPORT_BAD", "bad SECRET-2")
		for name, want := range map[string]string{
			"HARNEZ_TEST_IMPORT_UNSET": "--from HARNEZ_TEST_IMPORT_UNSET: the variable is not set or empty",
			"HARNEZ_TEST_IMPORT_EMPTY": "--from HARNEZ_TEST_IMPORT_EMPTY: the variable is not set or empty",
			"HARNEZ_TEST_IMPORT_BAD":   "--from HARNEZ_TEST_IMPORT_BAD: expected only the token, as copied from the forge's settings page",
		} {
			_, err := run(t, &scriptedTokenTTY{}, "", "--user", "u", "--from", name)
			if err == nil || err.Error() != want {
				t.Fatalf("%s: got %v, want %q", name, err, want)
			}
		}
		if stored(t, false) != "" {
			t.Fatal("a failed import stored a token")
		}
	})

	t.Run("no source prompts on the terminal without echo", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		tty := &scriptedTokenTTY{secrets: []string{"tty-SECRET-3"}}
		out, err := run(t, tty, "stdin-must-not-be-read", "--user", "u")
		if err != nil || strings.Contains(out, "tty-SECRET-3") || tty.secretCalls != 1 {
			t.Fatalf("err=%v out=%q prompts=%d", err, out, tty.secretCalls)
		}
		if len(tty.prompts) != 1 || !strings.HasPrefix(tty.prompts[0], "Token for u@codeberg.org (scopes: ") || !strings.HasSuffix(tty.prompts[0], "; paste it, not shown): ") {
			t.Fatalf("prompt = %q", tty.prompts)
		}
		if stored(t, false) != "tty-SECRET-3" {
			t.Fatal("prompted token not stored")
		}
	})

	t.Run("dash on a terminal stdin uses the hidden prompt", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		importStdinIsTerminal = func(io.Reader) bool { return true }
		t.Cleanup(func() { importStdinIsTerminal = oldIsTerm })
		tty := &scriptedTokenTTY{secrets: []string{"tty-SECRET-4"}}
		if _, err := run(t, tty, "stdin-must-not-be-read", "--write", "--user", "u", "-"); err != nil || tty.secretCalls != 1 {
			t.Fatalf("err=%v prompts=%d", err, tty.secretCalls)
		}
		if !strings.HasPrefix(tty.prompts[0], "Write token for u@codeberg.org") || stored(t, true) != "tty-SECRET-4" {
			t.Fatalf("prompt=%q stored=%v", tty.prompts, stored(t, true) == "tty-SECRET-4")
		}
	})

	t.Run("flag errors come before any prompt", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
		t.Setenv("HARNEZ_TEST_IMPORT_TOKEN", "env-SECRET-5")
		for _, c := range []struct {
			args []string
			want string
		}{
			{[]string{}, "--user is required: the forge username that owns the token"},
			{[]string{"--user", "u", "--from", "HARNEZ_TEST_IMPORT_TOKEN", "tok.txt"}, "--from and <file|-> are exclusive; give one token source"},
			{[]string{"--user", "u", "--remove-source"}, "--remove-source applies only to a <file>"},
			{[]string{"--user", "u", "--remove-source", "-"}, "--remove-source applies only to a <file>"},
		} {
			tty := &scriptedTokenTTY{secrets: []string{"never"}}
			_, err := run(t, tty, "", c.args...)
			if err == nil || err.Error() != c.want || tty.secretCalls != 0 {
				t.Fatalf("%v: err=%v prompts=%d, want %q before any prompt", c.args, err, tty.secretCalls, c.want)
			}
		}
	})
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

	// Non-wizard without a file arg now prompts for the token (owner request, 2026-11-23),
	// but bare import without --user still fails, before any prompt.
	noPrompt := &scriptedTokenTTY{secrets: []string{"never"}}
	forgeTokenOpenTTY = func(string) (tokenTTY, error) { return noPrompt, nil }
	cmd = newForgeTokenCmd()
	cmd.SetArgs([]string{"import"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || err.Error() != "--user is required: the forge username that owns the token" || noPrompt.secretCalls != 0 {
		t.Fatalf("expected --user error before any prompt, got %v (prompts %d)", err, noPrompt.secretCalls)
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

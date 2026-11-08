package forgetoken

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSpecScopesAreReadOnly(t *testing.T) {
	sp, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(sp.Scopes, ",") != "read:repository,read:issue" {
		t.Fatalf("scopes = %v", sp.Scopes)
	}
	for _, s := range sp.Scopes {
		if !strings.HasPrefix(s, "read:") {
			t.Fatalf("scope %q is not read-only", s)
		}
	}
}

func TestTokenName(t *testing.T) {
	now := time.Date(2026, 11, 4, 15, 30, 0, 0, time.UTC)
	if got := TokenName("harnez-read", "codeberg.org", now); got != "harnez-read-codeberg.org-20261104-1530" {
		t.Fatal(got)
	}
}

// fakeForge records requests and serves the token endpoints for user "uwe" / password "pw".
type fakeForge struct {
	t       *testing.T
	nextID  int64
	live    map[int64]string
	lastOTP string
	usedOTP map[string]bool
	created []map[string]any
	failDel bool
}

func (f *fakeForge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, p, ok := r.BasicAuth()
	if !ok || u != "uwe" || p != "pw" {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"user does not exist"}`))
		return
	}
	f.lastOTP = r.Header.Get("X-Forgejo-OTP")
	// Like Forgejo, reject a replayed TOTP code.
	if f.lastOTP != "" {
		if f.usedOTP[f.lastOTP] {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.usedOTP[f.lastOTP] = true
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/users/uwe/tokens":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.created = append(f.created, body)
		f.nextID++
		f.live[f.nextID] = body["name"].(string)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": f.nextID, "name": body["name"], "sha1": "secret" + string(rune('0'+f.nextID))})
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/users/uwe/tokens/"):
		if f.failDel {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var id int64
		_ = json.Unmarshal([]byte(strings.TrimPrefix(r.URL.Path, "/api/v1/users/uwe/tokens/")), &id)
		delete(f.live, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		f.t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusTeapot)
	}
}

func setup(t *testing.T) (*fakeForge, Client, Store, Spec) {
	t.Helper()
	f := &fakeForge{t: t, live: map[int64]string{}, usedOTP: map[string]bool{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	sp, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	return f, Client{HTTP: NewHTTPClient(5 * time.Second), APIBase: srv.URL + "/api/v1"}, Store{Dir: filepath.Join(t.TempDir(), "tok")}, sp
}

var login = Login{User: "uwe", Password: "pw", OTP: "123456"}

// otpSeq returns fresh 2FA codes 000001, 000002, ... and the login using the first.
func otpSeq() (Login, func() (string, error)) {
	n := 0
	next := func() (string, error) { n++; return fmt.Sprintf("%06d", n), nil }
	first, _ := next()
	return Login{User: "uwe", Password: "pw", OTP: first}, next
}

func TestClientRefusesRedirects(t *testing.T) {
	var hit bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	t.Cleanup(other.Close)
	redir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(redir.Close)
	c := Client{HTTP: NewHTTPClient(5 * time.Second), APIBase: redir.URL}
	if _, err := c.Create(login, "n", []string{"read:issue"}); err == nil || !strings.Contains(err.Error(), "HTTP 307") {
		t.Fatalf("err = %v", err)
	}
	if hit {
		t.Fatal("basic auth followed a redirect")
	}
}

func TestCreateStoresFileTokenWithReadScopes(t *testing.T) {
	f, c, st, sp := setup(t)
	now := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	res, err := Create(sp, st, c, "codeberg.org", login, now)
	if err != nil {
		t.Fatal(err)
	}
	if f.lastOTP != "123456" {
		t.Fatalf("OTP header = %q", f.lastOTP)
	}
	body := f.created[0]
	if body["name"] != "harnez-read-codeberg.org-20261104-0900" {
		t.Fatalf("name = %v", body["name"])
	}
	if scopes, _ := json.Marshal(body["scopes"]); string(scopes) != `["read:repository","read:issue"]` {
		t.Fatalf("scopes = %s", scopes)
	}
	if res.Meta.Store != "file" || st.Secret("codeberg.org") != "secret1" {
		t.Fatalf("meta %+v secret %q", res.Meta, st.Secret("codeberg.org"))
	}
	for _, name := range []string{"codeberg.org.token", "codeberg.org.json"} {
		fi, err := os.Stat(filepath.Join(st.Dir, name))
		if err != nil || fi.Mode().Perm() != 0o600 {
			t.Fatalf("%s: %v mode %v", name, err, fi)
		}
	}
	if fi, _ := os.Stat(st.Dir); fi.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode %v", fi.Mode().Perm())
	}
	meta, _ := os.ReadFile(filepath.Join(st.Dir, "codeberg.org.json"))
	if strings.Contains(string(meta), "secret1") || strings.Contains(string(meta), "pw") {
		t.Fatalf("metadata holds a secret: %s", meta)
	}
	if _, err := Create(sp, st, c, "codeberg.org", login, now); err == nil || !strings.Contains(err.Error(), "rotate") {
		t.Fatalf("second create must point to rotate, got %v", err)
	}
}

func TestWrongPasswordErrorNeverHoldsPassword(t *testing.T) {
	_, c, st, sp := setup(t)
	_, err := Create(sp, st, c, "codeberg.org", Login{User: "uwe", Password: "hunter2-wrong"}, time.Now())
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") || strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("err = %v", err)
	}
	if _, ok, _ := st.Meta("codeberg.org"); ok {
		t.Fatal("nothing may be stored after a failed create")
	}
}

func TestRotateRevokesOldTokenByID(t *testing.T) {
	f, c, st, sp := setup(t)
	t0 := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	if _, err := Create(sp, st, c, "codeberg.org", login, t0); err != nil {
		t.Fatal(err)
	}
	l, next := otpSeq()
	res, err := Rotate(sp, st, c, "codeberg.org", l, t0.Add(time.Hour), next)
	if err != nil || !res.OldRevoked {
		t.Fatalf("rotate: %v %+v", err, res)
	}
	if _, live := f.live[1]; live || len(f.live) != 1 || st.Secret("codeberg.org") != "secret2" {
		t.Fatalf("live %v secret %q", f.live, st.Secret("codeberg.org"))
	}
	// Without a fresh code the replayed one is rejected and the old token stays live.
	l, _ = otpSeq()
	l.OTP = "999999"
	res, err = Rotate(sp, st, c, "codeberg.org", l, t0.Add(90*time.Minute), nil)
	if err != nil || res.OldRevoked {
		t.Fatalf("replayed 2FA code must fail the revoke: %v %+v", err, res)
	}
	f.failDel = true
	l, _ = otpSeq()
	l.OTP, next = "a1", func() (string, error) { return "a2", nil }
	res, err = Rotate(sp, st, c, "codeberg.org", l, t0.Add(2*time.Hour), next)
	if err != nil || res.OldRevoked || res.OldName != "harnez-read-codeberg.org-20261104-1030" {
		t.Fatalf("failed revoke must keep new token and name the old one: %v %+v", err, res)
	}
	if st.Secret("codeberg.org") != "secret4" {
		t.Fatal("new token not stored")
	}
}

func TestRevoke(t *testing.T) {
	f, c, st, sp := setup(t)
	if _, err := Create(sp, st, c, "codeberg.org", login, time.Now()); err != nil {
		t.Fatal(err)
	}
	f.failDel = true
	if _, err := Revoke(st, c, "codeberg.org", Login{User: "uwe", Password: "pw", OTP: "r1"}, false); err == nil {
		t.Fatal("remote failure must fail revoke")
	}
	if st.Secret("codeberg.org") == "" {
		t.Fatal("failed remote revoke must keep the local token")
	}
	f.failDel = false
	if _, err := Revoke(st, c, "codeberg.org", Login{User: "uwe", Password: "pw", OTP: "r2"}, false); err != nil {
		t.Fatal(err)
	}
	if len(f.live) != 0 || st.Secret("codeberg.org") != "" {
		t.Fatalf("live %v", f.live)
	}
	entries, _ := os.ReadDir(st.Dir)
	if len(entries) != 0 {
		t.Fatalf("leftover files %v", entries)
	}
}

func TestImportChecksReadAndRevokesByName(t *testing.T) {
	var gotAuth, gotPath string
	read := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		if r.Header.Get("Authorization") != "token good" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte("[]"))
	}))
	t.Cleanup(read.Close)
	f, c, st, _ := setup(t)
	rc := Client{HTTP: NewHTTPClient(5 * time.Second), APIBase: read.URL + "/api/v1"}
	if _, err := Import(st, rc, "codeberg.org", "uwe", "manual", "bad", "harnez/taia", time.Now()); err == nil || !strings.Contains(err.Error(), "HTTP 404") || strings.Contains(err.Error(), "bad") {
		t.Fatalf("bad token: %v", err)
	}
	if _, ok, _ := st.Meta("codeberg.org"); ok {
		t.Fatal("failed check must store nothing")
	}
	m, err := Import(st, rc, "codeberg.org", "uwe", "manual", "good", "harnez/taia", time.Now())
	if err != nil || gotAuth != "token good" || gotPath != "/api/v1/repos/harnez/taia/issues" || st.Secret("codeberg.org") != "good" || m.ID != 0 {
		t.Fatalf("%v %q %q %+v", err, gotAuth, gotPath, m)
	}
	if _, err := Import(st, rc, "codeberg.org", "uwe", "x", "good", "", time.Now()); err == nil {
		t.Fatal("second import must refuse")
	}
	// Revoke of an imported token deletes it by name.
	f.live[0] = "manual"
	var path string
	c.HTTP.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		path = r.URL.Path
		return http.DefaultTransport.RoundTrip(r)
	})
	f.t = &testing.T{} // the fake does not parse names; only the path matters here
	_, _ = Revoke(st, c, "codeberg.org", Login{User: "uwe", Password: "pw"}, false)
	if path != "/api/v1/users/uwe/tokens/manual" {
		t.Fatalf("revoke path %q", path)
	}
	// Without a name, revoke refuses before any request.
	st2 := Store{Dir: t.TempDir()}
	if _, err := Import(st2, rc, "codeberg.org", "uwe", "", "good", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := Revoke(st2, c, "codeberg.org", Login{User: "uwe", Password: "pw"}, false); err == nil || !strings.Contains(err.Error(), "--local-only") {
		t.Fatalf("nameless revoke: %v", err)
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type fakeKeyring struct {
	m    map[string]string
	fail bool
}

func (k *fakeKeyring) run(stdin string, args ...string) (string, error) {
	if k.fail {
		return "", errors.New("no session bus")
	}
	host := args[len(args)-1]
	switch args[0] {
	case "store":
		k.m[host] = stdin
	case "lookup":
		v, ok := k.m[host]
		if !ok {
			return "", errors.New("exit 1")
		}
		return v, nil
	case "clear":
		delete(k.m, host)
	}
	return "", nil
}

func TestKeyringPreferredAndFallsBackToFile(t *testing.T) {
	_, c, st, sp := setup(t)
	kr := &fakeKeyring{m: map[string]string{}}
	st.Keyring = &Keyring{Service: sp.KeyringService, Run: kr.run}
	res, err := Create(sp, st, c, "codeberg.org", login, time.Now())
	if err != nil || res.Meta.Store != "keyring" || kr.m["codeberg.org"] != "secret1" {
		t.Fatalf("%v %+v %v", err, res.Meta, kr.m)
	}
	if _, err := os.Stat(filepath.Join(st.Dir, "codeberg.org.token")); !os.IsNotExist(err) {
		t.Fatal("keyring store must not leave a token file")
	}
	if st.Secret("codeberg.org") != "secret1" {
		t.Fatal("lookup through keyring")
	}
	kr.fail = true
	l, next := otpSeq()
	res, err = Rotate(sp, st, c, "codeberg.org", l, time.Now().Add(time.Minute), next)
	if err != nil || res.Meta.Store != "file" || st.Secret("codeberg.org") != "secret2" {
		t.Fatalf("fallback: %v %+v", err, res.Meta)
	}
}

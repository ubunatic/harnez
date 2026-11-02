package forge

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

var testScan = ScanOptions{Timeout: 2 * time.Second, PageSize: 2, MaxPages: 4}

// fakeForge serves a Forgejo- or GitHub-shaped pulls API. list maps a page number to the
// JSON items returned for the open list; files maps a PR number to its file names.
type fakeForge struct {
	t     *testing.T
	list  map[int]string
	files map[int]string
	seen  []string // request URLs, for query and header assertions
	auth  []string
}

func (f *fakeForge) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.seen = append(f.seen, r.URL.String())
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			page = atoi(f.t, p)
		}
		if strings.HasSuffix(r.URL.Path, "/files") {
			// A file list fits one page: later pages are empty, as on a real forge.
			body, ok := f.files[pullNumber(r.URL.Path)]
			if !ok {
				http.NotFound(w, r)
				return
			}
			if page > 1 {
				body = "[]"
			}
			_, _ = w.Write([]byte(body))
			return
		}
		body, ok := f.list[page]
		if !ok {
			_, _ = w.Write([]byte("[]"))
			return
		}
		_, _ = w.Write([]byte(body))
	})
}

func pullNumber(path string) int {
	// .../pulls/<n>/files
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "pulls" && i+1 < len(parts) {
			return atoi(nil, parts[i+1])
		}
	}
	return -1
}

func atoi(t *testing.T, s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			if t != nil {
				t.Fatalf("not a number: %q", s)
			}
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func TestOpenPullsForgejoPagesAndFiltersOpenOnly(t *testing.T) {
	f := &fakeForge{t: t,
		list: map[int]string{
			1: `[{"number":7,"state":"open"},{"number":8,"state":"closed"}]`, // full page
			2: `[{"number":9,"state":"open"}]`,                               // short page: done
		},
		files: map[int]string{
			7: `[{"filename":"issues/1099-x.md"}]`,
			9: `[{"filename":"issues/1100-y.md"},{"filename":"README.md"}]`,
		},
	}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	fg := Forge{Host: "codeberg.org", Kind: "forgejo", APIBase: srv.URL + "/api/v1"}

	got, err := NewPullLister(testScan, nil).OpenPulls(fg, "ubunatic", "harnez", testScan)
	if err != nil {
		t.Fatalf("OpenPulls: %v", err)
	}
	want := []OpenPull{
		{Number: 7, Files: []string{"issues/1099-x.md"}},
		{Number: 9, Files: []string{"issues/1100-y.md", "README.md"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("OpenPulls = %+v, want %+v", got, want)
	}
	if len(f.seen) == 0 || !strings.Contains(f.seen[0], "state=open") || !strings.Contains(f.seen[0], "limit=2") {
		t.Errorf("open list request = %v, want state=open and limit=2", f.seen)
	}
	for _, auth := range f.auth {
		if auth != "" {
			t.Errorf("no token configured, but Authorization = %q", auth)
		}
	}
}

func TestOpenPullsGitHubUsesPerPageAndBearerToken(t *testing.T) {
	f := &fakeForge{t: t,
		list:  map[int]string{1: `[{"number":3,"state":"open"}]`},
		files: map[int]string{3: `[{"filename":"issues/1101-z.md"}]`},
	}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	fg := Forge{Host: "github.com", Kind: "github", APIBase: srv.URL, TokenEnv: []string{"GITHUB_TOKEN"}}
	token := func(Forge) string { return "gh-secret" }

	scan := ScanOptions{Timeout: time.Second, PageSize: 50, MaxPages: 4}
	got, err := NewPullLister(scan, token).OpenPulls(fg, "o", "r", scan)
	if err != nil {
		t.Fatalf("OpenPulls: %v", err)
	}
	if len(got) != 1 || got[0].Number != 3 {
		t.Fatalf("OpenPulls = %+v, want PR 3", got)
	}
	if !strings.Contains(f.seen[0], "per_page=50") {
		t.Errorf("GitHub open list = %q, want per_page=50", f.seen[0])
	}
	if f.auth[0] != "Bearer gh-secret" {
		t.Errorf("GitHub Authorization = %q, want Bearer header", f.auth[0])
	}
}

func TestOpenPullsForgejoTokenHeader(t *testing.T) {
	f := &fakeForge{t: t, list: map[int]string{1: `[]`}}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	fg := Forge{Host: "codeberg.org", Kind: "forgejo", APIBase: srv.URL}
	_, err := NewPullLister(testScan, func(Forge) string { return "cb-secret" }).OpenPulls(fg, "o", "r", testScan)
	if err != nil {
		t.Fatalf("OpenPulls: %v", err)
	}
	if f.auth[0] != "token cb-secret" {
		t.Errorf("Forgejo Authorization = %q, want token header", f.auth[0])
	}
}

func TestOpenPullsPageBudgetIsIncomplete(t *testing.T) {
	// Every page is full, so the scan runs out of budget and must say the result is incomplete.
	f := &fakeForge{t: t, list: map[int]string{
		1: `[{"number":1,"state":"open"},{"number":2,"state":"open"}]`,
		2: `[{"number":3,"state":"open"},{"number":4,"state":"open"}]`,
	}}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	fg := Forge{Host: "codeberg.org", Kind: "forgejo", APIBase: srv.URL}
	scan := ScanOptions{Timeout: time.Second, PageSize: 2, MaxPages: 2}
	_, err := NewPullLister(scan, nil).OpenPulls(fg, "o", "r", scan)
	if !errors.Is(err, ErrTooManyPages) {
		t.Fatalf("err = %v, want ErrTooManyPages", err)
	}
}

func TestOpenPullsServerErrorIsTransientAndTokenFree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	fg := Forge{Host: "codeberg.org", Kind: "forgejo", APIBase: srv.URL}
	_, err := NewPullLister(testScan, func(Forge) string { return "very-secret" }).OpenPulls(fg, "o", "r", testScan)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Status != http.StatusServiceUnavailable || !apiErr.Transient {
		t.Errorf("APIError = %+v, want transient 503", apiErr)
	}
	if strings.Contains(err.Error(), "very-secret") {
		t.Errorf("error leaks the token: %v", err)
	}
}

func TestOpenPullsOfflineIsTransient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing listens any more: a connection error
	fg := Forge{Host: "codeberg.org", Kind: "forgejo", APIBase: url}
	_, err := NewPullLister(testScan, nil).OpenPulls(fg, "o", "r", testScan)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !apiErr.Transient || apiErr.Status != 0 {
		t.Fatalf("err = %v, want transient transport APIError", err)
	}
}

func TestOpenPullsDecodesNumbersOnly(t *testing.T) {
	// Guard the wire shape: a PR item without a usable number is skipped, not counted as 0.
	raw, _ := json.Marshal([]pullItem{{Number: 0, State: "open"}, {Number: 5, State: "open"}})
	f := &fakeForge{t: t, list: map[int]string{1: string(raw)}, files: map[int]string{5: `[]`}}
	srv := httptest.NewServer(f.handler())
	defer srv.Close()
	fg := Forge{Host: "codeberg.org", Kind: "forgejo", APIBase: srv.URL}
	got, err := NewPullLister(testScan, nil).OpenPulls(fg, "o", "r", testScan)
	if err != nil || len(got) != 1 || got[0].Number != 5 {
		t.Fatalf("OpenPulls = %+v, %v; want only PR 5", got, err)
	}
}

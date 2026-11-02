package issues

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/forge"
	"ubunatic.com/harnez/internal/prwait"
)

// fakeRemoteForge serves a Forgejo pulls API. Every open-list request returns list, whatever
// its state filter, so a test can also prove that closed items are dropped client-side.
func fakeRemoteForge(t *testing.T, list string, files map[string]string, listCalls *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/files") {
			body, ok := files[r.URL.Path]
			if !ok || r.URL.Query().Get("page") != "1" {
				_, _ = w.Write([]byte("[]"))
				return
			}
			_, _ = w.Write([]byte(body))
			return
		}
		if listCalls != nil {
			listCalls.Add(1)
		}
		if r.URL.Query().Get("page") != "1" {
			_, _ = w.Write([]byte("[]"))
			return
		}
		_, _ = w.Write([]byte(list))
	}))
}

// claimsSpec points the codeberg.org forge at the fake server.
func claimsSpec(srv *httptest.Server) prwait.Spec {
	return prwait.Spec{
		DefaultRemote: "origin",
		Forges:        []prwait.Forge{{Host: "codeberg.org", Kind: "forgejo", APIBase: srv.URL}},
		OpenPRScan:    forge.ScanOptions{Timeout: 2 * time.Second, PageSize: 50, MaxPages: 4},
	}
}

func claimsRepo(t *testing.T, remotes ...[2]string) string {
	t.Helper()
	dir := t.TempDir()
	runTestGit(t, dir, "init", "-q")
	for _, r := range remotes {
		runTestGit(t, dir, "remote", "add", r[0], r[1])
	}
	return dir
}

func TestOpenPRFilesSkipsNumbersOfOpenPulls(t *testing.T) {
	var calls atomic.Int32
	srv := fakeRemoteForge(t,
		`[{"number":12,"state":"open"}]`,
		map[string]string{"/repos/ubunatic/harnez/pulls/12/files": `[{"filename":"issues/1099-x.md"},{"filename":"issues/archive/1050-old.md"},{"filename":"README.md"}]`},
		&calls)
	defer srv.Close()
	dir := claimsRepo(t, [2]string{"origin", "https://codeberg.org/ubunatic/harnez.git"})
	spec := claimsSpec(srv)
	var warn bytes.Buffer

	lister := forge.NewPullLister(spec.OpenPRScan, nil)
	claims := openPRFiles(dir, spec, lister, &warn)

	if warn.Len() != 0 {
		t.Fatalf("unexpected warning: %s", warn.String())
	}
	got := map[string]string{}
	for _, c := range claims {
		got[c.Number] = c.Ref
	}
	if got["1099"] != "origin PR #12" || got["1050"] != "origin PR #12" || len(claims) != 2 {
		t.Fatalf("claims = %+v, want 1099 and 1050 from PR #12 only", claims)
	}

	// The local tree has 1098, the open PR claims 1099: the next free number is 1100.
	issuesDir := filepath.Join(dir, "issues")
	writeTestFile(t, filepath.Join(issuesDir, "1098-local.md"))
	num, _, err := Reserve(issuesDir, ReserveOptions{Title: "New", Elsewhere: claims})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if num != "1100" {
		t.Fatalf("Reserve = %s, want 1100 (1099 is claimed by an open PR)", num)
	}
}

func TestOpenPRFilesIgnoresClosedAndMergedPulls(t *testing.T) {
	// The server wrongly returns a closed and a merged pull; the client must still drop them.
	srv := fakeRemoteForge(t,
		`[{"number":13,"state":"closed"},{"number":14,"state":"closed","merged":true}]`,
		map[string]string{"/repos/ubunatic/harnez/pulls/13/files": `[{"filename":"issues/1099-abandoned.md"}]`,
			"/repos/ubunatic/harnez/pulls/14/files": `[{"filename":"issues/1098-merged.md"}]`},
		nil)
	defer srv.Close()
	dir := claimsRepo(t, [2]string{"origin", "https://codeberg.org/ubunatic/harnez.git"})
	spec := claimsSpec(srv)

	claims := openPRFiles(dir, spec, forge.NewPullLister(spec.OpenPRScan, nil), nil)
	if len(claims) != 0 {
		t.Fatalf("claims = %+v, want none: closed and merged pulls reserve nothing", claims)
	}
}

func TestOpenPRFilesScansEachRepoOnce(t *testing.T) {
	// origin and a mirror point at the same forge repository: one list call, one entry per file.
	var calls atomic.Int32
	srv := fakeRemoteForge(t,
		`[{"number":12,"state":"open"}]`,
		map[string]string{"/repos/ubunatic/harnez/pulls/12/files": `[{"filename":"issues/1099-x.md"}]`},
		&calls)
	defer srv.Close()
	dir := claimsRepo(t,
		[2]string{"origin", "https://codeberg.org/ubunatic/harnez.git"},
		[2]string{"mirror", "https://codeberg.org/ubunatic/harnez"})
	spec := claimsSpec(srv)

	claims := openPRFiles(dir, spec, forge.NewPullLister(spec.OpenPRScan, nil), nil)
	if calls.Load() != 1 {
		t.Errorf("open-list calls = %d, want 1 for one repository", calls.Load())
	}
	if len(claims) != 1 {
		t.Errorf("claims = %+v, want one entry for issues/1099-x.md", claims)
	}
}

func TestOpenPRFilesOfflineWarnsAndContinues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	base := srv.URL
	srv.Close() // connection refused: the forge is unreachable
	dir := claimsRepo(t, [2]string{"origin", "https://codeberg.org/ubunatic/harnez.git"})
	spec := prwait.Spec{
		DefaultRemote: "origin",
		Forges:        []prwait.Forge{{Host: "codeberg.org", Kind: "forgejo", APIBase: base}},
		OpenPRScan:    forge.ScanOptions{Timeout: time.Second, PageSize: 50, MaxPages: 4},
	}
	var warn bytes.Buffer

	claims := openPRFiles(dir, spec, forge.NewPullLister(spec.OpenPRScan, nil), &warn)
	if len(claims) != 0 {
		t.Errorf("claims = %+v, want none when the forge is unreachable", claims)
	}
	if !strings.Contains(warn.String(), "open PRs not checked on origin") {
		t.Errorf("warning = %q, want it to name origin and say open PRs were not checked", warn.String())
	}
}

func TestOpenPRFilesSkipsRemotesOffConfiguredForges(t *testing.T) {
	var calls atomic.Int32
	srv := fakeRemoteForge(t, `[]`, nil, &calls)
	defer srv.Close()
	dir := claimsRepo(t, [2]string{"origin", "https://example.org/o/r.git"})
	spec := claimsSpec(srv)

	if claims := openPRFiles(dir, spec, forge.NewPullLister(spec.OpenPRScan, nil), nil); len(claims) != 0 {
		t.Errorf("claims = %+v, want none for a non-forge remote", claims)
	}
	if calls.Load() != 0 {
		t.Errorf("forge calls = %d, want 0 for a non-forge remote", calls.Load())
	}
}

func TestOpenPRFilesNotAGitDirIsNoOp(t *testing.T) {
	var calls atomic.Int32
	srv := fakeRemoteForge(t, `[]`, nil, &calls)
	defer srv.Close()
	spec := claimsSpec(srv)
	dir := t.TempDir()
	if claims := openPRFiles(dir, spec, forge.NewPullLister(spec.OpenPRScan, nil), nil); len(claims) != 0 || calls.Load() != 0 {
		t.Errorf("non-git dir: claims = %+v, calls = %d; want none", claims, calls.Load())
	}
}

func TestClaimsOfflineMakesNoForgeCall(t *testing.T) {
	// Offline must not load the forge spec or touch the network, even with a forge remote.
	dir := claimsRepo(t, [2]string{"origin", "https://codeberg.org/ubunatic/harnez.git"})
	claims, err := Claims(dir, ClaimOptions{Offline: true})
	if err != nil {
		t.Fatalf("Claims offline: %v", err)
	}
	if len(claims) != 0 {
		t.Errorf("Claims offline = %+v, want none (no remote-tracking refs yet)", claims)
	}
}

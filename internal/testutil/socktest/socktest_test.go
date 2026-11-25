package socktest

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPickRootKeepsShortTmp(t *testing.T) {
	if got := pickRoot("/tmp/taia-1/tmp"); got != "/tmp/taia-1/tmp" {
		t.Fatalf("pickRoot = %q", got)
	}
}

func TestPickRootFallsBackFromLongOrRelativeTmp(t *testing.T) {
	long := "/" + strings.Repeat("d", 80)
	for _, tmp := range []string{long, "rel/tmp", ""} {
		if got := pickRoot(tmp); got != "/tmp" {
			t.Errorf("pickRoot(%q) = %q, want /tmp", tmp, got)
		}
	}
}

// TestSocketIgnoresTestTMPDIR: a fixture that points TMPDIR at a deep dir does
// not lengthen socket paths made after it.
func TestSocketIgnoresTestTMPDIR(t *testing.T) {
	deep := t.TempDir() + "/" + strings.Repeat("x", 60)
	t.Setenv("TMPDIR", deep)
	sock := Socket(t, "a-longish-socket-name.sock")
	if strings.HasPrefix(sock, deep) || len(sock) > MaxPath {
		t.Fatalf("socket %s (%d bytes) is below the test TMPDIR", sock, len(sock))
	}
}

// rawUnixListen matches a unix listener opened without this helper.
var rawUnixListen = regexp.MustCompile(`net\.Listen\("unix|net\.ListenUnix`)

// socketListenerAllowed are the production listeners. Each checks the path
// length against MaxPath before it listens.
var socketListenerAllowed = map[string]bool{
	"internal/server/server.go":        true, // path checked by Config.Socket
	"internal/subagent/interactive.go": true, // path checked by listenControl
}

// TestNoRawUnixListen fails when a Go file opens a unix listener outside this
// helper and the allowed production listeners. Test sockets must use Listen or
// Socket so their paths stay within MaxPath.
func TestNoRawUnixListen(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var violations []string
	err = filepath.WalkDir(repo, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		rel, err := filepath.Rel(repo, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "internal/testutil/socktest/") || socketListenerAllowed[rel] {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if rawUnixListen.Match(src) {
			violations = append(violations, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("raw unix listen outside socktest (use socktest.Listen or socktest.Socket): %s", strings.Join(violations, ", "))
	}
}

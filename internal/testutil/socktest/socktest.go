// Package socktest gives tests unix socket paths that fit the sun_path limit.
//
// A socket path under t.TempDir or os.MkdirTemp("", ...) inherits the length of
// $TMPDIR and of the test name. Inside a taia session, and on macOS, $TMPDIR is
// already deep, so bind fails with "invalid argument". Dir roots every socket
// dir at a base chosen once when the test binary starts, before any t.Setenv.
package socktest

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

// MaxPath is the sun_path limit, 107 bytes, that the production check in
// internal/server and internal/subagent also enforces.
const MaxPath = 107

// budget is the room a socket dir needs below the root: "/s" plus MkdirTemp's
// random suffix, a subdir or two and a socket name.
const budget = 64

var root = pickRoot(os.TempDir())

func pickRoot(tmp string) string {
	if filepath.IsAbs(tmp) && len(tmp)+budget <= MaxPath {
		return tmp
	}
	return "/tmp"
}

// Dir returns a new short dir for sockets, removed when the test ends.
func Dir(t testing.TB) string {
	t.Helper()
	dir, err := os.MkdirTemp(root, "s")
	if err != nil {
		t.Fatalf("socktest: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// Path returns name joined below a new Dir.
func Path(t testing.TB, name string) string {
	t.Helper()
	return filepath.Join(Dir(t), name)
}

// Socket listens on name below a new Dir and returns its path, for tests that
// need a live socket, such as a stand-in agent.
func Socket(t testing.TB, name string) string {
	t.Helper()
	path := Path(t, name)
	if _, err := Listen(t, path); err != nil {
		t.Fatalf("socktest: listen: %v", err)
	}
	return path
}

// Listen is a drop-in for net.Listen("unix", path). It creates the parent dir
// and closes the listener when the test ends, so a test may also close it.
func Listen(t testing.TB, path string) (net.Listener, error) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { _ = l.Close() })
	return l, nil
}

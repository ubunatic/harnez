package xdgpath

import (
	"path/filepath"
	"testing"
)

func TestRelativeXDGPathsUseHomeFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "relative-data")
	t.Setenv("XDG_CACHE_HOME", "relative-cache")
	if got, want := DataHome(), filepath.Join(home, ".local", "share"); got != want {
		t.Errorf("DataHome() = %q, want %q", got, want)
	}
	if got, want := CacheHome(), filepath.Join(home, ".cache"); got != want {
		t.Errorf("CacheHome() = %q, want %q", got, want)
	}
}

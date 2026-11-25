package selfinstall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePromoterRootsAndValidation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	t.Setenv("GOBIN", filepath.Join(home, "custom-bin"))
	t.Setenv("GOPATH", filepath.Join(home, "go"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg-data"))
	binDir, manDir, err := ResolvePromoterRoots("")
	if err != nil {
		t.Fatalf("ResolvePromoterRoots(GOBIN+XDG) err = %v", err)
	}
	if binDir != filepath.Join(home, "custom-bin") || manDir != filepath.Join(home, "xdg-data", "man", "man1") {
		t.Fatalf("got (%q, %q)", binDir, manDir)
	}

	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", filepath.Join(home, "gp1")+string(os.PathListSeparator)+filepath.Join(home, "gp2"))
	t.Setenv("XDG_DATA_HOME", "")
	binDir, manDir, err = ResolvePromoterRoots("")
	if err != nil || binDir != filepath.Join(home, "gp1", "bin") || manDir != filepath.Join(home, ".local", "share", "man", "man1") {
		t.Fatalf("fallback roots = (%q, %q, %v)", binDir, manDir, err)
	}

	t.Setenv("GOBIN", "relative/bin")
	if _, _, err := ResolvePromoterRoots(""); err == nil || !strings.Contains(err.Error(), "clean path") {
		t.Fatalf("relative GOBIN err = %v", err)
	}
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", ":/empty-leading")
	if _, _, err := ResolvePromoterRoots(""); err == nil || !strings.Contains(err.Error(), "empty leading entry") {
		t.Fatalf("empty leading GOPATH err = %v", err)
	}
}

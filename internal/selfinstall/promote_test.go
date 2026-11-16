package selfinstall

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createValidStagedDir(t *testing.T, parent string, manFiles []string) string {
	t.Helper()
	staged := filepath.Join(parent, "staged")
	if err := os.Mkdir(staged, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"bin", filepath.Join("share", "man", "man1")} {
		if err := os.MkdirAll(filepath.Join(staged, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(staged, "bin", "harnez"), []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range manFiles {
		if err := os.WriteFile(filepath.Join(staged, "share", "man", "man1", name), []byte(".TH HARNEZ 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return staged
}

func TestResolvePromoterRootsAndValidation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TAIA_SESSION_DIR", "")

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
	if _, _, err := ResolvePromoterRoots(""); err == nil {
		t.Fatal("expected relative GOBIN error")
	}
}

func TestPromoteStagedAtomicPinnedSuccessAndSecurityRejections(t *testing.T) {
	home := t.TempDir()
	gobin := filepath.Join(home, "gobin")
	xdgData := filepath.Join(home, "data")
	t.Setenv("HOME", home)
	t.Setenv("GOBIN", gobin)
	t.Setenv("XDG_DATA_HOME", xdgData)
	t.Setenv("TAIA_SESSION_DIR", "")

	manifest := []string{"harnez-agent.1", "harnez.1"}
	stageParent := filepath.Join(t.TempDir(), "priv")
	if err := os.Mkdir(stageParent, 0o700); err != nil {
		t.Fatal(err)
	}
	staged := createValidStagedDir(t, stageParent, manifest)

	var out bytes.Buffer
	var refreshed string
	res, err := PromoteStaged(context.Background(), PromoteOptions{
		StagedDir:        staged,
		ExpectedManFiles: manifest,
		RefreshService: func(_ context.Context, binPath string, _ io.Writer) error {
			refreshed = binPath
			return nil
		},
	}, &out)
	if err != nil {
		t.Fatalf("PromoteStaged failed: %v", err)
	}
	wantBin := filepath.Join(gobin, "harnez")
	if res.BinaryPath != wantBin || refreshed != wantBin {
		t.Fatalf("promoted=%q refreshed=%q, want %q", res.BinaryPath, refreshed, wantBin)
	}
	if link, err := os.Readlink(filepath.Join(gobin, "⚙")); err != nil || link != "harnez" {
		t.Fatalf("host-generated alias symlink = (%q, %v), want relative harnez", link, err)
	}
	for _, m := range manifest {
		if _, err := os.Stat(filepath.Join(xdgData, "man", "man1", m)); err != nil {
			t.Fatalf("promoted man page %s missing: %v", m, err)
		}
	}
	assertNoPromoteTempFiles(t, gobin)
	assertNoPromoteTempFiles(t, filepath.Join(xdgData, "man", "man1"))

	// Reject staged bin containing pre-created symlink/alias or extra file.
	badAliasParent := filepath.Join(t.TempDir(), "p1")
	_ = os.Mkdir(badAliasParent, 0o700)
	badAliasStaged := createValidStagedDir(t, badAliasParent, manifest)
	if err := os.Symlink("/etc/passwd", filepath.Join(badAliasStaged, "bin", "⚙")); err != nil {
		t.Fatal(err)
	}
	if _, err := PromoteStaged(context.Background(), PromoteOptions{StagedDir: badAliasStaged, ExpectedManFiles: manifest, NoRefresh: true}, nil); err == nil {
		t.Fatal("expected staged alias entry in bin/ to be rejected")
	}

	// Reject unexpected extra/hidden man page or missing manifest page.
	badManParent := filepath.Join(t.TempDir(), "p2")
	_ = os.Mkdir(badManParent, 0o700)
	badManStaged := createValidStagedDir(t, badManParent, append(manifest, "evil.1"))
	if _, err := PromoteStaged(context.Background(), PromoteOptions{StagedDir: badManStaged, ExpectedManFiles: manifest, NoRefresh: true}, nil); err == nil {
		t.Fatal("expected unexpected extra man page to be rejected")
	}

	// Reject symlinked binary in staged/bin via O_NOFOLLOW.
	symBinParent := filepath.Join(t.TempDir(), "p3")
	_ = os.Mkdir(symBinParent, 0o700)
	symBinStaged := createValidStagedDir(t, symBinParent, manifest)
	_ = os.Remove(filepath.Join(symBinStaged, "bin", "harnez"))
	_ = os.Symlink("/bin/sh", filepath.Join(symBinStaged, "bin", "harnez"))
	if _, err := PromoteStaged(context.Background(), PromoteOptions{StagedDir: symBinStaged, ExpectedManFiles: manifest, NoRefresh: true}, nil); err == nil {
		t.Fatal("expected symlink binary to be rejected")
	}

	// Reject symlink component in destination GOBIN via O_DIRECTORY|O_NOFOLLOW.
	realDst := filepath.Join(t.TempDir(), "real-dst")
	_ = os.Mkdir(realDst, 0o755)
	symGobin := filepath.Join(t.TempDir(), "sym-gobin")
	_ = os.Symlink(realDst, symGobin)
	t.Setenv("GOBIN", symGobin)
	if _, err := PromoteStaged(context.Background(), PromoteOptions{StagedDir: staged, ExpectedManFiles: manifest, NoRefresh: true}, nil); err == nil {
		t.Fatal("expected symlink destination GOBIN to be rejected")
	}
	assertNoPromoteTempFiles(t, realDst)

	// Reject wrong mode on staged dir.
	t.Setenv("GOBIN", gobin)
	badModeStaged := createValidStagedDir(t, filepath.Join(t.TempDir()), manifest)
	_ = os.Chmod(badModeStaged, 0o755)
	if _, err := PromoteStaged(context.Background(), PromoteOptions{StagedDir: badModeStaged, ExpectedManFiles: manifest, NoRefresh: true}, nil); err == nil {
		t.Fatal("expected mode 0755 staged dir to be rejected")
	}

	// Runner mode blocks both PromoteStaged and Run (goself shared install bypass).
	runnerRoot := filepath.Join(t.TempDir(), "runner-sess")
	_ = os.Mkdir(runnerRoot, 0o700)
	t.Setenv("TAIA_SESSION_DIR", runnerRoot)
	if _, err := PromoteStaged(context.Background(), PromoteOptions{StagedDir: staged, ExpectedManFiles: manifest, NoRefresh: true}, nil); err == nil || !strings.Contains(err.Error(), "host-only") {
		t.Fatalf("runner PromoteStaged err = %v, want host-only", err)
	}
	if err := Run(context.Background(), Options{Strategy: "self", Dir: filepath.Join(home, ".local", "bin")}, io.Discard); err == nil || !strings.Contains(err.Error(), "host-only") {
		t.Fatalf("runner selfinstall.Run err = %v, want host-only", err)
	}
}

func assertNoPromoteTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".promote-") {
			t.Fatalf("leftover promote temp file %s in %s", entry.Name(), dir)
		}
	}
}

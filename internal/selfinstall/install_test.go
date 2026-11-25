package selfinstall

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/goself"
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/fsutil"
)

func TestReleaseArchiveMatchesGoReleaser(t *testing.T) {
	cases := []struct {
		p    goself.Platform
		want string
	}{
		{goself.Platform{OS: "linux", Arch: "amd64"}, "harnez-0.5.8-x86_64-linux.tar.gz"},
		{goself.Platform{OS: "linux", Arch: "arm64"}, "harnez-0.5.8-aarch64-linux.tar.gz"},
		{goself.Platform{OS: "darwin", Arch: "arm64"}, "harnez-0.5.8-arm64-darwin.tar.gz"},
	}
	for _, tc := range cases {
		if got := ReleaseArchive("harnez", "0.5.8", tc.p); got != tc.want {
			t.Errorf("ReleaseArchive(%v) = %q, want %q", tc.p, got, tc.want)
		}
	}
}

func TestRunDryRunDoesNotWriteFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bin")
	var out bytes.Buffer
	err := Run(context.Background(), Options{
		Strategy: "self",
		Dir:      dir,
		DryRun:   true,
		InstallMan: func(io.Writer) error {
			t.Fatal("InstallMan must not run during dry run")
			return nil
		},
	}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("dry run created %s", dir)
	}
	for _, want := range []string{"would install harnez locally", "shell PATH", "roff man pages"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("dry run output missing %q:\n%s", want, out.String())
		}
	}
}

func TestRunLocalSelfInstallWiresBinaryShellGearAndMan(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin")
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("# existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	claude.SetCompletionRenderer(func(shell string) ([]byte, error) {
		return []byte("# harnez " + shell + " completion\n"), nil
	})
	t.Cleanup(func() { claude.SetCompletionRenderer(nil) })

	installDir := filepath.Join(home, ".local", "bin")
	wantBin := filepath.Join(installDir, "harnez")
	var manCalled, serviceRefreshed bool
	var out bytes.Buffer
	err := Run(context.Background(), Options{
		Strategy: "self",
		Dir:      installDir,
		Home:     home,
		GoBinDir: filepath.Join(home, "go", "bin"),
		InstallMan: func(w io.Writer) error {
			manCalled = true
			return nil
		},
		RefreshService: func(_ context.Context, binPath string, _ io.Writer) error {
			serviceRefreshed = binPath == wantBin
			return nil
		},
	}, &out)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(wantBin); err != nil {
		t.Fatalf("installed binary missing at %s: %v", wantBin, err)
	}
	gearTarget, err := os.Readlink(filepath.Join(installDir, "⚙"))
	if err != nil || gearTarget != "harnez" {
		t.Fatalf("gear symlink = %q, %v; want %q", gearTarget, err, "harnez")
	}
	bashrc, err := os.ReadFile(filepath.Join(home, ".bashrc"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# harnez:begin path", installDir[len(home)+1:], "# harnez:begin completion", "harnez.bash"} {
		if !strings.Contains(string(bashrc), want) {
			t.Errorf(".bashrc missing %q:\n%s", want, string(bashrc))
		}
	}
	compScript, err := os.ReadFile(filepath.Join(home, ".harnez", "completions", "harnez.bash"))
	if err != nil || !strings.Contains(string(compScript), "# harnez bash completion") {
		t.Fatalf("bash completion script = %q, %v", string(compScript), err)
	}
	if !manCalled {
		t.Error("InstallMan was not called")
	}
	if !serviceRefreshed {
		t.Errorf("RefreshService did not receive %s", wantBin)
	}
	if !strings.Contains(out.String(), "is not on this shell's PATH") {
		t.Errorf("expected PATH hint in output:\n%s", out.String())
	}
}

func TestRunUnknownStrategyErrors(t *testing.T) {
	var out bytes.Buffer
	err := Run(context.Background(), Options{Strategy: "bogus"}, &out)
	if err == nil || !strings.Contains(err.Error(), "unknown strategy") {
		t.Fatalf("error = %v, want unknown strategy", err)
	}
}

func TestShadowWarningNamesTheHarnezThatPATHFindsFirst(t *testing.T) {
	root := t.TempDir()
	// The warning shortens paths under $HOME to ~. Point HOME at the fixture root
	// so the expected text is the same whatever TMPDIR is.
	t.Setenv("HOME", root)
	installDir := filepath.Join(root, "local", "bin")
	otherDir := filepath.Join(root, "go", "bin")
	linkDir := filepath.Join(root, "link")
	for _, dir := range []string{installDir, otherDir, linkDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	binPath := filepath.Join(installDir, "harnez")
	for _, path := range []string{binPath, filepath.Join(otherDir, "harnez")} {
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(binPath, filepath.Join(linkDir, "harnez")); err != nil {
		t.Fatal(err)
	}
	empty := t.TempDir()
	sep := string(os.PathListSeparator)

	for _, tc := range []struct {
		name, path string
		warn       bool
	}{
		{"install dir first", installDir + sep + otherDir, false},
		{"other copy first", otherDir + sep + installDir, true},
		{"symlink to the installed file", linkDir + sep + otherDir, false},
		{"no harnez on PATH", empty, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PATH", tc.path)
			got := shadowWarning(binPath)
			if !tc.warn {
				if got != "" {
					t.Fatalf("shadowWarning = %q, want none", got)
				}
				return
			}
			for _, want := range []string{"warning:", fsutil.ContractHome(filepath.Join(otherDir, "harnez")), fsutil.ContractHome(binPath), fsutil.ContractHome(installDir) + " first"} {
				if !strings.Contains(got, want) {
					t.Errorf("shadowWarning = %q, missing %q", got, want)
				}
			}
		})
	}
}

func TestRunLocalSelfInstallSymlinksGoBinToLocalBin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	installDir := filepath.Join(home, ".local", "bin")
	goBin := filepath.Join(home, "go", "bin")
	if err := os.MkdirAll(goBin, 0o755); err != nil {
		t.Fatal(err)
	}
	staleGoBin := filepath.Join(goBin, "harnez")
	if err := os.WriteFile(staleGoBin, []byte("#!/bin/sh\necho stale\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	sep := string(os.PathListSeparator)
	t.Setenv("PATH", goBin+sep+installDir+sep+"/usr/bin"+sep+"/bin")

	wantBin := filepath.Join(installDir, "harnez")
	opts := Options{
		Strategy:       "self",
		Dir:            installDir,
		Home:           home,
		GoBinDir:       goBin,
		NoShell:        true,
		NoMan:          true,
		RefreshService: func(context.Context, string, io.Writer) error { return nil },
	}
	var out bytes.Buffer
	if err := Run(context.Background(), opts, &out); err != nil {
		t.Fatal(err)
	}

	gotTarget, err := os.Readlink(staleGoBin)
	if err != nil || gotTarget != wantBin {
		t.Fatalf("gobin symlink = (%q, %v), want %q", gotTarget, err, wantBin)
	}
	if strings.Contains(out.String(), "warning:") {
		t.Errorf("unexpected shadow warning after symlinking gobin:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "symlink ") {
		t.Errorf("expected symlink log line:\n%s", out.String())
	}

	// Second run is idempotent and does not re-create the gobin symlink.
	out.Reset()
	if err := Run(context.Background(), opts, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "symlink ") {
		t.Errorf("unexpected symlink rewrite on idempotent run:\n%s", out.String())
	}

	// Missing goBin dir is not created.
	missingGoBin := filepath.Join(home, "absent-go", "bin")
	if link, changed, err := ensureGoBinSymlink(wantBin, missingGoBin); err != nil || changed || link != "" {
		t.Fatalf("missing goBin = (%q, %v, %v), want empty/false/nil", link, changed, err)
	}
	if _, err := os.Stat(missingGoBin); !os.IsNotExist(err) {
		t.Fatalf("ensureGoBinSymlink created missing dir %s", missingGoBin)
	}

	// Same dir is a no-op (must not clobber the installed binary).
	if link, changed, err := ensureGoBinSymlink(wantBin, installDir); err != nil || changed || link != "" {
		t.Fatalf("same dir = (%q, %v, %v), want empty/false/nil", link, changed, err)
	}
	if fi, err := os.Lstat(wantBin); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("installed binary should remain a regular file, mode=%v err=%v", fi.Mode(), err)
	}
}

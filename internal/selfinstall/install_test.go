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

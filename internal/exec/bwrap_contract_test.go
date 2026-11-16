package exec

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnoseBwrapErrorClassifiesPrerequisites(t *testing.T) {
	cases := []struct {
		name     string
		output   string
		confined bool
		want     []string
	}{
		{
			name:     "mount slave host",
			output:   "bwrap: Failed to make / slave: Operation not permitted",
			confined: false,
			want:     []string{"Failed to make / slave", "host kernel/AppArmor"},
		},
		{
			name:     "uid map taia missing allow-bwrap",
			output:   "bwrap: setting up uid map: Permission denied",
			confined: true,
			want:     []string{"setting up uid map", "--allow-bwrap"},
		},
		{
			name:     "namespace creation",
			output:   "bwrap: Creating new namespace failed: Operation not permitted",
			confined: true,
			want:     []string{"namespace creation failed", "--allow-bwrap"},
		},
		{
			name:     "unrelated exit",
			output:   "FAIL: TestSomething",
			confined: false,
			want:     nil,
		},
		{
			name:     "unrelated mid-line mention",
			output:   "FAIL: assertion mentioned bwrap: without startup prefix",
			confined: true,
			want:     nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DiagnoseBwrapError(tc.output, tc.confined)
			if len(tc.want) == 0 && got != "" {
				t.Fatalf("DiagnoseBwrapError = %q, want empty", got)
			}
			for _, sub := range tc.want {
				if !strings.Contains(got, sub) {
					t.Fatalf("DiagnoseBwrapError = %q, missing %q", got, sub)
				}
			}
		})
	}
}

func TestQuota1SandboxBindsSessionDirAfterTmpfsMask(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil || cwd == "/tmp" || strings.HasPrefix(cwd, "/tmp/") {
		t.Fatalf("expected package cwd outside /tmp, got (%q, %v)", cwd, err)
	}
	tmpHostRoot, err := os.MkdirTemp("/tmp", "harnez-bwrap-tmp-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpHostRoot) })
	maskedCanary := filepath.Join(tmpHostRoot, "masked-canary")
	if err := os.WriteFile(maskedCanary, []byte("host"), 0o600); err != nil {
		t.Fatal(err)
	}
	sessDir := filepath.Join(tmpHostRoot, "sess")
	if err := os.Mkdir(sessDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", sessDir)
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		t.Skip("bwrap not installed")
	}
	script := `test ! -e "$1" && printf ok > "$TAIA_SESSION_DIR/probe.txt"`
	_, wrapped, ok, err := quota1SandboxCommand([]string{"sh", "-c", script, "sh", maskedCanary}, cwd)
	if err != nil || !ok {
		t.Fatalf("quota1SandboxCommand = (%v, %v)", ok, err)
	}
	tmpIdx := -1
	sessBindIdx := -1
	for i := 0; i+1 < len(wrapped); i++ {
		if wrapped[i] == "--tmpfs" && wrapped[i+1] == "/tmp" {
			tmpIdx = i
		}
		if wrapped[i] == "--bind" && wrapped[i+1] == sessDir {
			sessBindIdx = i
		}
	}
	if tmpIdx < 0 || sessBindIdx <= tmpIdx {
		t.Fatalf("expected --bind %s after --tmpfs /tmp in %v", sessDir, wrapped)
	}

	if err := ProbeBwrapSupport(context.Background()); err != nil {
		t.Skipf("bwrap unusable in current container/kernel: %v", err)
	}
	cmd := exec.Command(bwrap, wrapped...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bwrap write to bound TAIA_SESSION_DIR under masked /tmp failed: %v (%s)", err, string(out))
	}
	got, err := os.ReadFile(filepath.Join(sessDir, "probe.txt"))
	if err != nil || string(got) != "ok" {
		t.Fatalf("probe.txt = (%q, %v), want ok", string(got), err)
	}
}

func TestQuota1SandboxRejectsMalformedSessionDir(t *testing.T) {
	badDir := filepath.Join(t.TempDir(), "bad-sess")
	if err := os.Mkdir(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TAIA_SESSION_DIR", badDir)
	if _, _, _, err := quota1SandboxCommand([]string{"true"}, t.TempDir()); err == nil {
		t.Fatal("expected mode 0755 TAIA_SESSION_DIR to fail closed before entering bwrap")
	}

	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var errOut bytes.Buffer
	opts := testExecOptions(t)
	opts.DisableSandbox = false
	opts.Quota1Dir = t.TempDir()
	opts.Getenv = func(k string) string {
		if k == quota1Env {
			return "1"
		}
		return os.Getenv(k)
	}
	if code, err := runExecWrapper([]string{"true"}, opts, strings.NewReader(""), ioDiscard{}, &errOut); err == nil || code != 1 || !strings.Contains(err.Error(), "TAIA_SESSION_DIR") {
		t.Fatalf("runExecWrapper with malformed TAIA_SESSION_DIR = (%d, %v), want TAIA_SESSION_DIR fail-closed error", code, err)
	}
}

func TestQuota1SandboxFailsClosedWithoutBwrap(t *testing.T) {
	sessDir := filepath.Join(t.TempDir(), "sess")
	if err := os.Mkdir(sessDir, 0o700); err != nil {
		t.Fatal(err)
	}
	canary := filepath.Join(t.TempDir(), "unsandboxed-ran")
	t.Setenv("TAIA_SESSION_DIR", sessDir)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	opts := testExecOptions(t)
	opts.DisableSandbox = false
	opts.Quota1Dir = t.TempDir()
	opts.Getenv = func(k string) string {
		if k == quota1Env {
			return "1"
		}
		return os.Getenv(k)
	}
	var errOut bytes.Buffer
	code, err := runExecWrapper([]string{"sh", "-c", "touch " + canary}, opts, strings.NewReader(""), ioDiscard{}, &errOut)
	if code != 1 || err == nil || !strings.Contains(err.Error(), "bwrap") {
		t.Fatalf("runExecWrapper without bwrap = (%d, %v), want fail-closed bwrap error", code, err)
	}
	if _, statErr := os.Stat(canary); !os.IsNotExist(statErr) {
		t.Fatalf("unsandboxed command executed despite missing bwrap: stat=%v", statErr)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func TestProbeBwrapFailsClosedWithoutBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if err := ProbeBwrapSupport(context.Background()); err == nil || !strings.Contains(err.Error(), "bwrap") {
		t.Fatalf("ProbeBwrapSupport without bwrap = %v, want explicit error", err)
	}
}

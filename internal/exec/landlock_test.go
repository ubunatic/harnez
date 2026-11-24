package exec

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

const landlockChildEnv = "HARNEZ_TEST_LANDLOCK_CHILD"

// TestLandlockHelperChild is executed inside a Landlock-confined subprocess by
// the Landlock tests below.
func TestLandlockHelperChild(t *testing.T) {
	if os.Getenv(landlockChildEnv) != "1" {
		return
	}

	action := os.Getenv("HARNEZ_TEST_LANDLOCK_ACTION")
	switch action {
	case "allow":
		if v, ok := os.LookupEnv("HARNEZ_INTERNAL_LANDLOCK_WRITE_DIRS"); ok {
			t.Fatalf("HARNEZ_INTERNAL_LANDLOCK_WRITE_DIRS leaked to child: %q", v)
		}
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd: %v", err)
		}
		if err := os.WriteFile(filepath.Join(cwd, "cwd-ok.txt"), []byte("ok\n"), 0o600); err != nil {
			t.Fatalf("write in granted cwd failed: %v", err)
		}
		tmpFile := filepath.Join(os.TempDir(), "tmp-ok.txt")
		if err := os.WriteFile(tmpFile, []byte("ok\n"), 0o600); err != nil {
			t.Fatalf("write in granted TMPDIR failed: %v", err)
		}
		f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			t.Fatalf("open %s failed: %v", os.DevNull, err)
		}
		if _, err := f.WriteString("discard\n"); err != nil {
			_ = f.Close()
			t.Fatalf("write %s failed: %v", os.DevNull, err)
		}
		_ = f.Close()

		abi, err := strconv.Atoi(os.Getenv("HARNEZ_TEST_LANDLOCK_ABI"))
		if err == nil && abi >= 2 {
			src := filepath.Join(cwd, "subA", "move.txt")
			dst := filepath.Join(cwd, "subB", "moved.txt")
			if err := os.MkdirAll(filepath.Dir(src), 0o700); err != nil {
				t.Fatalf("mkdir subA: %v", err)
			}
			if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
				t.Fatalf("mkdir subB: %v", err)
			}
			if err := os.WriteFile(src, []byte("moved\n"), 0o600); err != nil {
				t.Fatalf("write subA file: %v", err)
			}
			if err := os.Rename(src, dst); err != nil {
				t.Fatalf("cross-subdir rename inside granted cwd failed (ABI %d): %v", abi, err)
			}
		}

	case "deny":
		for _, dir := range []string{os.Getenv("HOME"), os.Getenv("HARNEZ_TEST_OUTSIDE_DIR")} {
			if dir == "" {
				t.Fatal("missing target outside dir")
			}
			assertEACCES(t, "write", os.WriteFile(filepath.Join(dir, "new.txt"), []byte("x"), 0o600))
			assertEACCES(t, "truncate", os.Truncate(filepath.Join(dir, "existing.txt"), 0))
			assertEACCES(t, "remove", os.Remove(filepath.Join(dir, "existing.txt")))
			assertEACCES(t, "rename", os.Rename(filepath.Join(dir, "src.txt"), filepath.Join(dir, "dst.txt")))
		}

	case "broker":
		fdNum, err := strconv.Atoi(os.Getenv("HARNEZ_BROKER_FD"))
		if err != nil || fdNum < 3 {
			t.Fatalf("invalid HARNEZ_BROKER_FD=%q: %v", os.Getenv("HARNEZ_BROKER_FD"), err)
		}
		pipeFile := os.NewFile(uintptr(fdNum), "broker")
		if pipeFile == nil {
			t.Fatalf("os.NewFile(%d) returned nil", fdNum)
		}
		if _, err := pipeFile.WriteString("broker-ok\n"); err != nil {
			_ = pipeFile.Close()
			t.Fatalf("write to inherited broker fd %d failed: %v", fdNum, err)
		}
		if err := pipeFile.Close(); err != nil {
			t.Fatalf("close broker fd %d: %v", fdNum, err)
		}

	case "sentinel":
		sentinelPath := os.Getenv("HARNEZ_TEST_SENTINEL")
		_ = os.WriteFile(sentinelPath, []byte("ran\n"), 0o600)
		t.Fatal("sentinel child should never have executed")

	default:
		t.Fatalf("unknown HARNEZ_TEST_LANDLOCK_ACTION %q", action)
	}
}

func assertEACCES(t *testing.T, op string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s outside grant unexpectedly succeeded", op)
	}
	if !errors.Is(err, syscall.EACCES) && !os.IsPermission(err) {
		t.Fatalf("%s outside grant returned %v, want EACCES/permission denied", op, err)
	}
}

func requireLandlockABI(t *testing.T) int {
	t.Helper()
	abi, err := probeLandlockABI()
	if err != nil {
		t.Skipf("skipping Landlock kernel test: %v", err)
	}
	return abi
}

func runLandlockChild(t *testing.T, cwd string, writeDirs []string, extraEnv map[string]string, extraFiles []*os.File) ([]byte, error) {
	t.Helper()
	exePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	dirsJSON, err := json.Marshal(writeDirs)
	if err != nil {
		t.Fatalf("marshal writeDirs: %v", err)
	}
	return runLandlockChildRaw(t, exePath, cwd, string(dirsJSON), extraEnv, extraFiles)
}

func runLandlockChildRaw(t *testing.T, exePath, cwd, rawWriteDirs string, extraEnv map[string]string, extraFiles []*os.File) ([]byte, error) {
	t.Helper()
	cmd := osexec.Command(exePath, "__landlock-exec", "--", exePath, "-test.run=^TestLandlockHelperChild$")
	cmd.Dir = cwd
	cmd.ExtraFiles = extraFiles

	env := make([]string, 0, len(os.Environ())+len(extraEnv)+2)
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if key == "HARNEZ_INTERNAL_LANDLOCK_WRITE_DIRS" || key == landlockChildEnv {
			continue
		}
		if _, overridden := extraEnv[key]; overridden {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, landlockChildEnv+"=1")
	env = append(env, "HARNEZ_INTERNAL_LANDLOCK_WRITE_DIRS="+rawWriteDirs)
	for k, v := range extraEnv {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.Bytes(), err
}

func TestLandlockHelperAllowsGrantedWritesAndDevices(t *testing.T) {
	abi := requireLandlockABI(t)

	cwd := t.TempDir()
	tmpDir := t.TempDir()
	out, err := runLandlockChild(t, cwd, []string{cwd, tmpDir}, map[string]string{
		"TMPDIR":                      tmpDir,
		"HARNEZ_TEST_LANDLOCK_ACTION": "allow",
		"HARNEZ_TEST_LANDLOCK_ABI":    strconv.Itoa(abi),
	}, nil)
	if err != nil {
		t.Fatalf("landlock allow child failed: %v\noutput:\n%s", err, out)
	}

	if _, err := os.Stat(filepath.Join(cwd, "cwd-ok.txt")); err != nil {
		t.Fatalf("expected cwd-ok.txt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "tmp-ok.txt")); err != nil {
		t.Fatalf("expected tmp-ok.txt: %v", err)
	}
	if abi >= 2 {
		if _, err := os.Stat(filepath.Join(cwd, "subB", "moved.txt")); err != nil {
			t.Fatalf("expected cross-subdir renamed file: %v", err)
		}
	}
}

func TestLandlockHelperDeniesOutsideGrants(t *testing.T) {
	requireLandlockABI(t)

	cwd := t.TempDir()
	tmpDir := t.TempDir()
	fakeHome := t.TempDir()
	outsideDir := t.TempDir()

	for _, dir := range []string{fakeHome, outsideDir} {
		if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("keep\n"), 0o600); err != nil {
			t.Fatalf("seed existing.txt: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "src.txt"), []byte("src\n"), 0o600); err != nil {
			t.Fatalf("seed src.txt: %v", err)
		}
	}

	out, err := runLandlockChild(t, cwd, []string{cwd, tmpDir}, map[string]string{
		"HOME":                        fakeHome,
		"TMPDIR":                      tmpDir,
		"HARNEZ_TEST_OUTSIDE_DIR":     outsideDir,
		"HARNEZ_TEST_LANDLOCK_ACTION": "deny",
	}, nil)
	if err != nil {
		t.Fatalf("landlock deny child failed: %v\noutput:\n%s", err, out)
	}
}

func TestLandlockHelperInvalidEnvFailsClosed(t *testing.T) {
	requireLandlockABI(t)

	exePath, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	cases := []struct {
		name string
		raw  string
	}{
		{name: "empty_array", raw: "[]"},
		{name: "malformed_json", raw: "{not-json"},
		{name: "missing_dir", raw: `["/nonexistent/harnez/landlock/dir"]`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cwd := t.TempDir()
			sentinel := filepath.Join(cwd, "ran.txt")
			out, err := runLandlockChildRaw(t, exePath, cwd, tc.raw, map[string]string{
				"HARNEZ_TEST_LANDLOCK_ACTION": "sentinel",
				"HARNEZ_TEST_SENTINEL":        sentinel,
			}, nil)
			if err == nil {
				t.Fatalf("expected exit 126, got nil (output: %s)", out)
			}
			var exitErr *osexec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 126 {
				t.Fatalf("expected exit code 126, got %v\noutput:\n%s", err, out)
			}
			if _, statErr := os.Stat(sentinel); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("target command ran despite invalid Landlock config")
			}
		})
	}
}

func TestLandlockHelperPreservesBrokerFD(t *testing.T) {
	requireLandlockABI(t)

	cwd := t.TempDir()
	tmpDir := t.TempDir()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer func() { _ = r.Close() }()

	exePath, err := os.Executable()
	if err != nil {
		_ = w.Close()
		t.Fatalf("os.Executable: %v", err)
	}
	dirsJSON, err := json.Marshal([]string{cwd, tmpDir})
	if err != nil {
		_ = w.Close()
		t.Fatalf("marshal writeDirs: %v", err)
	}

	cmd := osexec.Command(exePath, "__landlock-exec", "--", exePath, "-test.run=^TestLandlockHelperChild$")
	cmd.Dir = cwd
	cmd.ExtraFiles = []*os.File{w}
	cmd.Env = append(os.Environ(),
		landlockChildEnv+"=1",
		"TMPDIR="+tmpDir,
		"HARNEZ_INTERNAL_LANDLOCK_WRITE_DIRS="+string(dirsJSON),
		"HARNEZ_TEST_LANDLOCK_ACTION=broker",
		"HARNEZ_BROKER_FD=3",
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		_ = w.Close()
		t.Fatalf("cmd.Start: %v", err)
	}
	_ = w.Close()

	got, readErr := io.ReadAll(r)
	waitErr := cmd.Wait()
	if waitErr != nil {
		t.Fatalf("broker child failed: %v\noutput:\n%s", waitErr, out.String())
	}
	if readErr != nil {
		t.Fatalf("read broker pipe: %v", readErr)
	}
	if string(got) != "broker-ok\n" {
		t.Fatalf("broker pipe = %q, want %q", string(got), "broker-ok\n")
	}
}

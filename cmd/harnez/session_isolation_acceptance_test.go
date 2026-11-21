package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	harnezexec "ubunatic.com/harnez/internal/exec"
	"ubunatic.com/harnez/internal/selfinstall"
)

type liveIsolationFixture struct {
	taiaBin     string
	builtBin    string
	home        string
	gobin       string
	agentReg    string
	stateDir    string
	siblingSess string
	workDir     string
	env         []string
}

func TestLiveTaiaLandlockCrossSessionAndBwrapAcceptance(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("live Taia Landlock acceptance requires Linux")
	}
	if testing.Short() {
		t.Skip("skipping live Taia integration test in -short mode")
	}
	taiaBin, err := exec.LookPath("taia")
	if err != nil {
		t.Skip("taia binary not installed in PATH")
	}
	if err := harnezexec.ProbeBwrapSupport(context.Background()); err != nil {
		t.Skipf("bwrap prerequisite unavailable on host: %v", err)
	}

	fx := setupLiveIsolationFixture(t, taiaBin)
	verifyLandlockConfinement(t, fx)
	verifyBwrapGrant(t, fx)
	verifyStagedHostPromotion(t, fx)
}

func setupLiveIsolationFixture(t *testing.T, taiaBin string) liveIsolationFixture {
	t.Helper()
	binDir := t.TempDir()
	builtBin := filepath.Join(binDir, "harnez")
	if out, err := exec.Command("go", "build", "-o", builtBin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build harnez binary for live test: %v\n%s", err, string(out))
	}

	home := t.TempDir()
	gobin := filepath.Join(home, "go", "bin")
	agentReg := filepath.Join(home, ".harnez", "agents")
	stateDir := filepath.Join(home, ".local", "state", "harnez")
	dataDir := filepath.Join(home, ".local", "share", "harnez")
	siblingSess := filepath.Join(t.TempDir(), "sibling-session")
	workDir := filepath.Join(t.TempDir(), "work")
	runDir := filepath.Join(home, "run")
	for _, d := range []string{gobin, agentReg, stateDir, dataDir, siblingSess, workDir, runDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(agentReg, "host-agent.json"), []byte(`{"id":"host-agent","name":"host-agent"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "model-blocks.json"), []byte(`[]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gobin, "sentinel"), []byte("host-sentinel"), 0o644); err != nil {
		t.Fatal(err)
	}
	return liveIsolationFixture{
		taiaBin:     taiaBin,
		builtBin:    builtBin,
		home:        home,
		gobin:       gobin,
		agentReg:    agentReg,
		stateDir:    stateDir,
		siblingSess: siblingSess,
		workDir:     workDir,
		env: []string{
			"PATH=" + binDir + ":" + os.Getenv("PATH"),
			"HOME=" + home,
			"XDG_DATA_HOME=" + filepath.Join(home, ".local", "share"),
			"XDG_STATE_HOME=" + filepath.Join(home, ".local", "state"),
			"XDG_CACHE_HOME=" + filepath.Join(home, ".cache"),
			"XDG_RUNTIME_DIR=" + runDir,
			"GOBIN=" + gobin,
		},
	}
}

func writeMockBashSession(t *testing.T, name, prompt, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	yaml := "steps:\n  - prompt: " + prompt + "\n    calls:\n      - tool: bash\n        args:\n          command: |\n"
	for _, line := range strings.Split(strings.TrimSpace(script), "\n") {
		yaml += "            " + line + "\n"
	}
	yaml += "    reply: done\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func verifyLandlockConfinement(t *testing.T, fx liveIsolationFixture) {
	t.Helper()
	probeScript := `set -euo pipefail
test -n "${TAIA_SESSION_DIR:-}"
test -d "$TAIA_SESSION_DIR"
printf 'ok' > "$TAIA_SESSION_DIR/runner-owned.txt"
"` + fx.builtBin + `" models block codex:luna:low --until 1h --reason live-test >/dev/null
test -f "$TAIA_SESSION_DIR/state/policy/model-blocks.json"
if touch "` + fx.siblingSess + `/pwn" 2>/dev/null; then echo "LEAK_SIBLING"; exit 11; fi
if touch "` + fx.agentReg + `/forged.json" 2>/dev/null; then echo "LEAK_AGENTS"; exit 12; fi
if touch "` + fx.stateDir + `/model-blocks.json" 2>/dev/null; then echo "LEAK_STATE"; exit 13; fi
if touch "` + fx.gobin + `/forged-bin" 2>/dev/null; then echo "LEAK_GOBIN"; exit 14; fi
if "` + fx.builtBin + `" agent stop --name host-agent >/dev/null 2>&1; then echo "LEAK_STOP"; exit 15; fi
if "` + fx.builtBin + `" agent resume --name host-agent hi >/dev/null 2>&1; then echo "LEAK_RESUME"; exit 16; fi
if "` + fx.builtBin + `" usage --offline >/dev/null 2>&1; then echo "LEAK_USAGE"; exit 17; fi
if "` + fx.builtBin + `" install --strategy self >/dev/null 2>&1; then echo "LEAK_INSTALL"; exit 18; fi
if bwrap --ro-bind / / --dev /dev --proc /proc -- true >/dev/null 2>&1; then echo "LEAK_BWRAP"; exit 19; fi
echo "RUNNER_ISOLATION_OK"
`
	mockFile := writeMockBashSession(t, "mock-confined.yaml", "verify confinement", probeScript)
	cmd := exec.Command(fx.taiaBin, "-C", fx.workDir, "--mock-session", mockFile, "--mock-live", "--mock-replay", "--allow-tool", "bash", "--allow-write", fx.workDir)
	cmd.Env = fx.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("live confined acceptance run failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "RUNNER_ISOLATION_OK") {
		t.Fatalf("missing RUNNER_ISOLATION_OK in output:\n%s", string(out))
	}
	if _, err := os.Stat(filepath.Join(fx.siblingSess, "pwn")); !os.IsNotExist(err) {
		t.Fatalf("sibling session mutated: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(fx.stateDir, "model-blocks.json")); err != nil || string(data) != "[]" {
		t.Fatalf("shared model-blocks mutated: %q, %v", string(data), err)
	}
}

func verifyBwrapGrant(t *testing.T, fx liveIsolationFixture) {
	t.Helper()
	hostTmpDir, err := os.MkdirTemp("/tmp", "harnez-acc-tmp-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(hostTmpDir) })
	hostTmpSentinel := filepath.Join(hostTmpDir, "host-tmp-sentinel")
	if err := os.WriteFile(hostTmpSentinel, []byte("host"), 0o644); err != nil {
		t.Fatal(err)
	}
	bwrapScript := `set -euo pipefail
test -e "` + hostTmpSentinel + `"
HARNEZ_QUOTA_1=1 "` + fx.builtBin + `" exec -- sh -c '
  test "${HARNEZ_SANDBOX:-}" = "quota1"
  grep -q " /tmp tmpfs " /proc/self/mountinfo
  test ! -e "` + hostTmpSentinel + `"
  test -d "$TAIA_SESSION_DIR"
  printf bwrap > "$TAIA_SESSION_DIR/bwrap-ok.txt"
  ! touch "$HOME/bwrap-ro-denied" 2>/dev/null
'
grep -qx bwrap "$TAIA_SESSION_DIR/bwrap-ok.txt"
echo "BWRAP_WITH_GRANT_OK"
`
	mockFile := writeMockBashSession(t, "mock-bwrap.yaml", "verify bwrap", bwrapScript)
	cmd := exec.Command(fx.taiaBin, "-C", fx.workDir, "--mock-session", mockFile, "--mock-live", "--mock-replay", "--allow-tool", "bash", "--allow-write", fx.workDir, "--allow-bwrap")
	cmd.Env = fx.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("live --allow-bwrap run failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "BWRAP_WITH_GRANT_OK") {
		t.Fatalf("missing BWRAP_WITH_GRANT_OK in output:\n%s", string(out))
	}
}

func verifyStagedHostPromotion(t *testing.T, fx liveIsolationFixture) {
	t.Helper()
	stageParent := filepath.Join(t.TempDir(), "stage-parent")
	if err := os.Mkdir(stageParent, 0o700); err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(stageParent, "staged-promo")
	stageCmd := exec.Command(fx.builtBin, "install", "--stage", staged)
	stageCmd.Env = append(os.Environ(), fx.env...)
	if stageOut, err := stageCmd.CombinedOutput(); err != nil {
		t.Fatalf("harnez install --stage failed: %v\n%s", err, string(stageOut))
	}
	stagedBinBytes, err := os.ReadFile(filepath.Join(staged, "bin", "harnez"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(stagedBinBytes)
	digestHex := hex.EncodeToString(sum[:])

	t.Setenv("HOME", fx.home)
	t.Setenv("GOBIN", fx.gobin)
	t.Setenv("XDG_DATA_HOME", filepath.Join(fx.home, ".local", "share"))
	t.Setenv("TAIA_SESSION_DIR", "")
	manifest, err := expectedManManifest(newRootCmd())
	if err != nil {
		t.Fatal(err)
	}
	var promoOut bytes.Buffer
	res, err := selfinstall.PromoteStaged(context.Background(), selfinstall.PromoteOptions{
		StagedDir:        staged,
		ExpectedManFiles: manifest,
		ExpectedSHA256:   digestHex,
		NoRefresh:        true,
	}, &promoOut)
	if err != nil {
		t.Fatalf("host PromoteStaged failed: %v", err)
	}
	if _, err := os.Stat(res.BinaryPath); err != nil {
		t.Fatalf("promoted binary missing: %v", err)
	}
	promotedBytes, err := os.ReadFile(res.BinaryPath)
	if err != nil {
		t.Fatal(err)
	}
	promotedSum := sha256.Sum256(promotedBytes)
	if hex.EncodeToString(promotedSum[:]) != digestHex {
		t.Fatalf("promoted binary SHA-256 mismatch")
	}
	if target, err := os.Readlink(filepath.Join(fx.gobin, "⚙")); err != nil || target != "harnez" {
		t.Fatalf("symlink ⚙ = (%q, %v), want harnez", target, err)
	}
	if data, err := os.ReadFile(filepath.Join(fx.gobin, "sentinel")); err != nil || string(data) != "host-sentinel" {
		t.Fatalf("sentinel in GOBIN modified or removed: (%q, %v)", string(data), err)
	}
	manDir := filepath.Join(fx.home, ".local", "share", "man", "man1")
	for _, name := range manifest {
		if _, err := os.Stat(filepath.Join(manDir, name)); err != nil {
			t.Fatalf("promoted man page %s missing: %v", name, err)
		}
	}
}

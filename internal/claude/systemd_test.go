package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	harnez "ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/server"
)

// TestInstallSystemdUnitWritesExpandedExecStart checks that
// installSystemdUnit (issue 082) writes the bundled unit template to
// ~/.config/systemd/user/, with {{HARNEZ_BIN}} substituted for the
// absolute path of the currently running test binary (standing in for the
// harnez binary), and that a second call is idempotent (no reported
// change).
func TestInstallSystemdUnitWritesExpandedExecStart(t *testing.T) {
	home := t.TempDir()
	runtimeDir := filepath.Join(home, "runtime")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("SSH_AUTH_SOCK", filepath.Join(runtimeDir, "keyring", "ssh"))

	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	dst, r, err := installSystemdUnit(harnez.DefaultFS)
	if err != nil {
		t.Fatalf("installSystemdUnit: %v", err)
	}
	if !r.changed {
		t.Fatal("expected first install to report changed=true")
	}

	wantDst := filepath.Join(home, ".config", "systemd", "user", cfg.SystemdUnitName)
	if dst != wantDst {
		t.Errorf("dst = %q, want %q", dst, wantDst)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read installed unit: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "{{") {
		t.Errorf("unit still contains an unexpanded placeholder:\n%s", content)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if !strings.Contains(content, "ExecStart="+exe+" server") {
		t.Errorf("unit ExecStart does not reference the running binary %q:\n%s", exe, content)
	}
	if !strings.Contains(content, "PrivateTmp=yes") {
		t.Errorf("unit must provide a writable private /tmp for SQLite temporary files:\n%s", content)
	}
	if !strings.Contains(content, `Environment=SSH_AUTH_SOCK="%t/keyring/ssh"`) {
		t.Errorf("unit must use %%t for an SSH agent socket under XDG_RUNTIME_DIR:\n%s", content)
	}

	// Second call: no changes should be reported (idempotent).
	_, r2, err := installSystemdUnit(harnez.DefaultFS)
	if err != nil {
		t.Fatalf("installSystemdUnit (second call): %v", err)
	}
	if r2.changed {
		t.Error("expected second install to report changed=false (idempotent)")
	}
}

func TestInstallSystemdUnitOmitsUnsetSSHAuthSock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("SSH_AUTH_SOCK", "")
	dst, _, err := installSystemdUnit(harnez.DefaultFS)
	if err != nil {
		t.Fatalf("installSystemdUnit: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read installed unit: %v", err)
	}
	if strings.Contains(string(data), "SSH_AUTH_SOCK") {
		t.Errorf("unit unexpectedly contains SSH_AUTH_SOCK without an apply-time socket:\n%s", data)
	}
}

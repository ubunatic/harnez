package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	harnez "ubunatic.com/harnez"
)

// TestInstallSystemdUnitWritesExpandedExecStart checks that
// installSystemdUnit (issue 082) writes the bundled unit template to
// ~/.config/systemd/user/, with {{HARNEZ_BIN}} substituted for the
// absolute path of the currently running test binary (standing in for the
// harnez binary), and that a second call is idempotent (no reported
// change).
func TestInstallSystemdUnitWritesExpandedExecStart(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dst, r, err := installSystemdUnit(harnez.DefaultFS)
	if err != nil {
		t.Fatalf("installSystemdUnit: %v", err)
	}
	if !r.changed {
		t.Fatal("expected first install to report changed=true")
	}

	wantDst := filepath.Join(home, ".config", "systemd", "user", systemdUnitName)
	if dst != wantDst {
		t.Errorf("dst = %q, want %q", dst, wantDst)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read installed unit: %v", err)
	}
	content := string(data)

	if strings.Contains(content, systemdUnitExecPlaceholder) {
		t.Errorf("unit still contains unexpanded placeholder %q:\n%s", systemdUnitExecPlaceholder, content)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if !strings.Contains(content, "ExecStart="+exe+" agent-collector") {
		t.Errorf("unit ExecStart does not reference the running binary %q:\n%s", exe, content)
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

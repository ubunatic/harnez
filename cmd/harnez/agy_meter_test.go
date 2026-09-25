package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgyMeterRunUsesAgyLaunchEnv(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(home, "env")
	command := filepath.Join(binDir, "agy")
	script := "#!/bin/sh\nprintf '%s\\n%s\\n' \"$PATH\" \"$ANTIGRAVITY_AGENT\" > \"$AGY_CAPTURE\"\n"
	if err := os.WriteFile(command, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+"/usr/bin:/bin")
	t.Setenv("AGY_CAPTURE", capture)

	cmd := newAgyMeterCmd()
	cmd.SetArgs([]string{"--", command})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agy-meter-run: %v", err)
	}
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatalf("read captured environment: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	wantPath := filepath.Join(home, ".harnez", "shims") + string(os.PathListSeparator) + binDir + string(os.PathListSeparator) + "/usr/bin:/bin"
	if len(lines) != 2 || lines[0] != wantPath || lines[1] != "1" {
		t.Fatalf("agy environment = %q, want PATH %q and ANTIGRAVITY_AGENT=1", data, wantPath)
	}
}

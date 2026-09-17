package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocsCardsCmd_BuildAndCheck(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "vision")

	// 1. Build specific bundle dev-3in1
	cmd := newRootCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"docs", "cards", "build", "--bundle=dev-3in1", "-o", outDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("docs cards build --bundle=dev-3in1 failed: %v", err)
	}

	bundlePath := filepath.Join(outDir, "dev-3in1.png")
	if _, err := os.Stat(bundlePath); err != nil {
		t.Fatalf("bundle image not found at %s: %v", bundlePath, err)
	}

	// 2. Check the generated bundle card
	stdout.Reset()
	stderr.Reset()
	cmd = newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"docs", "cards", "check", bundlePath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("docs cards check failed: %v", err)
	}

	if !strings.Contains(stdout.String(), "passed validation") {
		t.Errorf("expected validation success message, got:\n%s", stdout.String())
	}
}

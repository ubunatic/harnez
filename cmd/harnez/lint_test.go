package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/lint"
)

func TestLintCommandCleanFile(t *testing.T) {
	tmpDir := t.TempDir()
	cleanScript := filepath.Join(tmpDir, "clean.sh")
	err := os.WriteFile(cleanScript, []byte(`#!/usr/bin/env bash
set -euo pipefail
if test -f "$1"
then printf '%s\n' "$1"
fi
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err = runLint(&stdout, &stderr, []string{cleanScript}, lintOptions{Lang: "auto"})
	if err != nil {
		t.Fatalf("expected clean run, got error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "no issues found") {
		t.Errorf("expected clean message, got %q", out)
	}
}

func TestLintCommandViolations(t *testing.T) {
	tmpDir := t.TempDir()
	badScript := filepath.Join(tmpDir, "bad.sh")
	err := os.WriteFile(badScript, []byte(`#!/usr/bin/env bash
if [ -f "$1" ]; then
then
. ~/.bashrc
fi
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err = runLint(&stdout, &stderr, []string{badScript}, lintOptions{Lang: "auto"})
	if err == nil {
		t.Fatal("expected error on lint violations, got nil")
	}

	out := stdout.String()
	if !strings.Contains(out, "bad.sh:2:") || !strings.Contains(out, "bash-no-bracket") {
		t.Errorf("expected bracket violation output, got %q", out)
	}
	if !strings.Contains(out, "Found") || !strings.Contains(out, "lint issue(s)") {
		t.Errorf("expected summary line, got %q", out)
	}
}

func TestLintCommandJSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	badScript := filepath.Join(tmpDir, "bad.sh")
	err := os.WriteFile(badScript, []byte(`#!/usr/bin/env bash
if [ -f "$1" ]; then
    echo "hi"
fi
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err = runLint(&stdout, &stderr, []string{badScript}, lintOptions{Lang: "auto", JSON: true})
	if err == nil {
		t.Fatal("expected error on lint violations, got nil")
	}

	var findings []lint.Finding
	if err := json.Unmarshal(stdout.Bytes(), &findings); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput was: %s", err, stdout.String())
	}

	if len(findings) == 0 {
		t.Fatal("expected JSON findings, got none")
	}
}

func TestLintCommandLangOverride(t *testing.T) {
	tmpDir := t.TempDir()
	txtFile := filepath.Join(tmpDir, "script.txt")
	err := os.WriteFile(txtFile, []byte(`if [ "$a" = "$b" ]; then
echo hi
fi
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err = runLint(&stdout, &stderr, []string{txtFile}, lintOptions{Lang: "bash"})
	if err == nil {
		t.Fatal("expected error with lang=bash override")
	}
	if !strings.Contains(stdout.String(), "bash-no-bracket") {
		t.Errorf("expected bash-no-bracket finding, got: %s", stdout.String())
	}
}

func TestLintCommandMarkdownMarkers(t *testing.T) {
	tmpDir := t.TempDir()
	badMD := filepath.Join(tmpDir, "doc.md")
	err := os.WriteFile(badMD, []byte(`# Title
<!-- harnez:begin Section 1 -->
Content
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err = runLint(&stdout, &stderr, []string{badMD}, lintOptions{Lang: "auto"})
	if err == nil {
		t.Fatal("expected error on unclosed markdown marker")
	}
	if !strings.Contains(stdout.String(), "marker-unclosed") {
		t.Errorf("expected marker-unclosed finding, got: %s", stdout.String())
	}
}

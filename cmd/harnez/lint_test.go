package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
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
	err = runLint(&stdout, &stderr, []string{cleanScript}, lintOptions{Lang: "auto", Check: "error"})
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
	err = runLint(&stdout, &stderr, []string{badScript}, lintOptions{Lang: "auto", Check: "error"})
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
	err = runLint(&stdout, &stderr, []string{badScript}, lintOptions{Lang: "auto", JSON: true, Check: "error"})
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
	err = runLint(&stdout, &stderr, []string{txtFile}, lintOptions{Lang: "bash", Check: "error"})
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
	err = runLint(&stdout, &stderr, []string{badMD}, lintOptions{Lang: "auto", Check: "error"})
	if err == nil {
		t.Fatal("expected error on unclosed markdown marker")
	}
	if !strings.Contains(stdout.String(), "marker-unclosed") {
		t.Errorf("expected marker-unclosed finding, got: %s", stdout.String())
	}
}

// writeLintRepo creates a git repo with one tracked clean script, one tracked
// script with a warning only, one tracked script with an error, and an
// untracked bad script that tracked-file mode must ignore.
func writeLintRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"ok.sh":            "#!/usr/bin/env bash\nset -euo pipefail\necho ok\n",
		"sub/warn.sh":      "#!/usr/bin/env bash\necho no strict mode\n",
		"sub/err.sh":       "#!/usr/bin/env bash\nset -euo pipefail\nif [ -f x ]\nthen echo\nfi\n",
		"notes.txt":        "if [ -f x ]; then echo; fi\n",
		"untracked/bad.sh": "#!/usr/bin/env bash\n. x\n",
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "ok.sh", "sub", "notes.txt"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestLintTrackedFilesMode(t *testing.T) {
	t.Chdir(writeLintRepo(t))

	var stdout, stderr bytes.Buffer
	if err := runLint(&stdout, &stderr, nil, lintOptions{Lang: "auto"}); err != nil {
		t.Fatalf("report-only run must exit 0, got %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"sub/err.sh:3:", "sub/warn.sh:", "across 2 of 3 file(s)", "bash-no-bracket", "bash-strict-mode"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"untracked/bad.sh", "notes.txt"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("output must not lint %s:\n%s", unwanted, out)
		}
	}
}

func TestLintCheckLevels(t *testing.T) {
	t.Chdir(writeLintRepo(t))

	cases := []struct {
		name    string
		args    []string
		check   string
		wantErr bool
	}{
		{"warning-only dir passes error gate", []string{"sub/warn.sh"}, "error", false},
		{"warning-only dir fails warning gate", []string{"sub/warn.sh"}, "warning", true},
		{"error dir fails error gate", []string{"sub"}, "error", true},
		{"clean file passes warning gate", []string{"ok.sh"}, "warning", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := runLint(&stdout, &stderr, tc.args, lintOptions{Lang: "auto", Check: tc.check})
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v\n%s", err, tc.wantErr, stdout.String())
			}
		})
	}

	var stdout bytes.Buffer
	if err := runLint(&stdout, &stdout, nil, lintOptions{Lang: "auto", Check: "fatal"}); err == nil {
		t.Fatal("unknown check level must be rejected")
	}
}

func TestLintDirLangFilter(t *testing.T) {
	t.Chdir(writeLintRepo(t))

	var stdout, stderr bytes.Buffer
	if err := runLint(&stdout, &stderr, []string{"."}, lintOptions{Lang: "markdown"}); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); !strings.Contains(got, "0 file(s) checked") {
		t.Fatalf("--lang markdown over a repo without markdown should check 0 files, got:\n%s", got)
	}
}

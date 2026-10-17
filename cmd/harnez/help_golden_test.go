package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// helpGoldenPath pins `harnez <cmd> --help` for every command, so refactors
// of command constructors (issue 812) cannot silently change the CLI.
// Regenerate with: go test ./cmd/harnez -run TestHelpGolden -update
const helpGoldenPath = "testdata/help.golden"

// commandPaths lists the argument path of every command below root,
// hidden ones included, in tree order.
func commandPaths(cmd *cobra.Command, prefix []string) [][]string {
	var out [][]string
	for _, child := range cmd.Commands() {
		path := append(append([]string(nil), prefix...), child.Name())
		out = append(out, path)
		out = append(out, commandPaths(child, path)...)
	}
	return out
}

func renderAllHelp(t *testing.T) string {
	t.Helper()
	home, _ := os.UserHomeDir()
	var b strings.Builder
	for _, path := range append([][]string{nil}, commandPaths(newRootCmd(), nil)...) {
		root := newRootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(append(append([]string(nil), path...), "--help"))
		if err := root.Execute(); err != nil {
			t.Fatalf("harnez %s --help: %v", strings.Join(path, " "), err)
		}
		text := out.String()
		if home != "" {
			text = strings.ReplaceAll(text, home, "$HOME")
		}
		b.WriteString("=== harnez " + strings.Join(path, " ") + "\n" + text + "\n")
	}
	return b.String()
}

func TestHelpGolden(t *testing.T) {
	got := renderAllHelp(t)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(helpGoldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(helpGoldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(helpGoldenPath)
	if err != nil {
		t.Fatalf("read %s (run with -update to create): %v", helpGoldenPath, err)
	}
	if got != string(want) {
		gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := range min(len(gotLines), len(wantLines)) {
			if gotLines[i] != wantLines[i] {
				t.Fatalf("help output differs from %s at line %d:\n got: %q\nwant: %q", helpGoldenPath, i+1, gotLines[i], wantLines[i])
			}
		}
		t.Fatalf("help output differs from %s in length: got %d lines, want %d", helpGoldenPath, len(gotLines), len(wantLines))
	}
}

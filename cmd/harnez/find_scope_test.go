package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindFinderExitStatus(t *testing.T) {
	cases := []struct {
		name, script, diagnostic string
		code                     int
		matched                  bool
	}{
		{"no matches", "exit 1", "", 0, false},
		{"hard error", "echo broken-finder >&2\nexit 2", "broken-finder", 2, false},
		{"partial", "echo 'scope.go:2:needle'\necho unreadable-path >&2\nexit 2", "partial", 0, true},
	}
	for _, tc := range cases {
		for _, format := range []string{"--text", "--json", "--jsonl"} {
			t.Run(tc.name+format, func(t *testing.T) {
				dir := t.TempDir()
				bin := t.TempDir()
				t.Setenv("HOME", t.TempDir())
				t.Setenv("PATH", bin)
				if err := os.WriteFile(filepath.Join(bin, "rg"), []byte("#!/bin/sh\n"+tc.script+"\n"), 0o700); err != nil {
					t.Fatal(err)
				}
				cmd := newFindCmd()
				var out, errOut bytes.Buffer
				cmd.SetOut(&out)
				cmd.SetErr(&errOut)
				cmd.SetArgs([]string{"code", "needle", "-d", dir, format})
				err := cmd.Execute()
				if code := exitCodeFromRunError(err); code != tc.code {
					t.Fatalf("exit = %d, want %d; err=%v stderr=%s", code, tc.code, err, errOut.String())
				}
				if strings.Contains(out.String(), "scope.go") != tc.matched {
					t.Fatalf("unexpected results: %s", out.String())
				}
				if tc.diagnostic != "" && !strings.Contains(errOut.String(), tc.diagnostic) {
					t.Fatalf("missing diagnostic %q: %s", tc.diagnostic, errOut.String())
				}
				if tc.code == 2 && (!cmd.SilenceErrors || !cmd.SilenceUsage) {
					t.Fatal("finder failure should use existing diagnostics without a usage dump")
				}
			})
		}
	}
}

func TestFindCodeScope(t *testing.T) {
	for _, mode := range []string{"outside", "explicit-dir", "explicit-dot", "nested-worktree", "git-file"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HOME", t.TempDir())
			if mode == "nested-worktree" || mode == "git-file" {
				args := []string{"init"}
				if mode == "git-file" {
					args = append(args, "--separate-git-dir", filepath.Join(t.TempDir(), "git"))
				}
				gitTestRun(t, dir, args...)
			}
			cwd := filepath.Join(dir, "nested")
			if err := os.Mkdir(cwd, 0o755); err != nil {
				t.Fatal(err)
			}
			t.Chdir(cwd)
			// A selected finder with an observable result proves whether the
			// guard allowed search to run, independent of installed tools.
			cfg := "finders:\n  - name: scope\n    scope: code\n    command: [/bin/echo, '[{\"path\":\"scope.go\",\"line\":2}]']\n    timeout: 1s\n"
			if err := os.WriteFile(filepath.Join(cwd, "config.yaml"), []byte(cfg), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"code", "needle", "--via", "scope"}
			if mode == "explicit-dir" {
				args = append(args, "-d", cwd)
			}
			if mode == "explicit-dot" {
				args = append(args, "--dir", ".")
			}
			cmd := newFindCmd()
			var out, errOut bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errOut)
			cmd.SetArgs(args)
			err := cmd.Execute()
			if mode == "outside" {
				if exitCodeFromRunError(err) != 1 || !strings.Contains(err.Error(), "-d/--dir") || out.Len() != 0 {
					t.Fatalf("expected scope hint before search: err=%v out=%s", err, out.String())
				}
			} else if err != nil || !strings.Contains(out.String(), "scope.go:2") {
				t.Fatalf("expected scoped result: err=%v out=%s stderr=%s", err, out.String(), errOut.String())
			}
		})
	}
}

func TestFindMixedFinderFailure(t *testing.T) {
	for _, scope := range []string{"code", "docs"} {
		for _, hasResults := range []bool{false, true} {
			t.Run(scope+"/results="+map[bool]string{false: "false", true: "true"}[hasResults], func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv("HOME", t.TempDir())
				t.Setenv("PATH", t.TempDir())
				output := "[]"
				if hasResults {
					output = `[{"path":"scope.go","line":2}]`
				}
				cfg := "finders:\n  - name: healthy\n    scope: " + scope + "\n    command: [/bin/echo, '" + output + "']\n    timeout: 1s\n  - name: broken\n    scope: " + scope + "\n    command: [/bin/sh, -c, 'echo broken-finder >&2\n\n      exit 2']\n    timeout: 1s\n"
				if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(cfg), 0o600); err != nil {
					t.Fatal(err)
				}
				var out, errOut bytes.Buffer
				err := runFindWithOptions(&out, &errOut, []string{scope, "needle"}, findRunOptions{Dir: dir})
				want := 2
				if hasResults {
					want = 0
				}
				if exitCodeFromRunError(err) != want || !strings.Contains(errOut.String(), "broken-finder") || strings.Contains(out.String(), "scope.go") != hasResults {
					t.Fatalf("exit=%d want=%d out=%s stderr=%s", exitCodeFromRunError(err), want, out.String(), errOut.String())
				}
			})
		}
	}
}

// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// homePathRE matches an absolute home directory such as /home/<user>/ or
// /Users/<user>/. A hook command with one only exists on the machine that
// wrote it; on any other checkout the client fails every tool call.
var homePathRE = regexp.MustCompile(`/(home|Users)/[^/\s"']+/`)

// hookConfigDirs are the agent config trees that clients load from a repo
// checkout (issue 1001).
var hookConfigDirs = []string{".agents", ".claude", ".codex", ".gemini"}

// TestHookConfigsHaveNoAbsoluteHomePaths guards committed hook and agent
// configs against machine-specific paths. The working tree is scanned rather
// than git ls-files because the container test image drops .git.
func TestHookConfigsHaveNoAbsoluteHomePaths(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range hookConfigDirs {
		base := filepath.Join(root, dir)
		if _, err := os.Stat(base); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for lineNumber, line := range strings.Split(string(contents), "\n") {
				if match := homePathRE.FindString(line); match != "" {
					rel, _ := filepath.Rel(root, path)
					t.Errorf("%s:%d: absolute home path %q in committed config; use a repo-relative or installed command (issue 1001): %s",
						rel, lineNumber+1, match, strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

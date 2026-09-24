// Package statusline implements harnez's Claude Code statusLine command.
//
// MVP scope: current working directory only. Claude Code renders a custom
// statusLine on its own row above the built-in footer badges — it cannot
// share a line with the "esc to interrupt" / "? for shortcuts" hint row;
// that is a fixed constraint of the current tool, not a limitation of this
// package. See docs/other/... (issue 095) for the decision record.
package statusline

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// input is the subset of Claude Code's statusLine JSON payload this
// package reads. Other fields (model, cost, context_window, git, ...) are
// deliberately not modeled — MVP is cwd-only.
type input struct {
	CWD       string `json:"cwd"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
		ProjectDir string `json:"project_dir"`
	} `json:"workspace"`
}

// Render reads a Claude Code statusLine JSON payload from r and returns the
// line to print. workspace.current_dir is authoritative, with top-level cwd
// as a fallback. If the effective directory differs from workspace.project_dir,
// both are shown. Paths are tilde-collapsed relative to home when possible.
func Render(r io.Reader, home string) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("statusline: read stdin: %w", err)
	}

	var in input
	if err := json.Unmarshal(data, &in); err != nil {
		return "", fmt.Errorf("statusline: parse stdin JSON: %w", err)
	}

	dir := in.Workspace.CurrentDir
	if dir == "" {
		dir = in.CWD
	}
	dir = collapseHome(dir, home)
	projectDir := collapseHome(in.Workspace.ProjectDir, home)
	if dir != "" && projectDir != "" && dir != projectDir {
		return dir + " → " + projectDir, nil
	}
	return dir, nil
}

// collapseHome replaces a leading home-directory prefix with "~".
func collapseHome(dir, home string) string {
	if dir == "" {
		return ""
	}
	if home == "" {
		return dir
	}
	if dir == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(dir, home+string(os.PathSeparator)); ok {
		return "~" + string(os.PathSeparator) + rest
	}
	return dir
}

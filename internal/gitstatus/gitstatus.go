// Package gitstatus collects and summarizes git working-tree state for
// `harnez repo-status` (issue 154). It parses the machine-readable
// `git status --porcelain=v2 --branch` output rather than scraping the
// human-oriented `git status` text, and classifies the result as either
// "quiet" (nothing worth reporting) or "verbose" (something pending or
// broken) using a fixed, documented threshold -- see Status.Quiet.
package gitstatus

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Status is a parsed summary of `git status --porcelain=v2 --branch`.
type Status struct {
	Branch      string // current branch name, or "" if detached
	Detached    bool
	Upstream    string // e.g. "origin/main"; "" if no upstream configured
	HasUpstream bool
	Ahead       int
	Behind      int

	StagedFiles    []string // index differs from HEAD (X != '.')
	UnstagedFiles  []string // worktree differs from index (Y != '.')
	UntrackedFiles []string
	ConflictFiles  []string // unmerged ('u' porcelain lines)
}

// Quiet reports whether the repo state is boring enough to summarize in one
// short line, per the threshold fixed in issue 154:
//
//   - ANY staged, unstaged, untracked, or conflicted file makes the state
//     non-quiet ("any changed/untracked/staged file at all is not brief").
//   - A detached HEAD is always non-quiet (it's an unusual state worth
//     flagging on its own).
//   - Being behind the upstream by any amount is non-quiet, whether or not
//     the branch is also ahead (a plain "diverged" state as well as a pure
//     "behind" state both mean local history isn't a superset of upstream's,
//     which is worth a pull before doing more work).
//   - Being AHEAD of the upstream, with nothing else pending, is quiet on
//     its own -- this repo (and most solo/no-PR-workflow repos) treats
//     unpushed local commits as the expected normal state, not something
//     to narrate every time.
func (s Status) Quiet() bool {
	if len(s.StagedFiles) > 0 || len(s.UnstagedFiles) > 0 || len(s.UntrackedFiles) > 0 || len(s.ConflictFiles) > 0 {
		return false
	}
	if s.Detached {
		return false
	}
	if s.Behind > 0 {
		return false
	}
	return true
}

// Diverged reports whether the branch is both ahead and behind its upstream.
func (s Status) Diverged() bool {
	return s.Ahead > 0 && s.Behind > 0
}

// Collect runs `git status --porcelain=v2 --branch` in dir and parses the
// result. dir must be inside a git working tree.
func Collect(dir string) (Status, error) {
	cmd := exec.Command("git", "status", "--porcelain=v2", "--branch")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return Status{}, fmt.Errorf("git status: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return Status{}, fmt.Errorf("git status: %w", err)
	}
	return Parse(string(out))
}

// Parse parses the raw output of `git status --porcelain=v2 --branch` into
// a Status. Exported separately from Collect so tests can exercise parsing
// against fixed strings alongside the real-repo tests.
func Parse(raw string) (Status, error) {
	var s Status
	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			head := strings.TrimPrefix(line, "# branch.head ")
			if head == "(detached)" {
				s.Detached = true
			} else {
				s.Branch = head
			}
		case strings.HasPrefix(line, "# branch.upstream "):
			s.Upstream = strings.TrimPrefix(line, "# branch.upstream ")
			s.HasUpstream = true
		case strings.HasPrefix(line, "# branch.ab "):
			ahead, behind, err := parseAB(strings.TrimPrefix(line, "# branch.ab "))
			if err != nil {
				return Status{}, fmt.Errorf("parse branch.ab %q: %w", line, err)
			}
			s.Ahead, s.Behind = ahead, behind
		case strings.HasPrefix(line, "# "):
			// other header line (branch.oid, etc.) -- not needed
		case strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 "):
			path, xy, err := parseChangedEntry(line)
			if err != nil {
				return Status{}, err
			}

			if xy[0] != '.' {
				s.StagedFiles = append(s.StagedFiles, path)
			}
			if xy[1] != '.' {
				s.UnstagedFiles = append(s.UnstagedFiles, path)
			}
		case strings.HasPrefix(line, "u "):
			fields := strings.SplitN(line, " ", 11)
			if len(fields) < 11 {
				return Status{}, fmt.Errorf("malformed unmerged entry: %q", line)
			}
			s.ConflictFiles = append(s.ConflictFiles, fields[10])
		case strings.HasPrefix(line, "? "):
			s.UntrackedFiles = append(s.UntrackedFiles, strings.TrimPrefix(line, "? "))
		case strings.HasPrefix(line, "! "):
			// ignored file, only present with --ignored (not passed here)
		}
	}
	return s, nil
}

// parseAB parses a "+<ahead> -<behind>" branch.ab payload.
func parseAB(payload string) (ahead, behind int, err error) {
	fields := strings.Fields(payload)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("expected 2 fields, got %d", len(fields))
	}
	ahead, err = strconv.Atoi(strings.TrimPrefix(fields[0], "+"))
	if err != nil {
		return 0, 0, err
	}
	behind, err = strconv.Atoi(strings.TrimPrefix(fields[1], "-"))
	if err != nil {
		return 0, 0, err
	}
	// branch.ab reports behind as a negative count; normalize to positive.
	if behind < 0 {
		behind = -behind
	}
	return ahead, behind, nil
}

// parseChangedEntry parses a "1 ..." (ordinary) or "2 ..." (rename/copy)
// porcelain v2 entry, returning its path and 2-char XY status.
//
// "1" entries have 8 fixed fields before the path:
//
//	1 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <path>
//
// "2" entries have a 9th fixed field (the rename/copy score) before the
// path, and the path itself is "<newPath>\t<origPath>":
//
//	2 <XY> <sub> <mH> <mI> <mW> <hH> <hI> <X><score> <newPath>\t<origPath>
func parseChangedEntry(line string) (path string, xy string, err error) {
	fixedFields := 8
	if strings.HasPrefix(line, "2 ") {
		fixedFields = 9
	}
	fields := strings.SplitN(line, " ", fixedFields+1)
	if len(fields) < fixedFields+1 {
		return "", "", fmt.Errorf("malformed changed entry: %q", line)
	}
	if len(fields[1]) != 2 {
		return "", "", fmt.Errorf("malformed XY status in entry: %q", line)
	}
	pathField := fields[fixedFields]
	// rename/copy entries separate the new and original path with a tab;
	// keep only the new (current) path.
	if idx := strings.Index(pathField, "\t"); idx >= 0 {
		pathField = pathField[:idx]
	}
	return pathField, fields[1], nil
}

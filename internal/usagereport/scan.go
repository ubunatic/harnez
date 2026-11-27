package usagereport

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/subagent"
)

// ScanCalls reads every Claude session transcript under projectsDir
// (<cwd dir>/<session>.jsonl and <cwd dir>/<session>/subagents/*.jsonl) and
// returns one call per message id. A message repeated across files (the
// sidechain copy in a nested file) counts once, with its largest output. The
// session is the UUID directory or file name, so nested sidechains belong to
// their parent session. Only file names and the usage fields are used. A
// missing directory means no calls. A transcript last modified before since
// cannot hold a call in range, so it is not read (a zero since reads all). A
// transcript the parser cannot scan is an error.
func ScanCalls(projectsDir string, since time.Time) ([]Call, error) {
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return nil, nil
	}
	best := map[string]Call{}
	var order []string
	if _, err := walkProjectsDir(projectsDir, "", since, nil, best, &order); err != nil {
		return nil, err
	}
	return collectOrderedCalls(best, order), nil
}

// ScanDirs reads Claude session transcripts across multiple Claude config
// directories in list order, scanning <dir>/projects in each. A session ID
// claimed by an earlier directory is skipped in later directories; within the
// winning directory, nested subagent files still merge by MessageID. Missing or
// unreadable directories are skipped with a warning note, not an error.
func ScanDirs(dirs []string, since time.Time) ([]Call, []string, error) {
	home, _ := os.UserHomeDir()
	claimed := map[string]bool{}
	best := map[string]Call{}
	var order []string
	var warnings []string

	for _, dir := range dirs {
		source := contractHome(dir, home)
		projectsDir := filepath.Join(dir, "projects")
		if _, err := os.ReadDir(projectsDir); err != nil {
			warnings = append(warnings, fmt.Sprintf("skipped %s: %v", source, err))
			continue
		}
		seenInDir, err := walkProjectsDir(projectsDir, source, since, claimed, best, &order)
		if err != nil {
			return nil, warnings, err
		}
		for s := range seenInDir {
			claimed[s] = true
		}
	}
	return collectOrderedCalls(best, order), warnings, nil
}

func contractHome(dir, home string) string {
	clean := filepath.Clean(dir)
	if home == "" {
		return clean
	}
	home = filepath.Clean(home)
	if clean == home {
		return "~"
	}
	if strings.HasPrefix(clean, home+string(filepath.Separator)) {
		return "~/" + filepath.ToSlash(clean[len(home)+1:])
	}
	return clean
}

func collectOrderedCalls(best map[string]Call, order []string) []Call {
	calls := make([]Call, 0, len(order))
	for _, id := range order {
		calls = append(calls, best[id])
	}
	return calls
}

func walkProjectsDir(projectsDir, source string, since time.Time, claimed map[string]bool, best map[string]Call, order *[]string) (map[string]bool, error) {
	seenInDir := map[string]bool{}
	err := filepath.WalkDir(projectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		rel, err := filepath.Rel(projectsDir, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) < 2 {
			return nil
		}
		session := parts[1]
		if len(parts) == 2 {
			session = strings.TrimSuffix(parts[1], ".jsonl")
		}
		if claimed != nil && claimed[session] {
			return nil
		}
		if !since.IsZero() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.ModTime().Before(since) {
				return nil
			}
		}
		seenInDir[session] = true
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		parsed, err := subagent.ParseClaudeTranscriptCalls(data)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for _, c := range parsed {
			call := Call{Session: session, Source: source, MessageID: c.MessageID, At: c.At, Usage: c.Usage}
			cur, seen := best[c.MessageID]
			if !seen {
				*order = append(*order, c.MessageID)
				best[c.MessageID] = call
			} else if call.Usage.Output > cur.Usage.Output {
				best[c.MessageID] = call
			}
		}
		return nil
	})
	return seenInDir, err
}

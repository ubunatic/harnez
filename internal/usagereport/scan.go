package usagereport

import (
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
// missing directory means no calls.
func ScanCalls(projectsDir string, since time.Time) ([]Call, error) {
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return nil, nil
	}
	best := map[string]Call{}
	var order []string
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
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, c := range subagent.ParseClaudeTranscriptCalls(data) {
			call := Call{Session: session, MessageID: c.MessageID, At: c.At, Usage: c.Usage}
			cur, seen := best[c.MessageID]
			if !seen {
				order = append(order, c.MessageID)
				best[c.MessageID] = call
			} else if call.Usage.Output > cur.Usage.Output {
				best[c.MessageID] = call
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	calls := make([]Call, 0, len(order))
	for _, id := range order {
		calls = append(calls, best[id])
	}
	return calls, nil
}

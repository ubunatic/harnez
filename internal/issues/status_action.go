package issues

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// StatusResult is the command-independent result of a lifecycle mutation.
type StatusResult struct {
	Number        string `json:"number"`
	File          string `json:"file"`
	OldStatus     string `json:"old_status"`
	NewStatus     string `json:"new_status"`
	ReadmeUpdated bool   `json:"readme_updated"`
	Committed     bool   `json:"committed"`
	CommitSHA     string `json:"commit_sha"`
	Noop          bool   `json:"noop"`
	OldNumber     string `json:"old_number,omitempty"`
}

// StatusBatchOptions supplies index, snapshot, commit, and warning operations
// from the CLI layer without making the issue core depend on those packages.
type StatusBatchOptions struct {
	Dir, CommitMsg                  string
	Check, NoCommit, AmendReason    bool
	Stderr                          io.Writer
	UpdateIndexes                   func(issuesDir string) (bool, error)
	SnapshotIndexes                 func(issuesDir string) (any, error)
	RestoreIndexes                  func(issuesDir string, snapshot any) error
	IndexPaths                      func() []string
	Commit                          func(dir string, paths []string, message string) (string, error)
	PathsChanged                    func(dir string, paths []string) (bool, error)
	DefaultMessage                  func(verb, number, reason string) string
	WarnHeaderMismatch              func(io.Writer, string, string, string)
	WarnMissingImplementationCommit func(io.Writer, string, string)
}

// ApplyStatus changes one ticket status, refreshing the injected issue index
// and committing the affected paths unless check/no-commit was requested.
func ApplyStatus(verb, ticketArg, reason string, opts StatusBatchOptions) (StatusResult, bool, error) {
	reason = strings.TrimSpace(reason)
	newStatus, err := ComposeStatus(verb, reason)
	if err != nil {
		return StatusResult{}, false, err
	}
	issuesDir := filepath.Join(opts.Dir, "issues")
	f, err := FindTicketFile(issuesDir, ticketArg)
	if err != nil {
		return StatusResult{}, false, err
	}
	filePath := filepath.Join(issuesDir, f.RelPath)
	relPath := filepath.ToSlash(filepath.Join("issues", f.RelPath))
	content, err := os.ReadFile(filePath)
	if err != nil {
		return StatusResult{}, false, fmt.Errorf("issues: read %s: %w", filePath, err)
	}
	if opts.WarnHeaderMismatch != nil {
		opts.WarnHeaderMismatch(opts.Stderr, relPath, f.Number, ParseHeaderNumber(string(content)))
	}
	_, oldStatus, hasStatus := ParseIssueFile(string(content))
	if !hasStatus {
		return StatusResult{}, false, fmt.Errorf("issues: %s has no '**Status**:' line to update", relPath)
	}
	if err := ValidateLifecycleTransition(verb, oldStatus); err != nil {
		return StatusResult{}, false, fmt.Errorf("issues: %s: %w", relPath, err)
	}
	newStatus, err = CloseStatus(verb, oldStatus, newStatus, opts.AmendReason)
	if err != nil {
		return StatusResult{}, false, fmt.Errorf("issues: %s: %w", relPath, err)
	}
	result := StatusResult{Number: f.Number, File: relPath, OldStatus: oldStatus, NewStatus: newStatus}
	if (verb == "close" || verb == "done") && oldStatus == newStatus {
		result.Noop = true
		return result, false, nil
	}
	if verb == "close" || verb == "done" {
		if opts.WarnMissingImplementationCommit != nil {
			opts.WarnMissingImplementationCommit(opts.Stderr, opts.Dir, result.Number)
		}
	}
	if oldStatus == newStatus {
		if opts.Check || opts.NoCommit {
			result.Noop = true
			return result, false, nil
		}
		if opts.UpdateIndexes == nil || opts.Commit == nil || opts.PathsChanged == nil {
			return StatusResult{}, false, fmt.Errorf("issues: status index/commit operations are not configured")
		}
		readmeChanged, err := opts.UpdateIndexes(issuesDir)
		if err != nil {
			return StatusResult{}, false, fmt.Errorf("issues: %w", err)
		}
		result.ReadmeUpdated = readmeChanged
		paths := []string{relPath}
		if opts.IndexPaths != nil {
			paths = append(paths, opts.IndexPaths()...)
		}
		changed, err := opts.PathsChanged(opts.Dir, paths)
		if err != nil {
			return StatusResult{}, false, fmt.Errorf("issues: %w", err)
		}
		if !changed {
			result.Noop = true
			return result, false, nil
		}
		msg := opts.CommitMsg
		if msg == "" && opts.DefaultMessage != nil {
			msg = opts.DefaultMessage(verb, f.Number, reason)
		}
		sha, err := opts.Commit(opts.Dir, paths, msg)
		if err != nil {
			return StatusResult{}, false, fmt.Errorf("issues: %w", err)
		}
		result.Committed, result.CommitSHA = true, sha
		return result, false, nil
	}
	newContent, _, err := RewriteStatus(string(content), newStatus)
	if err != nil {
		return StatusResult{}, false, fmt.Errorf("issues: %w", err)
	}
	if opts.Check {
		if opts.SnapshotIndexes == nil || opts.RestoreIndexes == nil || opts.UpdateIndexes == nil {
			return StatusResult{}, false, fmt.Errorf("issues: check index operations are not configured")
		}
		snapshot, err := opts.SnapshotIndexes(issuesDir)
		if err != nil {
			return StatusResult{}, false, fmt.Errorf("issues --check: snapshot indexes: %w", err)
		}
		if err := os.WriteFile(filePath, []byte(newContent), 0o644); err != nil {
			return StatusResult{}, false, fmt.Errorf("issues --check: write %s: %w", filePath, err)
		}
		readmeChanged, updateErr := opts.UpdateIndexes(issuesDir)
		if err := os.WriteFile(filePath, content, 0o644); err != nil {
			return StatusResult{}, false, fmt.Errorf("issues --check: restore %s: %w", filePath, err)
		}
		if err := opts.RestoreIndexes(issuesDir, snapshot); err != nil {
			return StatusResult{}, false, fmt.Errorf("issues --check: restore indexes: %w", err)
		}
		if updateErr != nil {
			return StatusResult{}, false, fmt.Errorf("issues --check: %w", updateErr)
		}
		result.ReadmeUpdated = readmeChanged
		return result, true, nil
	}
	if opts.UpdateIndexes == nil {
		return StatusResult{}, false, fmt.Errorf("issues: status index operation is not configured")
	}
	if err := os.WriteFile(filePath, []byte(newContent), 0o644); err != nil {
		return StatusResult{}, false, fmt.Errorf("issues: write %s: %w", filePath, err)
	}
	readmeChanged, err := opts.UpdateIndexes(issuesDir)
	if err != nil {
		return StatusResult{}, false, fmt.Errorf("issues: %w", err)
	}
	result.ReadmeUpdated = readmeChanged
	if !opts.NoCommit {
		if opts.Commit == nil {
			return StatusResult{}, false, fmt.Errorf("issues: status commit operation is not configured")
		}
		msg := opts.CommitMsg
		if msg == "" && opts.DefaultMessage != nil {
			msg = opts.DefaultMessage(verb, f.Number, reason)
		}
		paths := []string{relPath}
		if opts.IndexPaths != nil {
			paths = append(paths, opts.IndexPaths()...)
		}
		sha, err := opts.Commit(opts.Dir, paths, msg)
		if err != nil {
			return StatusResult{}, false, fmt.Errorf("issues: %w", err)
		}
		result.Committed, result.CommitSHA = true, sha
	}
	return result, false, nil
}

// ApplyStatusBatch validates every ticket before mutation, rewrites statuses,
// refreshes injected indexes, and commits once when requested.
func ApplyStatusBatch(verb string, ticketArgs, reasonArgs []string, opts StatusBatchOptions) ([]StatusResult, bool, error) {
	reason := strings.TrimSpace(strings.Join(reasonArgs, " "))
	newStatus, err := ComposeStatus(verb, reason)
	if err != nil {
		return nil, false, err
	}
	type target struct {
		path, relPath string
		old, updated  []byte
		result        StatusResult
	}
	targets := make([]target, 0, len(ticketArgs))
	seen := make(map[string]bool, len(ticketArgs))
	issuesDir := filepath.Join(opts.Dir, "issues")
	for _, ticketArg := range ticketArgs {
		f, err := FindTicketFile(issuesDir, ticketArg)
		if err != nil {
			return nil, false, err
		}
		if seen[f.Number] {
			continue
		}
		seen[f.Number] = true
		filePath := filepath.Join(issuesDir, f.RelPath)
		relPath := filepath.ToSlash(filepath.Join("issues", f.RelPath))
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, false, fmt.Errorf("issues: read %s: %w", filePath, err)
		}
		if opts.WarnHeaderMismatch != nil {
			opts.WarnHeaderMismatch(opts.Stderr, relPath, f.Number, ParseHeaderNumber(string(content)))
		}
		_, oldStatus, hasStatus := ParseIssueFile(string(content))
		if !hasStatus {
			return nil, false, fmt.Errorf("issues: %s has no '**Status**:' line to update", relPath)
		}
		if err := ValidateLifecycleTransition(verb, oldStatus); err != nil {
			return nil, false, fmt.Errorf("issues: %s: %w", relPath, err)
		}
		targetStatus, err := CloseStatus(verb, oldStatus, newStatus, opts.AmendReason)
		if err != nil {
			return nil, false, fmt.Errorf("issues: %s: %w", relPath, err)
		}
		updated, _, err := RewriteStatus(string(content), targetStatus)
		if err != nil {
			return nil, false, fmt.Errorf("issues: %w", err)
		}
		result := StatusResult{Number: f.Number, File: relPath, OldStatus: oldStatus, NewStatus: targetStatus, Noop: oldStatus == targetStatus}
		targets = append(targets, target{path: filePath, relPath: relPath, old: content, updated: []byte(updated), result: result})
	}
	var changed []int
	for i := range targets {
		if !targets[i].result.Noop {
			changed = append(changed, i)
		}
	}
	if verb == "close" || verb == "done" {
		for _, i := range changed {
			if opts.WarnMissingImplementationCommit != nil {
				opts.WarnMissingImplementationCommit(opts.Stderr, opts.Dir, targets[i].result.Number)
			}
		}
	}
	results := func() []StatusResult {
		out := make([]StatusResult, len(targets))
		for i := range targets {
			out[i] = targets[i].result
		}
		return out
	}
	if len(changed) == 0 {
		return results(), false, nil
	}
	if opts.SnapshotIndexes == nil || opts.RestoreIndexes == nil || opts.UpdateIndexes == nil {
		return nil, false, fmt.Errorf("issues: status batch index operations are not configured")
	}
	snapshot, err := opts.SnapshotIndexes(issuesDir)
	if err != nil {
		return nil, false, fmt.Errorf("issues: snapshot indexes: %w", err)
	}
	restore := func() {
		for _, i := range changed {
			_ = os.WriteFile(targets[i].path, targets[i].old, 0o644)
		}
		_ = opts.RestoreIndexes(issuesDir, snapshot)
	}
	for _, i := range changed {
		if err := os.WriteFile(targets[i].path, targets[i].updated, 0o644); err != nil {
			restore()
			return nil, false, fmt.Errorf("issues: write %s: %w", targets[i].path, err)
		}
	}
	readmeChanged, err := opts.UpdateIndexes(issuesDir)
	if err != nil {
		restore()
		return nil, false, fmt.Errorf("issues: update README: %w", err)
	}
	if opts.Check {
		restore()
		for _, i := range changed {
			targets[i].result.ReadmeUpdated = readmeChanged
			break
		}
		return results(), true, nil
	}
	for _, i := range changed {
		targets[i].result.ReadmeUpdated = readmeChanged
		break
	}
	if !opts.NoCommit {
		paths := make([]string, 0, len(changed)+1)
		numbers := make([]string, 0, len(changed))
		for _, i := range changed {
			paths = append(paths, targets[i].relPath)
			numbers = append(numbers, targets[i].result.Number)
		}
		if opts.IndexPaths != nil {
			paths = append(paths, opts.IndexPaths()...)
		}
		msg := opts.CommitMsg
		if msg == "" {
			commitVerb := verb
			if commitVerb == "done" {
				commitVerb = "close"
			}
			msg = fmt.Sprintf("feat(issues): %s %s", commitVerb, strings.Join(numbers, ", "))
			if reason != "" {
				msg += ", " + reason
			}
		}
		if opts.Commit == nil {
			return nil, false, fmt.Errorf("issues: status batch commit operation is not configured")
		}
		sha, err := opts.Commit(opts.Dir, paths, msg)
		if err != nil {
			return nil, false, fmt.Errorf("issues: commit: %w", err)
		}
		for _, i := range changed {
			targets[i].result.Committed = true
			targets[i].result.CommitSHA = sha
		}
	}
	return results(), false, nil
}

// Package feedback implements `harnez feedback` (issue 184): a low-friction
// mechanism for an agent to durably record something it noticed mid-session
// (a harnez bug, a broken/contradictory instruction, a gap in its own
// guidance) without either losing the observation when the session ends or
// having to hand-author a full issues/NNN-*.md ticket on the spot.
//
// Two-tier design, per issue 184's decision:
//
//  1. A cheap append-only JSONL log, one file per project, under
//     ~/.harnez/feedback/ — reusing the ~/.harnez/... storage convention
//     resolve.DefaultStateDir already established for ~/.harnez/sessions/
//     (issues 121/183) rather than inventing a new location such as
//     ~/.cache/harnez/. Appending an entry is the whole cost of calling
//     `harnez feedback issue "<description>"`.
//  2. An explicit promotion path (--file-ticket at log time, or `harnez
//     feedback promote <id>` later) that turns a logged entry into a proper
//     issues/NNN-*.md ticket, reusing internal/issues' existing scan/number
//     logic instead of reimplementing ticket-file discovery.
//
// The log is append-only on disk, but Load folds repeated records for the
// same entry ID (last one wins), so "promote" can flip an entry's Status
// without ever rewriting or truncating the file — the same event-sourced
// shape sessionstate.go uses for its own per-session file, just keyed by
// project instead of session.
package feedback

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/issues"
)

// Entry is one feedback record.
type Entry struct {
	ID          string    `json:"id"`
	Time        time.Time `json:"time"`
	Project     string    `json:"project"`              // filepath.Base of the resolved project dir, for display
	SessionID   string    `json:"session_id,omitempty"` // best-effort; empty when unresolved
	Description string    `json:"description"`
	Severity    string    `json:"severity,omitempty"` // free-form, e.g. "bug", "instruction"; empty is fine
	Status      string    `json:"status"`             // "new" or "promoted"
	TicketPath  string    `json:"ticket_path,omitempty"`
}

const (
	StatusNew      = "new"
	StatusPromoted = "promoted"
)

// DefaultDir returns ~/.harnez/feedback, sibling to resolve.DefaultStateDir's
// ~/.harnez/sessions.
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".harnez", "feedback")
}

// projectKey hashes an absolute project directory path into the log
// filename, the same shortHash-and-hide-the-real-path scheme resolve.go and
// sessionstate.go both already use for their own per-key files.
func projectKey(absDir string) string {
	sum := sha256.Sum256([]byte(absDir))
	return hex.EncodeToString(sum[:8])
}

// Path returns the JSONL log file path for the project rooted at dir (any
// path; it is resolved to absolute before hashing).
func Path(feedbackDir, dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("feedback: resolve project dir: %w", err)
	}
	return filepath.Join(feedbackDir, projectKey(abs)+".jsonl"), nil
}

// NewID mints a short, session/description/time-derived entry ID. Not
// cryptographically unique, only collision-resistant enough for one
// project's feedback log.
func NewID(project, description string, t time.Time) string {
	sum := sha256.Sum256([]byte(project + "|" + description + "|" + t.Format(time.RFC3339Nano)))
	return hex.EncodeToString(sum[:4])
}

// Append writes one entry as a new JSONL line, creating the log file and its
// parent directory if needed.
func Append(feedbackDir, projectDir string, e Entry) error {
	path, err := Path(feedbackDir, projectDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("feedback: creating feedback dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("feedback: opening log: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("feedback: encoding entry: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("feedback: writing entry: %w", err)
	}
	return nil
}

// Load reads a project's feedback log, folding repeated records for the
// same ID (last write wins) so a later "promoted" record supersedes the
// original "new" one, then returns entries in first-seen order. A missing
// log file is not an error: it returns an empty slice, matching
// sessionstate.Load's "no file yet" convention.
func Load(feedbackDir, projectDir string) ([]Entry, error) {
	path, err := Path(feedbackDir, projectDir)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("feedback: opening log: %w", err)
	}
	defer f.Close()

	var order []string
	byID := map[string]Entry{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // skip a corrupt line rather than failing the whole read
		}
		if _, seen := byID[e.ID]; !seen {
			order = append(order, e.ID)
		}
		byID[e.ID] = e
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("feedback: reading log: %w", err)
	}

	entries := make([]Entry, 0, len(order))
	for _, id := range order {
		entries = append(entries, byID[id])
	}
	return entries, nil
}

// slugRegex matches runs of characters that aren't lowercase letters or
// digits, for turning a free-text description into a kebab-case slug.
var slugRegex = regexp.MustCompile(`[^a-z0-9]+`)

// Slug renders description as a kebab-case fragment suitable for a ticket
// filename, capped at 60 characters (matching the existing issues/*.md
// filename style).
func Slug(description string) string {
	s := strings.ToLower(description)
	s = slugRegex.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "feedback"
	}
	if len(s) > 60 {
		s = strings.Trim(s[:60], "-")
	}
	return s
}

// NextTicketNumber scans issuesDir the same way `harnez find`/`harnez index`
// do (via internal/issues.Scan) and returns the next free 3-digit ticket
// number, reusing existing numbering instead of a separate allocator.
func NextTicketNumber(issuesDir string) (string, error) {
	files, err := issues.Scan(issuesDir)
	if err != nil {
		return "", fmt.Errorf("feedback: scanning issues dir: %w", err)
	}
	max := 0
	for _, f := range files {
		if n, err := strconv.Atoi(f.Number); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%03d", max+1), nil
}

// ticketTemplate is the standard metadata block from docs/IssueTracking.md
// §3, filled in for an agent-filed feedback promotion.
const ticketTemplate = `# %s — %s

**Status**: Draft — agent-filed via ` + "`harnez feedback`" + `, needs triage
**Priority**: P3 (Low)
**Severity**: %s
**Category**: %s
**Related**: harnez feedback entry %s (filed %s%s)

---

## 1. Problem & Motivation

%s

## 2. Technical Specification / Findings

_Filed automatically from an agent's ` + "`harnez feedback issue --file-ticket`" + ` call; needs
human or agent triage to fill in specifics, confirm priority/severity, and flesh out a plan._

## 3. Implementation & Verification Plan

1. Triage: confirm this is a real, distinct issue (not a duplicate of an existing ticket).
2. Investigate and scope a fix.
3. Verify and update this ticket's Status.
`

// severityOrDefault normalizes a free-text severity into one of
// docs/IssueTracking.md's allowed Severity values, defaulting to Moderate
// when unset/unrecognized -- ticket promotion always needs a valid value,
// even though Entry.Severity itself is deliberately free-form at log time.
func severityOrDefault(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return "Critical"
	case "major":
		return "Major"
	case "minor":
		return "Minor"
	default:
		return "Moderate"
	}
}

// Promote writes issuesDir/NNN-<slug>.md for entry e, following
// docs/IssueTracking.md's metadata schema, and returns the created file's
// path. It does not run `harnez index` — the caller (or the filing agent)
// is expected to run that afterward, same as any other manually filed
// ticket in this repo's own convention.
func Promote(issuesDir string, e Entry) (string, error) {
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		return "", fmt.Errorf("feedback: creating issues dir: %w", err)
	}
	number, err := NextTicketNumber(issuesDir)
	if err != nil {
		return "", err
	}
	slug := Slug(e.Description)
	relName := fmt.Sprintf("%s-%s.md", number, slug)
	path := filepath.Join(issuesDir, relName)

	category := "Agentic Ergonomics"
	if strings.EqualFold(e.Severity, "bug") {
		category = "Bug"
	}

	sessionNote := ""
	if e.SessionID != "" {
		sessionNote = ", session " + e.SessionID
	}

	title := e.Description
	if len(title) > 100 {
		title = strings.TrimSpace(title[:100])
	}

	content := fmt.Sprintf(ticketTemplate,
		number, title,
		severityOrDefault(e.Severity), category,
		e.ID, e.Time.Format(time.RFC3339),
		sessionNote,
		e.Description,
	)

	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("feedback: ticket file already exists: %s", path)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("feedback: writing ticket file: %w", err)
	}
	return path, nil
}

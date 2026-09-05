// Package index regenerates the hand-maintained tracker/index tables that
// drift out of sync with their source files: issues/README.md's ticket
// table (from issues/*.md + issues/archive/*.md) and docs/README.md's
// docs/studies/ table (from docs/studies/*.md). See issue 148.
//
// Both regenerators are pure: read source files, render a table, compare
// against the existing content, and only write if it changed -- the same
// compare-before-write idempotency convention internal/claude/apply.go
// uses for managed sections.
package index

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"ubunatic.com/harnez/internal/issues"
)

// readmeLockRetries/readmeLockDelay bound how long UpdateIssuesReadme waits
// for the advisory flock on issues/README.md before giving up (~250ms total
// budget) -- the same non-blocking, bounded-retry flock idiom
// internal/usage/livefetchcache.go uses for its shared cache file (issue
// 232). Unlike that best-effort cache, this write is not optional, so a
// lock that can't be acquired within the budget is a hard error rather
// than a silent skip.
const readmeLockRetries = 5
const readmeLockDelay = 50 * time.Millisecond

// lockReadme takes a non-blocking, bounded-retry exclusive flock on a
// sidecar ".lock" file next to path, so two concurrent `harnez index` (or
// `harnez issues <verb>`) invocations never interleave writes to the same
// issues/README.md. Released automatically on process exit even if the
// holder crashes or is killed, since it's a kernel-held advisory lock, not
// a file whose mere existence signals "locked".
func lockReadme(path string) (*os.File, error) {
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", lockPath, err)
	}
	for attempt := 0; attempt <= readmeLockRetries; attempt++ {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			return f, nil
		}
		if attempt < readmeLockRetries {
			time.Sleep(readmeLockDelay)
		}
	}
	f.Close()
	return nil, fmt.Errorf("could not acquire lock on %s (held by another process)", path)
}

func unlockReadme(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}

var issueNumTitlePrefix = regexp.MustCompile(`^\d{3}\s*[—-]\s*`)

// IssuesTable renders the full issues/README.md table (header row,
// separator row, and one row per ticket) from the tickets found under
// issuesDir and issuesDir/archive.
func IssuesTable(issuesDir string) (string, error) {
	files, err := issues.Scan(issuesDir)
	if err != nil {
		return "", fmt.Errorf("scan issues: %w", err)
	}

	var b strings.Builder
	b.WriteString("| # | File | Title | Status |\n")
	b.WriteString("|---|------|-------|--------|\n")
	for _, f := range files {
		title := issueNumTitlePrefix.ReplaceAllString(f.Title, "")
		title = strings.ReplaceAll(title, "|", `\|`)
		status := f.RawStatus
		if status == "" {
			status = "Unknown"
		}
		fmt.Fprintf(&b, "| %s | [%s](%s) | %s | %s |\n", f.Number, f.RelPath, f.RelPath, title, status)
	}
	return b.String(), nil
}

// StatusCounts aggregates the same ticket set IssuesTable renders
// (issuesDir + issuesDir/archive/*.md) into open/closed/draft/unknown
// counts by canonical status category -- the rollup issue 228's
// `harnez index` snapshot-write path records into the telemetry DB's
// issue_status_snapshots table (internal/telemetry).
func StatusCounts(issuesDir string) (open, closed, draft, unknown int, err error) {
	files, err := issues.Scan(issuesDir)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("scan issues: %w", err)
	}
	for _, f := range files {
		switch f.Canonical {
		case issues.StatusOpen:
			open++
		case issues.StatusClosed:
			closed++
		case issues.StatusDraft:
			draft++
		default:
			unknown++
		}
	}
	return open, closed, draft, unknown, nil
}

var issuesTableHeaderRe = regexp.MustCompile(`(?m)^\|\s*#\s*\|`)

const issuesTableHeader = "| # | File | Title | Status |"

// UpdateIssuesReadme regenerates the ticket table in readmePath (normally
// issues/README.md) from the tickets in issuesDir. Content before and after
// the canonical table is preserved verbatim. A customized table schema is
// refused so project-specific columns are never silently discarded. Returns
// whether the file's content changed.
func UpdateIssuesReadme(readmePath, issuesDir string) (bool, error) {
	lockFile, err := lockReadme(readmePath)
	if err != nil {
		return false, fmt.Errorf("update %s: %w", readmePath, err)
	}
	defer unlockReadme(lockFile)

	table, err := IssuesTable(issuesDir)
	if err != nil {
		return false, err
	}

	orig, err := os.ReadFile(readmePath)
	if err != nil {
		if os.IsNotExist(err) {
			orig = []byte("# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n")
		} else {
			return false, fmt.Errorf("read %s: %w", readmePath, err)
		}
	}
	content := string(orig)

	loc := issuesTableHeaderRe.FindStringIndex(content)
	if loc == nil {
		return false, fmt.Errorf("%s: could not find issues table header ('| # | File | Title | Status |')", readmePath)
	}

	headerEnd := strings.IndexByte(content[loc[0]:], '\n')
	if headerEnd == -1 {
		headerEnd = len(content)
	} else {
		headerEnd += loc[0]
	}
	header := strings.TrimSpace(content[loc[0]:headerEnd])
	if header != issuesTableHeader {
		return false, fmt.Errorf("%s: refusing to replace customized issues table header %q; expected %q (project-specific columns must be reconciled manually)", readmePath, header, issuesTableHeader)
	}

	// The managed block is the consecutive run of Markdown table lines that
	// starts at the canonical header. Stop before the first non-table line so
	// hand-authored sections after the table survive regeneration.
	tableEnd := loc[0]
	for tableEnd < len(content) {
		lineEnd := strings.IndexByte(content[tableEnd:], '\n')
		if lineEnd == -1 {
			lineEnd = len(content)
		} else {
			lineEnd += tableEnd + 1
		}
		if !strings.HasPrefix(strings.TrimSpace(content[tableEnd:lineEnd]), "|") {
			break
		}
		tableEnd = lineEnd
	}

	newContent := content[:loc[0]] + table + content[tableEnd:]
	if newContent == content {
		return false, nil
	}
	if err := os.WriteFile(readmePath, []byte(newContent), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", readmePath, err)
	}
	return true, nil
}

var (
	topicOverrideRe   = regexp.MustCompile(`<!--\s*harnez:topic:\s*(.+?)\s*-->`)
	h1Re              = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	titleKindPrefixRe = regexp.MustCompile(`^(?:Case Study|Study|ADR):\s*`)
	titleDatePrefixRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\s*[—-]\s*`)
)

// StudyTopic derives the one-line topic/summary text for a docs/studies/*.md
// file's index row, in priority order:
//  1. An explicit `<!-- harnez:topic: ... -->` override anywhere in the file
//     -- the escape hatch for curated phrasing a generator can't derive.
//  2. The `**Scope**:` metadata field (joined across its wrapped lines,
//     since several studies wrap it over 2-3 lines without markdown hard
//     breaks).
//  3. The H1 title, with common study-doc prefixes ("Case Study: ",
//     "Study: ", "ADR: ") and a leading "YYYY-MM-DD — " date stripped.
func StudyTopic(content string) string {
	if m := topicOverrideRe.FindStringSubmatch(content); m != nil {
		return strings.TrimSpace(m[1])
	}
	if scope := extractScope(content); scope != "" {
		return scope
	}
	if m := h1Re.FindStringSubmatch(content); m != nil {
		t := strings.TrimSpace(m[1])
		t = titleKindPrefixRe.ReplaceAllString(t, "")
		t = titleDatePrefixRe.ReplaceAllString(t, "")
		return strings.TrimSpace(t)
	}
	return ""
}

// extractScope finds a "**Scope**: ..." bold-label line and joins it with
// any immediately following non-blank, non-bold-label lines (a soft-wrapped
// continuation of the same field), collapsing them into one line.
func extractScope(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "**Scope**:") {
			continue
		}
		parts := []string{strings.TrimSpace(strings.TrimPrefix(trimmed, "**Scope**:"))}
		for j := i + 1; j < len(lines); j++ {
			next := strings.TrimSpace(lines[j])
			if next == "" || strings.HasPrefix(next, "**") {
				break
			}
			parts = append(parts, next)
		}
		return strings.TrimSpace(strings.Join(parts, " "))
	}
	return ""
}

type studyRow struct {
	relPath string
	topic   string
}

// StudiesTable renders the docs/README.md docs/studies/ table (header row,
// separator row, and one row per *.md file directly under docsDir/studies)
// sorted by filename.
func StudiesTable(docsDir string) (string, error) {
	studiesDir := filepath.Join(docsDir, "studies")
	entries, err := os.ReadDir(studiesDir)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", studiesDir, err)
	}

	var rows []studyRow
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(studiesDir, e.Name()))
		if err != nil {
			return "", fmt.Errorf("read %s: %w", e.Name(), err)
		}
		rows = append(rows, studyRow{
			relPath: "studies/" + e.Name(),
			topic:   StudyTopic(string(data)),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].relPath < rows[j].relPath })

	var b strings.Builder
	b.WriteString("| File | Topic |\n")
	b.WriteString("|------|-------|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| [%s](%s) | %s |\n", r.relPath, r.relPath, r.topic)
	}
	return b.String(), nil
}

const (
	docsStudiesAnchor    = "**`docs/studies/`**"
	docsStudiesTableHead = "| File | Topic |"
)

// UpdateDocsReadme regenerates only the docs/studies/ table inside
// readmePath (normally docs/README.md) -- the section immediately following
// the docsStudiesAnchor marker line -- from docsDir/studies/*.md, leaving
// every other table (root docs, lang, practices/other, feedback, proposed)
// untouched. Returns whether the file's content changed.
func UpdateDocsReadme(readmePath, docsDir string) (bool, error) {
	table, err := StudiesTable(docsDir)
	if err != nil {
		return false, err
	}

	orig, err := os.ReadFile(readmePath)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", readmePath, err)
	}
	content := string(orig)

	anchorIdx := strings.Index(content, docsStudiesAnchor)
	if anchorIdx == -1 {
		return false, fmt.Errorf("%s: could not find %q section anchor", readmePath, docsStudiesAnchor)
	}
	headerRel := strings.Index(content[anchorIdx:], docsStudiesTableHead)
	if headerRel == -1 {
		return false, fmt.Errorf("%s: could not find studies table header after %q", readmePath, docsStudiesAnchor)
	}
	headerIdx := anchorIdx + headerRel

	rest := content[headerIdx:]
	endRel := strings.Index(rest, "\n\n")
	if endRel == -1 {
		return false, fmt.Errorf("%s: could not find end of studies table (expected a trailing blank line)", readmePath)
	}
	blockEnd := headerIdx + endRel

	newContent := content[:headerIdx] + strings.TrimRight(table, "\n") + content[blockEnd:]
	if newContent == content {
		return false, nil
	}
	if err := os.WriteFile(readmePath, []byte(newContent), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", readmePath, err)
	}
	return true, nil
}

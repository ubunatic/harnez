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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ubunatic.com/harnez/internal/issues"
	"ubunatic.com/harnez/internal/markdown"
)

// readmeLockRetries/readmeLockDelay bound how long UpdateIssuesReadme waits
// for the advisory flock on issues/README.md before giving up (~250ms total
// budget) -- the same non-blocking, bounded-retry flock idiom
// internal/usage/livefetchcache.go uses for its shared cache file (issue
// 232). Unlike that best-effort cache, this write is not optional, so a
// lock that can't be acquired within the budget is a hard error rather
// than a silent skip.
const readmeLockRetries = 5
const readmeMaxStaleReopens = 10
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
	stale := 0
	for attempt := 0; attempt <= readmeLockRetries; {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			// The previous holder removes the file on unlock; make sure we
			// locked the file that is still on disk, else reopen (issue 1066).
			fi, e1 := f.Stat()
			pi, e2 := os.Stat(lockPath)
			if e1 == nil && e2 == nil && os.SameFile(fi, pi) {
				return f, nil
			}
			f.Close()
			// Stale reopen does not consume the retry budget; bounded separately.
			if stale++; stale > readmeMaxStaleReopens {
				return nil, fmt.Errorf("could not acquire lock on %s (lock file kept being replaced)", path)
			}
			if f, err = os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600); err != nil {
				return nil, fmt.Errorf("open lock file %s: %w", lockPath, err)
			}
			continue
		}
		if attempt < readmeLockRetries {
			time.Sleep(readmeLockDelay)
		}
		attempt++
	}
	f.Close()
	return nil, fmt.Errorf("could not acquire lock on %s (held by another process)", path)
}

func unlockReadme(f *os.File) {
	// Remove while still holding the lock so no sidecar is left in the tree.
	_ = os.Remove(f.Name())
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}

// IssuesTable renders the active-ticket table used by issues/README.md.
// Open (including In Progress and Blocked) and Draft tickets are active.
func IssuesTable(issuesDir string) (string, error) {
	files, err := issues.Scan(issuesDir)
	if err != nil {
		return "", fmt.Errorf("scan issues: %w", err)
	}
	return renderIssueTable(files, func(f issues.IssueFile) bool {
		return f.Canonical == issues.StatusOpen || f.Canonical == issues.StatusDraft
	}), nil
}

// IssueIndexFiles renders the three generated tracker indexes. The README
// contains active work; Backlog and Closed retain their complete status sets,
// including archived closed tickets.
func IssueIndexFiles(issuesDir string) (map[string]string, error) {
	files, err := issues.Scan(issuesDir)
	if err != nil {
		return nil, fmt.Errorf("scan issues: %w", err)
	}
	active := renderIssueTable(files, func(f issues.IssueFile) bool {
		return f.Canonical == issues.StatusOpen || f.Canonical == issues.StatusDraft
	})
	backlog := renderIssueTable(files, func(f issues.IssueFile) bool {
		return f.Canonical == issues.StatusBacklog
	})
	closed := renderIssueTable(files, func(f issues.IssueFile) bool {
		return f.Canonical == issues.StatusClosed
	})
	return map[string]string{
		"README.md":  managedDocument("# Issues\n\nActive tickets. See [Backlog](BACKLOG.md) and [Closed](CLOSED.md).\n\n"+active, "issues-active-index", issuesActiveNotice),
		"BACKLOG.md": managedDocument("# Backlog\n\n"+backlog, "issues-backlog-index", issuesFullNotice),
		"CLOSED.md":  managedDocument("# Closed\n\n"+closed, "issues-closed-index", issuesFullNotice),
	}, nil
}

const (
	issuesActiveNotice = "<!-- managed by harnez index: active ticket table generated from issue files; edit ticket files instead -->"
	issuesFullNotice   = "<!-- managed by harnez index: generated from issue files; edit ticket files instead -->"
	docsStudiesNotice  = "<!-- managed by harnez index: only the studies table is generated; evergreen documentation and other tables are hand-written -->"
)

func managedDocument(content, id, notice string) string {
	return notice + "\n" + markdown.MDMarkers.Begin(id) + "\n" + strings.TrimRight(content, "\n") + "\n" + markdown.MDMarkers.End(id) + "\n"
}

// HasConcurrentEditsOutsideManagedBlock reports whether both sides changed
// the hand-written bytes outside id's managed block from the common ancestor
// and disagree with one another. Missing or malformed markers treat the full
// document as hand-written, which makes a merge driver fail safely.
func HasConcurrentEditsOutsideManagedBlock(ancestor, ours, theirs, id string) bool {
	outside := func(content string) string {
		start, end, found := markdown.SectionBounds(content, markdown.MDMarkers.Begin(id), markdown.MDMarkers.End(id))
		if !found {
			return content
		}
		return content[:start] + content[end:]
	}
	baseOutside, oursOutside, theirsOutside := outside(ancestor), outside(ours), outside(theirs)
	return oursOutside != baseOutside && theirsOutside != baseOutside && oursOutside != theirsOutside
}

// MergeIssuesReadme merges generated active-ticket rows from the three index
// versions while preserving the current branch's hand-written content.
func MergeIssuesReadme(ancestor, ours, theirs string) (string, error) {
	return mergeIssuesReadme(ancestor, ours, theirs, false)
}

// MergeIssuesReadmeReplay is MergeIssuesReadme for an `issues rebase` replay:
// a conflicting row keeps the current version, because the repair step that
// follows the replay regenerates the index from the ticket files.
func MergeIssuesReadmeReplay(ancestor, ours, theirs string) (string, error) {
	return mergeIssuesReadme(ancestor, ours, theirs, true)
}

func mergeIssuesReadme(ancestor, ours, theirs string, keepOurs bool) (string, error) {
	table, err := mergeGeneratedTableRows(ancestor, ours, theirs, "issues-active-index", issuesTableHeader, "|---|------|-------|--------|", keepOurs)
	if err != nil {
		return "", err
	}
	return replaceManagedBlock(ours, "issues-active-index", table, -1, -1)
}

// MergeFullIssueIndex merges a generated Backlog or Closed index. The full
// document is managed, so the current branch's title and notice remain the
// document shell around the merged rows.
func MergeFullIssueIndex(ancestor, ours, theirs, name string) (string, error) {
	return mergeFullIssueIndex(ancestor, ours, theirs, name, false)
}

// MergeFullIssueIndexReplay is MergeFullIssueIndex for an `issues rebase`
// replay; see MergeIssuesReadmeReplay.
func MergeFullIssueIndexReplay(ancestor, ours, theirs, name string) (string, error) {
	return mergeFullIssueIndex(ancestor, ours, theirs, name, true)
}

func mergeFullIssueIndex(ancestor, ours, theirs, name string, keepOurs bool) (string, error) {
	id, title := "issues-backlog-index", "# Backlog"
	if name == "CLOSED.md" {
		id, title = "issues-closed-index", "# Closed"
	} else if name != "BACKLOG.md" {
		return "", fmt.Errorf("unsupported full issue index %q", name)
	}
	table, err := mergeGeneratedTableRows(ancestor, ours, theirs, id, issuesTableHeader, "|---|------|-------|--------|", keepOurs)
	if err != nil {
		return "", err
	}
	content := managedDocument(title+"\n\n"+table, id, issuesFullNotice)
	return content, nil
}

// MergeDocsReadme merges generated study rows from the three index versions
// while preserving the current branch's hand-written content byte-for-byte.
func MergeDocsReadme(ancestor, ours, theirs, docsDir, readmePath string) (string, error) {
	table, err := mergeGeneratedTableRows(ancestor, ours, theirs, "docs-studies-index", "| File | Topic |", "|------|-------|", false)
	if err != nil {
		return "", err
	}
	canonical, err := RenderDocsReadme(ours, docsDir, readmePath)
	if err != nil {
		return "", err
	}
	return replaceManagedBlock(canonical, "docs-studies-index", table, -1, -1)
}

func mergeGeneratedTableRows(ancestor, ours, theirs, id, header, separator string, keepOurs bool) (string, error) {
	baseRows := generatedTableRows(ancestor, id)
	ourRows := generatedTableRows(ours, id)
	theirRows := generatedTableRows(theirs, id)
	keys := make(map[string]struct{}, len(baseRows)+len(ourRows)+len(theirRows))
	for key := range baseRows {
		keys[key] = struct{}{}
	}
	for key := range ourRows {
		keys[key] = struct{}{}
	}
	for key := range theirRows {
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	var table strings.Builder
	table.WriteString(header + "\n" + separator + "\n")
	for _, key := range ordered {
		base, baseExists := baseRows[key]
		oursRow, oursExists := ourRows[key]
		theirsRow, theirsExists := theirRows[key]
		merged := ""
		switch {
		case sameGeneratedRow(oursRow, oursExists, theirsRow, theirsExists):
			merged = oursRow
		case sameGeneratedRow(oursRow, oursExists, base, baseExists):
			merged = theirsRow
		case sameGeneratedRow(theirsRow, theirsExists, base, baseExists):
			merged = oursRow
		case keepOurs:
			merged = oursRow
		case !baseExists:
			return "", fmt.Errorf("conflicting generated rows for %s in managed block %q", key, id)
		default:
			return "", fmt.Errorf("conflicting edits to generated row %s in managed block %q", key, id)
		}
		if merged != "" {
			table.WriteString(merged + "\n")
		}
	}
	return table.String(), nil
}

func sameGeneratedRow(a string, aExists bool, b string, bExists bool) bool {
	return aExists == bExists && (!aExists || a == b)
}

func generatedTableRows(document, id string) map[string]string {
	rows := make(map[string]string)
	start, end, found := markdown.SectionBounds(document, markdown.MDMarkers.Begin(id), markdown.MDMarkers.End(id))
	if !found {
		return rows
	}
	block := document[start:end]
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cellEnd := strings.Index(line[1:], "|")
		if cellEnd < 0 {
			continue
		}
		firstCell := strings.TrimSpace(line[1 : cellEnd+1])
		key := firstCell
		if strings.HasPrefix(firstCell, "[") {
			linkEnd := strings.Index(firstCell, "](")
			if linkEnd < 0 {
				continue
			}
			key = strings.TrimSuffix(firstCell[linkEnd+2:], ")")
		} else if _, err := strconv.Atoi(firstCell); err != nil {
			continue
		}
		rows[key] = line
	}
	return rows
}

func ensureManagedNotice(content, notice string) string {
	if strings.HasPrefix(content, "<!-- managed by harnez index:") {
		if end := strings.IndexByte(content, '\n'); end >= 0 {
			return notice + content[end:]
		}
		return notice + "\n"
	}
	return notice + "\n" + content
}

// replaceManagedBlock replaces a marked section or migrates a legacy section
// using its source offsets. Content outside the section is preserved verbatim.
func replaceManagedBlock(content, id, generated string, legacyStart, legacyEnd int) (string, error) {
	begin, end := markdown.MDMarkers.Begin(id), markdown.MDMarkers.End(id)
	beginAt, endAt := strings.Index(content, begin), strings.Index(content, end)
	if beginAt >= 0 || endAt >= 0 {
		if beginAt < 0 || endAt < beginAt || strings.Contains(content[beginAt+len(begin):], begin) {
			return "", fmt.Errorf("malformed generated block %q", id)
		}
		lineStart := strings.LastIndex(content[:beginAt], "\n") + 1
		lineEnd := strings.IndexByte(content[endAt+len(end):], '\n')
		if lineStart != beginAt || (lineEnd < 0 && endAt+len(end) != len(content)) {
			return "", fmt.Errorf("generated block %q markers must occupy complete lines", id)
		}
		blockEnd := len(content)
		blockSuffix := "\n"
		if strings.Contains(content, "\r\n") {
			blockSuffix = "\r\n"
		}
		if lineEnd >= 0 {
			blockEnd = endAt + len(end) + lineEnd + 1
			blockSuffix = "\n"
			if lineEnd > 0 && content[endAt+len(end)+lineEnd-1] == '\r' {
				blockSuffix = "\r\n"
			}
		}
		block := begin + "\n" + strings.TrimRight(generated, "\n") + "\n" + end + blockSuffix
		return content[:lineStart] + block + content[blockEnd:], nil
	}
	if legacyStart < 0 || legacyEnd < legacyStart || legacyEnd > len(content) {
		return "", fmt.Errorf("cannot locate legacy generated section %q", id)
	}
	block := begin + "\n" + strings.TrimRight(generated, "\n") + "\n" + end + "\n"
	return content[:legacyStart] + block + content[legacyEnd:], nil
}

// HasConflictOutsideManagedBlock reports Git conflict markers outside a
// generated section. A merge driver may regenerate the marked section, but it
// must leave hand-written conflicts for the user to resolve.
func HasConflictOutsideManagedBlock(content, id string) bool {
	begin, end := markdown.MDMarkers.Begin(id), markdown.MDMarkers.End(id)
	beginAt, endAt := strings.Index(content, begin), strings.Index(content, end)
	if beginAt < 0 || endAt < beginAt {
		return hasConflictMarkers(content)
	}
	lineStart := strings.LastIndex(content[:beginAt], "\n") + 1
	if lineStart != beginAt {
		return hasConflictMarkers(content)
	}
	endAfter := endAt + len(end)
	if newline := strings.IndexByte(content[endAfter:], '\n'); newline >= 0 {
		endAfter += newline + 1
	}
	return hasConflictMarkers(content[:lineStart]) || hasConflictMarkers(content[endAfter:])
}

func hasConflictMarkers(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(line, "<<<<<<< ") || strings.HasPrefix(line, ">>>>>>> ") || strings.HasPrefix(line, "||||||| ") || line == "=======" {
			return true
		}
	}
	return false
}

func renderIssueTable(files []issues.IssueFile, include func(issues.IssueFile) bool) string {

	var b strings.Builder
	b.WriteString("| # | File | Title | Status |\n")
	b.WriteString("|---|------|-------|--------|\n")
	for _, f := range files {
		if !include(f) {
			continue
		}
		title := issues.StripTicketNumber(f.Title)
		title = strings.ReplaceAll(title, "|", `\|`)
		status := strings.ReplaceAll(f.RawStatus, "|", `\|`)
		if status == "" {
			status = "Unknown"
		}
		fmt.Fprintf(&b, "| %s | [%s](%s) | %s | %s |\n", f.Number, f.RelPath, f.RelPath, title, status)
	}
	return b.String()
}

// StatusCounts aggregates the same ticket set IssuesTable renders
// (issuesDir + issuesDir/archive/*.md) into open/closed/draft/backlog/unknown
// counts by canonical status category -- the rollup issue 228's
// `harnez index` snapshot-write path records into the telemetry DB's
// issue_status_snapshots table (internal/telemetry).
func StatusCounts(issuesDir string) (open, closed, draft, backlog, unknown int, err error) {
	files, err := issues.Scan(issuesDir)
	if err != nil {
		return 0, 0, 0, 0, 0, fmt.Errorf("scan issues: %w", err)
	}
	for _, f := range files {
		switch f.Canonical {
		case issues.StatusOpen:
			open++
		case issues.StatusClosed:
			closed++
		case issues.StatusDraft:
			draft++
		case issues.StatusBacklog:
			backlog++
		default:
			unknown++
		}
	}
	return open, closed, draft, backlog, unknown, nil
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

	orig, err := os.ReadFile(readmePath)
	if err != nil {
		if os.IsNotExist(err) {
			orig = []byte("# Issues\n\n| # | File | Title | Status |\n|---|------|-------|--------|\n")
		} else {
			return false, fmt.Errorf("read %s: %w", readmePath, err)
		}
	}
	newContent, err := RenderIssuesReadme(string(orig), issuesDir, readmePath)
	if err != nil {
		return false, err
	}
	indexes, err := IssueIndexFiles(issuesDir)
	if err != nil {
		return false, err
	}
	changed := newContent != string(orig)
	if changed {
		if err := os.WriteFile(readmePath, []byte(newContent), 0o644); err != nil {
			return false, fmt.Errorf("write %s: %w", readmePath, err)
		}
	}
	for _, name := range []string{"BACKLOG.md", "CLOSED.md"} {
		path := filepath.Join(filepath.Dir(readmePath), name)
		old, readErr := os.ReadFile(path)
		if readErr != nil && !os.IsNotExist(readErr) {
			return false, fmt.Errorf("read %s: %w", path, readErr)
		}
		if string(old) == indexes[name] {
			continue
		}
		if err := os.WriteFile(path, []byte(indexes[name]), 0o644); err != nil {
			return false, fmt.Errorf("write %s: %w", path, err)
		}
		changed = true
	}
	return changed, nil
}

// RenderIssuesReadme returns the canonical generated tracker content without
// touching the filesystem. Callers such as lint and Git merge drivers use it
// when mutation of the checked-out README would be unsafe.
func RenderIssuesReadme(content, issuesDir, readmePath string) (string, error) {
	table, err := IssuesTable(issuesDir)
	if err != nil {
		return "", err
	}
	content = ensureManagedNotice(content, issuesActiveNotice)
	if strings.Contains(content, markdown.MDMarkers.Begin("issues-active-index")) || strings.Contains(content, markdown.MDMarkers.End("issues-active-index")) {
		return replaceManagedBlock(content, "issues-active-index", table, -1, -1)
	}

	loc := issuesTableHeaderRe.FindStringIndex(content)
	if loc == nil {
		return "", fmt.Errorf("%s: could not find issues table header ('| # | File | Title | Status |')", readmePath)
	}

	headerEnd := strings.IndexByte(content[loc[0]:], '\n')
	if headerEnd == -1 {
		headerEnd = len(content)
	} else {
		headerEnd += loc[0]
	}
	header := strings.TrimSpace(content[loc[0]:headerEnd])
	if header != issuesTableHeader {
		return "", fmt.Errorf("%s: refusing to replace customized issues table header %q; expected %q (project-specific columns must be reconciled manually)", readmePath, header, issuesTableHeader)
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

	prefix := content[:loc[0]]
	var missingLinks []string
	if !strings.Contains(prefix, "[Backlog](BACKLOG.md)") {
		missingLinks = append(missingLinks, "[Backlog](BACKLOG.md)")
	}
	if !strings.Contains(prefix, "[Closed](CLOSED.md)") {
		missingLinks = append(missingLinks, "[Closed](CLOSED.md)")
	}
	if len(missingLinks) > 0 {
		prefix += strings.Join(missingLinks, " · ") + "\n\n"
	}
	content = prefix + content[loc[0]:]
	tableStart := len(prefix)
	return replaceManagedBlock(content, "issues-active-index", table, tableStart, tableStart+tableEnd-loc[0])
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

// studySource is one study to index: its file name and the content its topic
// is read from.
type studySource struct {
	name string
	data []byte
}

// IsStudyName reports whether a file under docs/studies is indexed: a *.md
// file that is not a dotfile.
func IsStudyName(name string) bool {
	return strings.HasSuffix(name, ".md") && !strings.HasPrefix(name, ".")
}

// studyLister lists the studies of a docs directory.
type studyLister func(docsDir string) ([]studySource, error)

// studySources lists the studies the table indexes. Inside a git work tree it
// follows git's index, so untracked notes never reach the table and a staged
// study is indexed with its staged content, matching the --cached lint's
// snapshot. Outside one it lists the directory. Dotfiles are never indexed.
func studySources(docsDir string) ([]studySource, error) {
	inTree, err := gitInWorkTree(docsDir)
	if err != nil {
		return nil, err
	}
	if inTree {
		return gitStudySources(docsDir)
	}
	return dirStudySources(docsDir)
}

// gitRepoEnv names the variables a git hook exports to point git at its own
// repository. harnez reads other repositories, so they are removed.
var gitRepoEnv = map[string]bool{
	"GIT_DIR":        true,
	"GIT_WORK_TREE":  true,
	"GIT_INDEX_FILE": true,
	"GIT_PREFIX":     true,
	"GIT_COMMON_DIR": true,
}

// gitCommand runs git in dir, so that git discovers the repository of dir
// itself and not one named by an inherited hook variable.
func GitCommand(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if !gitRepoEnv[name] {
			env = append(env, kv)
		}
	}
	cmd.Env = env
	return cmd
}

// gitError wraps a failed git command, adding git's own stderr when it has any.
func gitError(op string, err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if msg := strings.TrimSpace(string(exit.Stderr)); msg != "" {
			return fmt.Errorf("git %s: %w: %s", op, err, msg)
		}
	}
	return fmt.Errorf("git %s: %w", op, err)
}

// gitInWorkTree reports whether docsDir lies inside a git work tree. Outside
// any repository, or for a missing directory, git exits 128 with a known
// message and the answer is no. Any other failure is an error: falling back to
// the directory then could index untracked notes.
func gitInWorkTree(docsDir string) (bool, error) {
	out, err := GitCommand(docsDir, "rev-parse", "--is-inside-work-tree").Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 128 {
			msg := string(exit.Stderr)
			if strings.Contains(msg, "not a git repository") || strings.Contains(msg, "cannot change to") {
				return false, nil
			}
		}
		return false, gitError("rev-parse --is-inside-work-tree", err)
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

// dirStudySources lists docsDir/studies from the worktree, skipping
// directories, symlinks (git does not index them either) and dotfiles.
func dirStudySources(docsDir string) ([]studySource, error) {
	studiesDir := filepath.Join(docsDir, "studies")
	entries, err := os.ReadDir(studiesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", studiesDir, err)
	}
	var sources []studySource
	for _, e := range entries {
		if e.IsDir() || e.Type()&os.ModeSymlink != 0 || !IsStudyName(e.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(studiesDir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		sources = append(sources, studySource{name: e.Name(), data: data})
	}
	return sources, nil
}

// gitStudySources lists docsDir/studies from git's index. A study is listed
// once even when it is unmerged (one entry per conflict stage). Its content is
// the index blob, or the worktree file when the path has no stage-0 entry
// (conflicted), and skipped when that file is gone (deleted by one side).
// Symlinks and submodules are not studies.
func gitStudySources(docsDir string) ([]studySource, error) {
	out, err := GitCommand(docsDir, "ls-files", "-s", "-z", "--", "studies").Output()
	if err != nil {
		return nil, gitError("ls-files studies", err)
	}
	type indexed struct {
		oid string // stage-0 blob; empty when the path is only unmerged
	}
	byName := map[string]*indexed{}
	var names []string
	for _, rec := range strings.Split(string(out), "\x00") {
		if rec == "" {
			continue
		}
		// Record: "<mode> SP <object> SP <stage> TAB <path>".
		tab := strings.IndexByte(rec, '\t')
		if tab < 0 {
			return nil, fmt.Errorf("git ls-files studies: unexpected record %q", rec)
		}
		fields := strings.Fields(rec[:tab])
		if len(fields) != 3 {
			return nil, fmt.Errorf("git ls-files studies: unexpected record %q", rec)
		}
		path := rec[tab+1:]
		name := filepath.Base(path)
		if filepath.Dir(path) != "studies" || !IsStudyName(name) {
			continue
		}
		if fields[0] == "120000" || fields[0] == "160000" {
			continue
		}
		entry := byName[name]
		if entry == nil {
			entry = &indexed{}
			byName[name] = entry
			names = append(names, name)
		}
		if fields[2] == "0" {
			entry.oid = fields[1]
		}
	}
	var oids []string
	for _, name := range names {
		if oid := byName[name].oid; oid != "" {
			oids = append(oids, oid)
		}
	}
	blobs, err := catFileBlobs(docsDir, oids)
	if err != nil {
		return nil, err
	}
	var sources []studySource
	next := 0
	for _, name := range names {
		if byName[name].oid == "" {
			data, err := os.ReadFile(filepath.Join(docsDir, "studies", name))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", name, err)
			}
			sources = append(sources, studySource{name: name, data: data})
			continue
		}
		sources = append(sources, studySource{name: name, data: blobs[next]})
		next++
	}
	return sources, nil
}

// catFileBlobs returns the contents of the given blobs in order, using one
// git cat-file --batch process.
func catFileBlobs(docsDir string, oids []string) ([][]byte, error) {
	if len(oids) == 0 {
		return nil, nil
	}
	cmd := GitCommand(docsDir, "cat-file", "--batch")
	cmd.Stdin = strings.NewReader(strings.Join(oids, "\n") + "\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git cat-file --batch: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return readCatFileBatch(&stdout, oids)
}

// readCatFileBatch parses the output of git cat-file --batch for oids, one
// blob per oid, in order. It rejects headers that are not blob headers, bad
// sizes, truncated blobs and blobs without their newline terminator.
func readCatFileBatch(src io.Reader, oids []string) ([][]byte, error) {
	r := bufio.NewReader(src)
	blobs := make([][]byte, 0, len(oids))
	for range oids {
		header, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("git cat-file --batch: read header: %w", err)
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[1] != "blob" {
			return nil, fmt.Errorf("git cat-file --batch: unexpected header %q", strings.TrimSpace(header))
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 {
			return nil, fmt.Errorf("git cat-file --batch: bad size in %q", strings.TrimSpace(header))
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, fmt.Errorf("git cat-file --batch: read blob %s: %w", fields[0], err)
		}
		term, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("git cat-file --batch: read blob terminator: %w", err)
		}
		if term != '\n' {
			return nil, fmt.Errorf("git cat-file --batch: blob %s is not newline-terminated", fields[0])
		}
		blobs = append(blobs, data)
	}
	return blobs, nil
}

// StudiesTable renders the docs/README.md docs/studies/ table (header row,
// separator row, and one row per indexed study; see studySources) sorted by
// filename.
func StudiesTable(docsDir string) (string, error) {
	return studiesTableFrom(docsDir, studySources)
}

// IndexedDiffers returns the paths among rel, relative to repoDir, whose
// worktree content differs from the index. Untracked paths are not reported.
func IndexedDiffers(repoDir string, rel []string) ([]string, error) {
	var differs []string
	for _, p := range rel {
		err := GitCommand(repoDir, "diff", "--quiet", "--", p).Run()
		if err == nil {
			continue
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			differs = append(differs, p)
			continue
		}
		return nil, gitError("diff "+p, err)
	}
	return differs, nil
}

// StagedContent returns the staged blob of rel, relative to repoDir. ok is
// false when rel has no index entry, for example a file only on disk.
func StagedContent(repoDir, rel string) (content []byte, ok bool, err error) {
	out, err := GitCommand(repoDir, "show", ":./"+rel).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 128 {
			return nil, false, nil
		}
		return nil, false, gitError("show "+rel, err)
	}
	return out, true, nil
}

func studiesTableFrom(docsDir string, list studyLister) (string, error) {
	sources, err := list(docsDir)
	if err != nil {
		return "", err
	}

	var rows []studyRow
	for _, s := range sources {
		rows = append(rows, studyRow{
			relPath: "studies/" + s.name,
			topic:   StudyTopic(string(s.data)),
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
	if err := os.MkdirAll(filepath.Join(docsDir, "studies"), 0o755); err != nil {
		return false, fmt.Errorf("create %s: %w", filepath.Join(docsDir, "studies"), err)
	}
	orig, err := os.ReadFile(readmePath)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", readmePath, err)
	}
	newContent, err := RenderDocsReadme(string(orig), docsDir, readmePath)
	if err != nil {
		return false, err
	}
	if newContent == string(orig) {
		return false, nil
	}
	if err := os.WriteFile(readmePath, []byte(newContent), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", readmePath, err)
	}
	return true, nil
}

// RenderDocsReadme regenerates the marked studies table without writing files.
// The rest of docs/README.md is kept byte-for-byte, including local line endings.
func RenderDocsReadme(content, docsDir, readmePath string) (string, error) {
	return renderDocsReadmeFrom(content, docsDir, readmePath, studySources)
}

// RenderDocsReadmeSnapshot is RenderDocsReadme for a directory holding a
// materialized snapshot, such as the --cached lint's temp dir. It reads the
// studies from that directory and never asks git, so no hook variable can
// redirect it to another repository.
func RenderDocsReadmeSnapshot(content, docsDir, readmePath string) (string, error) {
	return renderDocsReadmeFrom(content, docsDir, readmePath, dirStudySources)
}

func renderDocsReadmeFrom(content, docsDir, readmePath string, list studyLister) (string, error) {
	table, err := studiesTableFrom(docsDir, list)
	if err != nil {
		return "", err
	}
	content = ensureManagedNotice(content, docsStudiesNotice)
	if strings.Contains(content, markdown.MDMarkers.Begin("docs-studies-index")) || strings.Contains(content, markdown.MDMarkers.End("docs-studies-index")) {
		newContent, err := replaceManagedBlock(content, "docs-studies-index", table, -1, -1)
		if err != nil {
			return "", fmt.Errorf("%s: %w", readmePath, err)
		}
		return newContent, nil
	}

	anchorIdx := strings.Index(content, docsStudiesAnchor)
	if anchorIdx == -1 {
		prefix := content
		if !strings.HasSuffix(prefix, "\n") {
			prefix += "\n"
		}
		if !strings.HasSuffix(prefix, "\n\n") {
			prefix += "\n"
		}
		section := docsStudiesAnchor + " — case studies\n\n" + markdown.MDMarkers.Begin("docs-studies-index") + "\n" + strings.TrimRight(table, "\n") + "\n" + markdown.MDMarkers.End("docs-studies-index") + "\n"
		newContent := prefix + section
		return newContent, nil
	}
	headerRel := strings.Index(content[anchorIdx:], docsStudiesTableHead)
	if headerRel == -1 {
		return "", fmt.Errorf("%s: could not find studies table header after %q", readmePath, docsStudiesAnchor)
	}
	headerIdx := anchorIdx + headerRel

	rest := content[headerIdx:]
	endRel := strings.Index(rest, "\n\n")
	blockEnd := len(content)
	if endRel >= 0 {
		blockEnd = headerIdx + endRel
	}

	newContent, err := replaceManagedBlock(content, "docs-studies-index", table, headerIdx, blockEnd)
	if err != nil {
		return "", fmt.Errorf("%s: %w", readmePath, err)
	}
	return newContent, nil
}

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

	"ubunatic.com/harnez/internal/issues"
)

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
		status := f.RawStatus
		if status == "" {
			status = "Unknown"
		}
		fmt.Fprintf(&b, "| %s | [%s](%s) | %s | %s |\n", f.Number, f.RelPath, f.RelPath, title, status)
	}
	return b.String(), nil
}

var issuesTableHeaderRe = regexp.MustCompile(`(?m)^\|\s*#\s*\|`)

// UpdateIssuesReadme regenerates the ticket table in readmePath (normally
// issues/README.md) from the tickets in issuesDir, preserving every line
// before the table verbatim. Returns whether the file's content changed.
func UpdateIssuesReadme(readmePath, issuesDir string) (bool, error) {
	table, err := IssuesTable(issuesDir)
	if err != nil {
		return false, err
	}

	orig, err := os.ReadFile(readmePath)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", readmePath, err)
	}
	content := string(orig)

	loc := issuesTableHeaderRe.FindStringIndex(content)
	if loc == nil {
		return false, fmt.Errorf("%s: could not find issues table header ('| # | File | Title | Status |')", readmePath)
	}

	newContent := content[:loc[0]] + table
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

package issues

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// IssueFile represents an individual issue markdown file on disk.
type IssueFile struct {
	Number       string         // e.g. "036"
	RelPath      string         // e.g. "036-harnez-status-issues-tracker-linter.md" or "archive/001-diff-clean-wrong-path.md"
	Title        string         // e.g. "036 — `harnez status` Issues Tracker Status Linter"
	RawStatus    string         // exact string after "**Status:**", e.g. "Closed — fixed in 444c29f"
	Canonical    StatusCategory // Open, Closed, Draft, Unknown
	HasStatusTag bool
	Body         string // ticket content after the first metadata-closing horizontal rule following the title; empty if none is found (see 158)
}

// TableRow represents a row parsed from issues/README.md.
type TableRow struct {
	LineNumber int
	Number     string // column 1 (e.g. "036")
	LinkText   string // markdown link text (e.g. "036-harnez-status-issues-tracker-linter.md")
	LinkTarget string // markdown link target (e.g. "036-harnez-status-issues-tracker-linter.md")
	Title      string // column 3
	Status     string // column 4
	Canonical  StatusCategory
}

type StatusCategory string

const (
	StatusOpen    StatusCategory = "Open"
	StatusClosed  StatusCategory = "Closed"
	StatusDraft   StatusCategory = "Draft"
	StatusUnknown StatusCategory = "Unknown"
)

type DiagnosticKind string

const (
	DiagStatusMismatch  DiagnosticKind = "status_mismatch"
	DiagUnindexedFile   DiagnosticKind = "unindexed_file"
	DiagBrokenLink      DiagnosticKind = "broken_link"
	DiagMissingStatus   DiagnosticKind = "missing_status_tag"
	DiagMalformedRow    DiagnosticKind = "malformed_table_row"
	DiagDuplicateNumber DiagnosticKind = "duplicate_issue_number"
)

type Diagnostic struct {
	Kind     DiagnosticKind
	IssueNum string
	Path     string
	Message  string
}

type Report struct {
	TotalFiles  int
	TotalRows   int
	Diagnostics []Diagnostic
}

var (
	issueFileRegex   = regexp.MustCompile(`^(\d{3})-.*\.md$`)
	statusLineRegex  = regexp.MustCompile(`(?i)^\s*[-*]?\s*\*\*status:?\*\*:?\s*(.+)$`)
	// tableRowRegex splits a Markdown table row into its four cells. A cell
	// is any run of characters that are neither a bare "|" nor a backslash,
	// or a backslash-escaped character (e.g. "\|" for a literal pipe inside
	// a title) -- so an escaped pipe written by IssuesTable (or by hand)
	// is not mistaken for a column boundary. See issue 239.
	tableRowRegex    = regexp.MustCompile(`^\|((?:\\.|[^|\\])*)\|((?:\\.|[^|\\])*)\|((?:\\.|[^|\\])*)\|((?:\\.|[^|\\])*)\|$`)
	markdownLinkRegx = regexp.MustCompile(`^\[([^\]]+)\]\(([^)]+)\)$`)
	headerLineRegex  = regexp.MustCompile(`^(\s*#\s*)(\d+)(\s*[—–:-].*|\s*)$`)
)

// LeadingLifecycle extracts the leading raw lifecycle token from a free-form
// status string, e.g. "Blocked — waiting for upstream" -> "blocked", "In
// Progress" -> "in progress". It strips common Markdown emphasis characters,
// lowercases, and truncates at the first explanatory-suffix separator, the
// same way CanonicalizeStatus locates its "lead" substring. Used by
// `harnez find status:`/`is:` filters (issue 158), which distinguish between
// raw lifecycle stages that CanonicalizeStatus otherwise collapses into one
// category (Open, In Progress, and Blocked are all StatusOpen).
func LeadingLifecycle(status string) string {
	s := strings.ToLower(status)
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if idx := strings.IndexAny(s, "—–-:,"); idx != -1 {
		s = strings.TrimSpace(s[:idx])
	}
	return s
}

// CanonicalizeStatus normalizes free-form status strings into core categories.
func CanonicalizeStatus(status string) StatusCategory {
	s := strings.ToLower(status)
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.TrimSpace(s)

	if s == "" {
		return StatusUnknown
	}

	// Check leading status prefix first (e.g. "Open — partially fixed")
	lead := s
	if idx := strings.IndexAny(s, "—–-:,"); idx != -1 {
		lead = strings.TrimSpace(s[:idx])
	}
	// Check draft
	for _, kw := range []string{"draft", "wip", "proposal"} {
		if strings.Contains(lead, kw) {
			return StatusDraft
		}
	}
	// Check open
	for _, kw := range []string{"open", "in progress", "blocked", "investigating"} {
		if strings.Contains(lead, kw) {
			return StatusOpen
		}
	}
	// Check closed
	for _, kw := range []string{"closed", "resolved", "fixed", "complete", "completed", "implemented", "invalid"} {
		if strings.Contains(lead, kw) {
			return StatusClosed
		}
	}

	// Fallback to searching anywhere in the full string
	for _, kw := range []string{"draft", "wip", "proposal"} {
		if strings.Contains(s, kw) {
			return StatusDraft
		}
	}
	for _, kw := range []string{"closed", "resolved", "fixed", "complete", "completed", "implemented", "invalid"} {
		if strings.Contains(s, kw) {
			return StatusClosed
		}
	}
	for _, kw := range []string{"open", "in progress", "blocked", "partially"} {
		if strings.Contains(s, kw) {
			return StatusOpen
		}
	}

	return StatusUnknown
}

// ParseIssueFile extracts the declared title and status from an issue markdown document.
func ParseIssueFile(content string) (title string, rawStatus string, hasStatus bool) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if title == "" && strings.HasPrefix(trimmed, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}

		if !hasStatus {
			if matches := statusLineRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
				hasStatus = true
				rawStatus = strings.TrimSpace(matches[1])
			}
		}
	}
	return title, rawStatus, hasStatus
}

// RewriteStatus replaces the value portion of a ticket's "**Status**:" line
// with newStatus, leaving every other line -- including the "**Status**:"
// label text itself, its original leading whitespace/list-bullet prefix,
// and unrelated content such as `[[wikilink]]` references elsewhere in the
// file -- byte-for-byte untouched. It shares statusLineRegex, the same
// anchor ParseIssueFile uses to locate the line, so read and write agree on
// exactly where the Status line is (issue 232). Returns the rewritten
// content and whether it differs from content; an error is returned only
// if no "**Status**:" line is found at all.
func RewriteStatus(content, newStatus string) (string, bool, error) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		loc := statusLineRegex.FindStringSubmatchIndex(line)
		if loc == nil {
			continue
		}
		// loc[2:4] is the span of capture group 1 (the status value) within
		// this exact line -- statusLineRegex tolerates leading whitespace via
		// `^\s*`, so it matches the raw (non-trimmed) line directly.
		valStart, valEnd := loc[2], loc[3]
		newLine := line[:valStart] + newStatus + line[valEnd:]
		if newLine == line {
			return content, false, nil
		}
		lines[i] = newLine
		return strings.Join(lines, "\n"), true, nil
	}
	return "", false, fmt.Errorf("no '**Status**:' line found")
}

// RewriteHeaderNumber replaces the ticket number in the first H1 header line
// (e.g. "# 042 — Title") with newNum, leaving every other line and the rest
// of the header line (title, separator, spacing) untouched.
func RewriteHeaderNumber(content, newNum string) (string, error) {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		loc := headerLineRegex.FindStringSubmatchIndex(line)
		if loc == nil {
			continue
		}
		numStart, numEnd := loc[4], loc[5]
		lines[i] = line[:numStart] + newNum + line[numEnd:]
		return strings.Join(lines, "\n"), nil
	}
	return "", fmt.Errorf("no '# <num> — ...' header line found")
}

// thematicBreakRegex matches a Markdown thematic break ("---", "***", "___",
// optionally space-separated) on its own line, per CommonMark.
var thematicBreakRegex = regexp.MustCompile(`^(-[ \t]*-[ \t]*-[ \t]*(?:-[ \t]*)*|\*[ \t]*\*[ \t]*\*[ \t]*(?:\*[ \t]*)*|_[ \t]*_[ \t]*_[ \t]*(?:_[ \t]*)*)$`)

// ParseBody extracts the ticket body searched by `harnez find` (issue 158,
// extended by issue 162) using a three-tier fallback, in order:
//
//  1. Everything after the first metadata-closing thematic break ("---",
//     "***", or "___" on its own line) that appears after the H1 title.
//  2. Else, everything after the first "## " heading that appears after the
//     H1 title.
//  3. Else, everything after the H1 title itself — the whole remaining
//     document becomes the body.
//
// Returns "" only if no H1 title is found at all.
func ParseBody(content string) string {
	lines := strings.Split(content, "\n")
	titleIdx := -1
	headingIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if titleIdx == -1 {
			if strings.HasPrefix(trimmed, "# ") {
				titleIdx = i
			}
			continue
		}
		if thematicBreakRegex.MatchString(trimmed) {
			return strings.Join(lines[i+1:], "\n")
		}
		if headingIdx == -1 && strings.HasPrefix(trimmed, "## ") {
			headingIdx = i
		}
	}
	if titleIdx == -1 {
		return ""
	}
	if headingIdx != -1 {
		return strings.Join(lines[headingIdx+1:], "\n")
	}
	return strings.Join(lines[titleIdx+1:], "\n")
}

// ticketNumberPrefixRegex strips the leading "NNN <sep>" portion of a parsed
// issue title, e.g. "158 — Add a `harnez find` command" -> "Add a `harnez
// find` command".
var ticketNumberPrefixRegex = regexp.MustCompile(`^\d+\s*[—–:-]\s*`)

// StripTicketNumber removes the leading ticket-number portion of a raw H1
// title, if present.
func StripTicketNumber(title string) string {
	return ticketNumberPrefixRegex.ReplaceAllString(title, "")
}

// mdFormattingReplacer strips common inline Markdown formatting delimiters
// from displayed/searched text without touching the text they wrap.
var mdFormattingReplacer = strings.NewReplacer("`", "", "*", "", "_", "", "#", "")

// PlainTitle renders a raw H1 title (with its leading ticket number) as
// display/search text: the ticket number is removed, Markdown formatting
// delimiters are stripped, and embedded whitespace is collapsed.
func PlainTitle(rawTitle string) string {
	t := StripTicketNumber(rawTitle)
	t = mdFormattingReplacer.Replace(t)
	return strings.Join(strings.Fields(t), " ")
}

// unescapeTableCell reverses the "\|" -> "|" escaping IssuesTable applies
// when writing a cell, so a title containing a literal pipe round-trips
// back to its original text. See issue 239.
func unescapeTableCell(s string) string {
	return strings.ReplaceAll(s, `\|`, `|`)
}

// ParseTrackerTable extracts table entries from issues/README.md.
func ParseTrackerTable(content string) ([]TableRow, error) {
	var rows []TableRow
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	inTable := false

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			if inTable {
				inTable = false
			}
			continue
		}

		matches := tableRowRegex.FindStringSubmatch(line)
		if len(matches) != 5 {
			continue
		}

		c1 := unescapeTableCell(strings.TrimSpace(matches[1]))
		c2 := unescapeTableCell(strings.TrimSpace(matches[2]))
		c3 := unescapeTableCell(strings.TrimSpace(matches[3]))
		c4 := unescapeTableCell(strings.TrimSpace(matches[4]))

		if strings.EqualFold(c1, "#") || strings.HasPrefix(c1, "---") || strings.HasPrefix(c1, ":---") || strings.HasPrefix(c1, "-") {
			inTable = true
			continue
		}

		if !inTable {
			inTable = true
		}

		linkText := c2
		linkTarget := c2
		if lMatches := markdownLinkRegx.FindStringSubmatch(c2); len(lMatches) == 3 {
			linkText = strings.TrimSpace(lMatches[1])
			linkTarget = strings.TrimSpace(lMatches[2])
		}

		num := c1
		if n, err := strconv.Atoi(num); err == nil {
			num = fmt.Sprintf("%03d", n)
		}

		row := TableRow{
			LineNumber: lineNum,
			Number:     num,
			LinkText:   linkText,
			LinkTarget: linkTarget,
			Title:      c3,
			Status:     c4,
			Canonical:  CanonicalizeStatus(c4),
		}
		rows = append(rows, row)
	}

	return rows, nil
}

// Scan walks issuesDir (and its "archive" subdir) collecting IssueFile
// entries for every NNN-*.md file found, sorted by Number then RelPath.
func Scan(issuesDir string) ([]IssueFile, error) {
	return ScanFS(os.DirFS(issuesDir), ".")
}

// ScanFS allows in-memory and embedded fs testing of Scan.
func ScanFS(sysFS fs.FS, root string) ([]IssueFile, error) {
	var issueFiles []IssueFile

	err := fs.WalkDir(sysFS, filepath.ToSlash(root), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)

		if d.IsDir() {
			if relSlash != "." && relSlash != "archive" {
				return fs.SkipDir
			}
			return nil
		}

		base := filepath.Base(path)
		if matches := issueFileRegex.FindStringSubmatch(base); len(matches) > 1 {
			num := matches[1]
			content, err := fs.ReadFile(sysFS, path)
			if err != nil {
				return err
			}
			title, rawStatus, hasStatus := ParseIssueFile(string(content))
			issueFiles = append(issueFiles, IssueFile{
				Number:       num,
				RelPath:      relSlash,
				Title:        title,
				RawStatus:    rawStatus,
				Canonical:    CanonicalizeStatus(rawStatus),
				HasStatusTag: hasStatus,
				Body:         ParseBody(string(content)),
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan issue files: %w", err)
	}

	sort.Slice(issueFiles, func(i, j int) bool {
		if issueFiles[i].Number != issueFiles[j].Number {
			return issueFiles[i].Number < issueFiles[j].Number
		}
		return issueFiles[i].RelPath < issueFiles[j].RelPath
	})

	return issueFiles, nil
}

// Lint scans the issues directory and issues/README.md on disk, returning a report of all discrepancies.
func Lint(issuesDir string) (*Report, error) {
	return LintFS(os.DirFS(issuesDir), ".")
}

// LintFS allows in-memory and embedded fs testing.
func LintFS(sysFS fs.FS, root string) (*Report, error) {
	readmePath := filepath.Join(root, "README.md")
	readmeBytes, err := fs.ReadFile(sysFS, filepath.ToSlash(readmePath))
	if err != nil {
		return nil, fmt.Errorf("read tracker readme %s: %w", readmePath, err)
	}

	tableRows, err := ParseTrackerTable(string(readmeBytes))
	if err != nil {
		return nil, fmt.Errorf("parse tracker table: %w", err)
	}

	issueFiles, err := ScanFS(sysFS, root)
	if err != nil {
		return nil, err
	}

	fileMap := make(map[string]IssueFile)
	numToFileMap := make(map[string][]IssueFile)
	for _, f := range issueFiles {
		fileMap[f.RelPath] = f
		numToFileMap[f.Number] = append(numToFileMap[f.Number], f)
	}

	var diags []Diagnostic

	for _, f := range issueFiles {
		if !f.HasStatusTag {
			diags = append(diags, Diagnostic{
				Kind:     DiagMissingStatus,
				IssueNum: f.Number,
				Path:     f.RelPath,
				Message:  fmt.Sprintf("%s: ticket missing '**Status:**' header", f.RelPath),
			})
		}
	}

	tableNumCount := make(map[string]int)
	tablePathMap := make(map[string]TableRow)

	for _, row := range tableRows {
		tableNumCount[row.Number]++
		cleanTarget := filepath.ToSlash(filepath.Clean(row.LinkTarget))
		tablePathMap[cleanTarget] = row

		file, exists := fileMap[cleanTarget]
		if !exists {
			var alternative string
			if strings.HasPrefix(cleanTarget, "archive/") {
				alt := strings.TrimPrefix(cleanTarget, "archive/")
				if _, ok := fileMap[alt]; ok {
					alternative = alt
				}
			} else {
				alt := "archive/" + cleanTarget
				if _, ok := fileMap[alt]; ok {
					alternative = alt
				}
			}

			if alternative != "" {
				diags = append(diags, Diagnostic{
					Kind:     DiagBrokenLink,
					IssueNum: row.Number,
					Path:     row.LinkTarget,
					Message:  fmt.Sprintf("table links to '%s', but file is located at '%s'", row.LinkTarget, alternative),
				})
			} else {
				diags = append(diags, Diagnostic{
					Kind:     DiagBrokenLink,
					IssueNum: row.Number,
					Path:     row.LinkTarget,
					Message:  fmt.Sprintf("table links to non-existent file '%s'", row.LinkTarget),
				})
			}
		} else {
			if file.HasStatusTag && file.Canonical != row.Canonical {
				diags = append(diags, Diagnostic{
					Kind:     DiagStatusMismatch,
					IssueNum: row.Number,
					Path:     file.RelPath,
					Message:  fmt.Sprintf("%s: ticket says '%s' (%s), table says '%s' (%s)", file.RelPath, file.RawStatus, file.Canonical, row.Status, row.Canonical),
				})
			}
		}
	}

	for num, count := range tableNumCount {
		if count > 1 {
			diags = append(diags, Diagnostic{
				Kind:     DiagDuplicateNumber,
				IssueNum: num,
				Message:  fmt.Sprintf("duplicate issue number %s appears %d times in table", num, count),
			})
		}
	}

	for _, f := range issueFiles {
		if _, indexedByPath := tablePathMap[f.RelPath]; !indexedByPath {
			if tableNumCount[f.Number] == 0 {
				diags = append(diags, Diagnostic{
					Kind:     DiagUnindexedFile,
					IssueNum: f.Number,
					Path:     f.RelPath,
					Message:  fmt.Sprintf("unindexed ticket file '%s' not present in README table", f.RelPath),
				})
			}
		}
	}

	sort.Slice(diags, func(i, j int) bool {
		if diags[i].IssueNum != diags[j].IssueNum {
			return diags[i].IssueNum < diags[j].IssueNum
		}
		return diags[i].Kind < diags[j].Kind
	})

	report := &Report{
		TotalFiles:  len(issueFiles),
		TotalRows:   len(tableRows),
		Diagnostics: diags,
	}

	return report, nil
}

// NextNumber scans issuesDir (and archive) and calculates the next free ticket
// number formatted with at least 3 digits (e.g. "195").
// If the directory has no issues, it returns ("001", 1, nil).
// Otherwise it returns max(allocated)+1.
func NextNumber(issuesDir string) (string, int, error) {
	files, err := Scan(issuesDir)
	if err != nil {
		return "", 0, err
	}
	return NextNumberFromFiles(files), maxNumberFromFiles(files) + 1, nil
}

func maxNumberFromFiles(files []IssueFile) int {
	maxNum := 0
	for _, f := range files {
		if n, err := strconv.Atoi(f.Number); err == nil {
			if n > maxNum {
				maxNum = n
			}
		}
	}
	return maxNum
}

// NextNumberFromFiles computes the next ticket number string formatted with at
// least 3 digits (e.g. "001", "195", "1000") given a slice of scanned issue files.
func NextNumberFromFiles(files []IssueFile) string {
	next := maxNumberFromFiles(files) + 1
	return fmt.Sprintf("%03d", next)
}

var nonAlphanumericSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a raw title into a lowercase kebab-case slug for filenames.
func Slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumericSlugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// ReserveOptions configures ticket reservation.
type ReserveOptions struct {
	Title string // Optional title for the reserved ticket
}

// Reserve allocates the next ticket number and atomically creates a placeholder
// ticket file under issuesDir. If a race occurs (file exists), it retries with the
// next number until successful.
func Reserve(issuesDir string, opts ReserveOptions) (num string, filename string, err error) {
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create issues dir: %w", err)
	}

	for attempts := 0; attempts < 100; attempts++ {
		files, err := Scan(issuesDir)
		if err != nil {
			return "", "", fmt.Errorf("scan issues for reservation: %w", err)
		}
		nextNum := NextNumberFromFiles(files)

		var baseName string
		var titleText string
		slug := Slugify(opts.Title)
		if slug != "" {
			baseName = fmt.Sprintf("%s-%s.md", nextNum, slug)
			titleText = strings.TrimSpace(opts.Title)
		} else {
			baseName = fmt.Sprintf("%s-reserved.md", nextNum)
			titleText = "Reserved"
		}

		content := fmt.Sprintf("# %s — %s\n\n**Status**: Draft\n\n---\n\nReserved placeholder ticket.\n", nextNum, titleText)

		targetPath := filepath.Join(issuesDir, baseName)
		f, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			if os.IsExist(err) {
				// File already exists; retry loop to allocate the next number.
				continue
			}
			return "", "", fmt.Errorf("reserve ticket file %s: %w", targetPath, err)
		}
		if _, err := f.WriteString(content); err != nil {
			_ = f.Close()
			return "", "", fmt.Errorf("write reserved ticket file %s: %w", targetPath, err)
		}
		if err := f.Close(); err != nil {
			return "", "", fmt.Errorf("close reserved ticket file %s: %w", targetPath, err)
		}

		return nextNum, baseName, nil
	}

	return "", "", fmt.Errorf("failed to reserve ticket after multiple attempts due to concurrent collisions")
}


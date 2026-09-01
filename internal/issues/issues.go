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
	tableRowRegex    = regexp.MustCompile(`^\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|$`)
	markdownLinkRegx = regexp.MustCompile(`^\[([^\]]+)\]\(([^)]+)\)$`)
)

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

		c1 := strings.TrimSpace(matches[1])
		c2 := strings.TrimSpace(matches[2])
		c3 := strings.TrimSpace(matches[3])
		c4 := strings.TrimSpace(matches[4])

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

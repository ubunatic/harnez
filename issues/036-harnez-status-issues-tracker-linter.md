# 036 — `harnez status` Issues Tracker Status Linter & Reconciliation

**Status**: Closed (2026-08-19)  
**Category**: Feature / CLI Tooling  
**Related**: [Issue 011: autoDetectDocs non-deterministic order](011-autodetect-nondeterministic-order.md), [Feedback: 2026-08-19](../docs/feedback/2026-08-19-orchestrated-subagents-process-hygiene-and-review-loops.md)

---

## 1. Problem & Context

Issue status is currently maintained in two separate locations in the repository:
1. The individual markdown file's header/frontmatter (`issues/XXX-*.md` and `issues/archive/XXX-*.md`, e.g. `**Status**: Closed`).
2. The central tracking table in `issues/README.md`.

During a recent session, Issue 011 was discovered to have been fixed and tested in code on 2026-07-05 (`444c29f`), but remained listed as `Open` in `issues/README.md` for over a month because the index table is updated manually. Additional manual drift risks include:
- New issue tickets created but not added to `issues/README.md` (unindexed tickets).
- Tickets moved to `issues/archive/` without updating the relative link path in `issues/README.md`.
- Broken links in `issues/README.md` pointing to deleted or renamed markdown files.
- Missing `**Status**` declarations in individual issue tickets (e.g., `005`, `013`).

## 2. Proposed Architecture & CLI Integration

### 2.1 Package Location & Separation of Concerns

Create a dedicated package: `internal/issues/issues.go` (and `internal/issues/issues_test.go`).

Keeping this logic in `internal/issues` rather than directly inside `internal/claude` preserves modularity:
- Can be invoked from `harnez status` when running inside a project root containing an `issues/` directory.
- Can be exposed via a dedicated subcommand `harnez issues [check|lint|sync]` or flags if needed later.
- Avoids bloat in `internal/claude/status.go`.

### 2.2 CLI Integration Points

1. **`harnez status` Integration**:
   - If `issues/README.md` exists in current working dir (or target project root), run `issues.Lint("issues")`.
   - Output summary line under a new `Issues:` section in `harnez status`:
     ```
     Issues:
       tracker:       issues/README.md [27 tickets: 25 ok, 2 drift]
       ! 011-autodetect-nondeterministic-order.md: ticket says 'Closed', table says 'Open'
       ! 005-permissions-grow-only.md: ticket missing '**Status:**' line
     ```
   - If no issues dir exists, the check is cleanly skipped with 0 overhead.

2. **Dedicated Command / Flag**:
   - `harnez issues lint` (or `harnez status --lint-issues`): prints detailed diagnostics and returns non-zero exit code if drift is detected.
   - `harnez issues sync` (or `harnez issues --fix`): automatically updates status column values and adds unindexed issues to `issues/README.md`.

## 3. Data Structures & Function Signatures

```go
package issues

import (
	"io/fs"
	"regexp"
)

// IssueFile represents an individual issue markdown file on disk.
type IssueFile struct {
	Number       string // e.g. "036"
	RelPath      string // e.g. "036-harnez-status-issues-tracker-linter.md" or "archive/001-diff-clean-wrong-path.md"
	Title        string // e.g. "036 — `harnez status` Issues Tracker Status Linter"
	RawStatus    string // exact string after "**Status:**", e.g. "Closed — fixed in 444c29f"
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
	TotalFiles    int
	TotalRows     int
	Diagnostics   []Diagnostic
}

// Core Functions:

// Lint scans the issues directory and issues/README.md, returning a report of all discrepancies.
func Lint(issuesDir string) (*Report, error)

// LintFS allows in-memory and embedded fs testing.
func LintFS(sysFS fs.FS, issuesDir string) (*Report, error)

// ParseIssueFile extracts the declared title and status from an issue markdown document.
func ParseIssueFile(content string) (title string, rawStatus string, hasStatus bool)

// ParseTrackerTable extracts table entries from issues/README.md.
func ParseTrackerTable(content string) ([]TableRow, error)

// CanonicalizeStatus normalizes free-form status strings into core categories for discrepancy comparison.
func CanonicalizeStatus(status string) StatusCategory

// SyncTrackerTable updates issues/README.md content to reconcile table rows with on-disk issue files.
func SyncTrackerTable(existingReadme string, files []IssueFile) (string, error)
```

## 4. Parser & Reconciliation Logic & Edge Cases

### 4.1 Parsing Status in Issue Markdown Files
- **Status Key Variations**:
  - `**Status:** <text>`
  - `**Status**: <text>`
  - `- **Status:** <text>`
  - `* **Status:** <text>`
  - Case-insensitive key matching: `(?i)^\s*[-*]?\s*\*\*status\*\*\s*:\s*(.+)$`
- **Prefix Emojis & Formatting**:
  - Strip markdown formatting and emojis before canonical classification (e.g., `🔴 Open` → `Open`, `**Closed**` → `Closed`).
- **Canonical Status Mapping Rules**:
  - Contains `closed`, `resolved`, `fixed`, `completed`, `implemented`, `invalid` → `StatusClosed`.
  - Contains `draft`, `wip`, `proposal` → `StatusDraft`.
  - Contains `open`, `in progress`, `blocked` → `StatusOpen`.
  - Otherwise → `StatusUnknown`.
- **Status Discrepancy Detection**:
  - Flag if `CanonicalizeStatus(ticket.RawStatus) != CanonicalizeStatus(row.Status)`.
  - (Optional Warning) If canonical statuses match but descriptions significantly drift.

### 4.2 Handling Missing / Archived Files & Links
- **Directory Traversal**: Walk both `issues/` and `issues/archive/` (matching `[0-9]{3}-*.md`).
- **Path Reconciliation**:
  - If a file is in `issues/archive/020-foo.md`, the link in `issues/README.md` must be `[archive/020-foo.md](archive/020-foo.md)` or `[020-foo.md](archive/020-foo.md)`.
  - If `issues/README.md` links to `020-foo.md` but the file was moved to `archive/020-foo.md`, flag as broken link / moved file drift.
- **Unindexed Tickets**:
  - Any `[0-9]{3}-*.md` found on disk not present in `issues/README.md`.
- **Missing Tickets (Ghost rows)**:
  - Any row in `issues/README.md` whose target file does not exist on disk in `issues/` or `issues/archive/`.

### 4.3 Handling Missing `**Status:**` in Tickets
- If an issue file lacks a `**Status:**` declaration (e.g. `005-permissions-grow-only.md`, `013-promote-command.md`), report `DiagMissingStatus` warning so authors keep ticket headers uniform.

### 4.4 Table Parsing & Re-generation Safety
- Locate table bounds in `issues/README.md` (lines matching `| # | File | Title | Status |` down to the next non-table line or EOF).
- When syncing / auto-repairing:
  - Preserve non-table content (preamble, headers, comments) verbatim.
  - Maintain clean markdown column alignment.
  - Sort rows numerically by ticket number (`#`).

## 5. Unit Test Strategy (`internal/issues/issues_test.go`)

1. **`TestParseIssueFile`**:
   - Standard format: `**Status**: Open`
   - Colon inside bold: `**Status:** Closed — fixed in 444c29f`
   - List bullet item: `- **Status:** Completed`
   - Leading emojis: `**Status:** 🔴 Open`
   - Missing status line: file with only title and problem description.
2. **`TestParseTrackerTable`**:
   - Standard table with valid markdown links.
   - Variations in spacing and markdown pipe boundaries.
   - Archive relative links (`archive/020-tools.md`).
3. **`TestCanonicalizeStatus`**:
   - Table of test cases mapping strings to `StatusOpen`, `StatusClosed`, `StatusDraft`, `StatusUnknown`.
4. **`TestLint_Scenarios`** (using `fstest.MapFS`):
   - Perfect sync: all files present, all statuses match.
   - Status mismatch: file says Closed, table says Open.
   - Unindexed ticket: file `039-new.md` on disk, absent in table.
   - Broken link: table lists `099-deleted.md`, file absent on disk.
   - Moved to archive: file in `archive/020.md`, table links `020.md`.
5. **`TestSyncTrackerTable`**:
   - Verify table generation updates mismatched status without clobbering custom notes if configured, or corrects status column cleanly.
   - Verify unindexed ticket is appended in correct numerical sort order.


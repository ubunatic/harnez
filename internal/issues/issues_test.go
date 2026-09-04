package issues

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestParseIssueFile(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantTitle  string
		wantStatus string
		wantHas    bool
	}{
		{
			name: "standard status colon outside bold",
			content: `# 036 — Issues Tracker Status Linter

**Status**: Open
**Category**: Feature
`,
			wantTitle:  "036 — Issues Tracker Status Linter",
			wantStatus: "Open",
			wantHas:    true,
		},
		{
			name: "colon inside bold with comment",
			content: `# 011 — autodetect nondeterministic order

**Status:** Closed — fixed in 444c29f
`,
			wantTitle:  "011 — autodetect nondeterministic order",
			wantStatus: "Closed — fixed in 444c29f",
			wantHas:    true,
		},
		{
			name: "list item bullet",
			content: `# 099 — Some task

- **Status:** Resolved (2026-07-04)
`,
			wantTitle:  "099 — Some task",
			wantStatus: "Resolved (2026-07-04)",
			wantHas:    true,
		},
		{
			name: "status with extra whitespace and emoji",
			content: `# 012 — Demo video

  **Status:**  🔴 Open  
`,
			wantTitle:  "012 — Demo video",
			wantStatus: "🔴 Open",
			wantHas:    true,
		},
		{
			name: "missing status",
			content: `# 005 — Permissions grow only

**Severity:** Medium
## Problem
`,
			wantTitle:  "005 — Permissions grow only",
			wantStatus: "",
			wantHas:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, status, has := ParseIssueFile(tt.content)
			if title != tt.wantTitle {
				t.Errorf("ParseIssueFile() title = %q, want %q", title, tt.wantTitle)
			}
			if status != tt.wantStatus {
				t.Errorf("ParseIssueFile() status = %q, want %q", status, tt.wantStatus)
			}
			if has != tt.wantHas {
				t.Errorf("ParseIssueFile() has = %v, want %v", has, tt.wantHas)
			}
		})
	}
}

func TestParseBody(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "standard metadata rule",
			content: "# 158 — Add find\n\n**Status**: Open\n**Related**: x\n\n---\n\nBody paragraph one.\n\nBody paragraph two.\n",
			want: "\nBody paragraph one.\n\nBody paragraph two.\n",
		},
		{
			name:    "no thematic break present, falls back to ## heading",
			content: "# 154 — repo-status\n\n**Status**: Closed\n\n## Problem\n\nSome text.\n",
			want:    "\nSome text.\n",
		},
		{
			name:    "star-style thematic break",
			content: "# 001 — Title\n\n**Status**: Open\n\n***\n\nStarred body.\n",
			want:    "\nStarred body.\n",
		},
		{
			name:    "no title at all",
			content: "no heading here\n\n---\n\nunreached body\n",
			want:    "",
		},
		{
			name:    "no thematic break and no ## heading, falls back to whole document after title",
			content: "# 000 — init\n\n**Status**: Open\n\nFlat free-text ticket body.\n\nMore free-text.\n",
			want:    "\n**Status**: Open\n\nFlat free-text ticket body.\n\nMore free-text.\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseBody(tt.content); got != tt.want {
				t.Errorf("ParseBody() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStripTicketNumber(t *testing.T) {
	tests := []struct{ in, want string }{
		{"158 — Add a `harnez find` command", "Add a `harnez find` command"},
		{"036 - Issues Tracker Status Linter", "Issues Tracker Status Linter"},
		{"No leading number", "No leading number"},
	}
	for _, tt := range tests {
		if got := StripTicketNumber(tt.in); got != tt.want {
			t.Errorf("StripTicketNumber(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPlainTitle(t *testing.T) {
	tests := []struct{ in, want string }{
		{"158 — Add a `harnez find` command", "Add a harnez find command"},
		{"036 — `harnez status` Issues Tracker  Status   Linter", "harnez status Issues Tracker Status Linter"},
		{"099 — **Bold** and _italic_ title", "Bold and italic title"},
	}
	for _, tt := range tests {
		if got := PlainTitle(tt.in); got != tt.want {
			t.Errorf("PlainTitle(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLeadingLifecycle(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Open", "open"},
		{"In Progress", "in progress"},
		{"Blocked — waiting for upstream", "blocked"},
		{"Closed — resolved in abc123", "closed"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := LeadingLifecycle(tt.in); got != tt.want {
			t.Errorf("LeadingLifecycle(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCanonicalizeStatus(t *testing.T) {
	tests := []struct {
		input string
		want  StatusCategory
	}{
		{"Open", StatusOpen},
		{"🔴 Open", StatusOpen},
		{"in progress", StatusOpen},
		{"Open — partially fixed", StatusOpen},
		{"Blocked on upstream", StatusOpen},
		{"Closed", StatusClosed},
		{"Closed — invalid", StatusClosed},
		{"Resolved — moved to init", StatusClosed},
		{"Fixed in abc1234", StatusClosed},
		{"Complete — extracted to standalone repo", StatusClosed},
		{"Implemented", StatusClosed},
		{"Draft", StatusDraft},
		{"WIP", StatusDraft},
		{"Proposal", StatusDraft},
		{"", StatusUnknown},
		{"Random notes", StatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := CanonicalizeStatus(tt.input)
			if got != tt.want {
				t.Errorf("CanonicalizeStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTrackerTable(t *testing.T) {
	readme := `# Issues Tracker

| # | File | Title | Status |
|---|------|-------|--------|
| 002 | [002-lang-symlink-no-expandhome.md](002-lang-symlink-no-expandhome.md) | lang symlink | Closed — invalid |
| 005 | [005-permissions-grow-only.md](005-permissions-grow-only.md) | permissions | Open |
| 020 | [archive/020-tools-command-os-tools.md](archive/020-tools-command-os-tools.md) | tools command | Closed |
`
	rows, err := ParseTrackerTable(readme)
	if err != nil {
		t.Fatalf("ParseTrackerTable unexpected error: %v", err)
	}

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	if rows[0].Number != "002" || rows[0].LinkTarget != "002-lang-symlink-no-expandhome.md" || rows[0].Canonical != StatusClosed {
		t.Errorf("row 0 mismatch: %+v", rows[0])
	}
	if rows[1].Number != "005" || rows[1].LinkTarget != "005-permissions-grow-only.md" || rows[1].Canonical != StatusOpen {
		t.Errorf("row 1 mismatch: %+v", rows[1])
	}
	if rows[2].Number != "020" || rows[2].LinkTarget != "archive/020-tools-command-os-tools.md" || rows[2].Canonical != StatusClosed {
		t.Errorf("row 2 mismatch: %+v", rows[2])
	}
}

func TestLintFS_Scenarios(t *testing.T) {
	t.Run("all synced", func(t *testing.T) {
		fs := fstest.MapFS{
			"README.md": &fstest.MapFile{
				Data: []byte(`
| # | File | Title | Status |
|---|------|-------|--------|
| 001 | [archive/001-diff.md](archive/001-diff.md) | Diff fix | Closed |
| 002 | [002-symlink.md](002-symlink.md) | Symlink | Open |
`),
			},
			"archive/001-diff.md": &fstest.MapFile{
				Data: []byte("# 001 — Diff\n\n**Status:** Closed\n"),
			},
			"002-symlink.md": &fstest.MapFile{
				Data: []byte("# 002 — Symlink\n\n**Status:** Open\n"),
			},
		}

		report, err := LintFS(fs, ".")
		if err != nil {
			t.Fatalf("LintFS error: %v", err)
		}
		if len(report.Diagnostics) != 0 {
			t.Errorf("expected 0 diagnostics, got %d: %+v", len(report.Diagnostics), report.Diagnostics)
		}
		if report.TotalFiles != 2 || report.TotalRows != 2 {
			t.Errorf("unexpected counts: %+v", report)
		}
	})

	t.Run("drift scenarios", func(t *testing.T) {
		fs := fstest.MapFS{
			"README.md": &fstest.MapFile{
				Data: []byte(`
| # | File | Title | Status |
|---|------|-------|--------|
| 001 | [001-diff.md](001-diff.md) | Diff fix | Closed |
| 002 | [002-symlink.md](002-symlink.md) | Symlink | Open |
| 003 | [003-missing.md](003-missing.md) | Ghost | Open |
`),
			},
			// 001 moved to archive without updating link
			"archive/001-diff.md": &fstest.MapFile{
				Data: []byte("# 001 — Diff\n\n**Status:** Closed\n"),
			},
			// 002 has status mismatch (ticket says Closed, table says Open)
			"002-symlink.md": &fstest.MapFile{
				Data: []byte("# 002 — Symlink\n\n**Status:** Closed\n"),
			},
			// 004 is unindexed and missing status tag
			"004-unindexed.md": &fstest.MapFile{
				Data: []byte("# 004 — Unindexed\n\nNo status tag here\n"),
			},
		}

		report, err := LintFS(fs, ".")
		if err != nil {
			t.Fatalf("LintFS error: %v", err)
		}

		// Expected diagnostics:
		// 001: broken link (moved to archive)
		// 002: status mismatch
		// 003: broken link (non-existent file)
		// 004: missing status tag
		// 004: unindexed file
		if len(report.Diagnostics) != 5 {
			t.Fatalf("expected 5 diagnostics, got %d: %+v", len(report.Diagnostics), report.Diagnostics)
		}

		kinds := make(map[DiagnosticKind]int)
		for _, d := range report.Diagnostics {
			kinds[d.Kind]++
		}

		if kinds[DiagBrokenLink] != 2 {
			t.Errorf("expected 2 broken links, got %d", kinds[DiagBrokenLink])
		}
		if kinds[DiagStatusMismatch] != 1 {
			t.Errorf("expected 1 status mismatch, got %d", kinds[DiagStatusMismatch])
		}
		if kinds[DiagMissingStatus] != 1 {
			t.Errorf("expected 1 missing status, got %d", kinds[DiagMissingStatus])
		}
		if kinds[DiagUnindexedFile] != 1 {
			t.Errorf("expected 1 unindexed file, got %d", kinds[DiagUnindexedFile])
		}
	})
}

func TestNextNumber(t *testing.T) {
	t.Run("empty directory returns 001", func(t *testing.T) {
		fs := fstest.MapFS{}
		files, err := ScanFS(fs, ".")
		if err != nil {
			t.Fatalf("ScanFS error: %v", err)
		}
		got := NextNumberFromFiles(files)
		if got != "001" {
			t.Errorf("NextNumberFromFiles() = %q, want %q", got, "001")
		}
	})

	t.Run("max plus one with gaps and archive", func(t *testing.T) {
		fs := fstest.MapFS{
			"001-first.md":         &fstest.MapFile{Data: []byte("# 001 — First\n\n**Status:** Closed\n")},
			"005-gap.md":           &fstest.MapFile{Data: []byte("# 005 — Gap\n\n**Status:** Open\n")},
			"archive/010-arch.md":  &fstest.MapFile{Data: []byte("# 010 — Arch\n\n**Status:** Closed\n")},
			"099-near-hundred.md":  &fstest.MapFile{Data: []byte("# 099 — Near Hundred\n\n**Status:** Open\n")},
		}
		files, err := ScanFS(fs, ".")
		if err != nil {
			t.Fatalf("ScanFS error: %v", err)
		}
		got := NextNumberFromFiles(files)
		if got != "100" {
			t.Errorf("NextNumberFromFiles() = %q, want %q", got, "100")
		}
	})

	t.Run("large numbers 1000+", func(t *testing.T) {
		fs := fstest.MapFS{
			"999-end.md": &fstest.MapFile{Data: []byte("# 999 — End\n\n**Status:** Open\n")},
		}
		files, err := ScanFS(fs, ".")
		if err != nil {
			t.Fatalf("ScanFS error: %v", err)
		}
		got := NextNumberFromFiles(files)
		if got != "1000" {
			t.Errorf("NextNumberFromFiles() = %q, want %q", got, "1000")
		}
	})
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"Simple Title", "simple-title"},
		{"`harnez find` Issues: Next & Reserve!", "harnez-find-issues-next-reserve"},
		{"---Already-Kebab---", "already-kebab"},
		{"Multiple   Spaces   and --- dashes", "multiple-spaces-and-dashes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := Slugify(tt.input); got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestReserve_AtomicAndCollisionAvoidance(t *testing.T) {
	dir := t.TempDir()

	// 1. Initial reservation on empty directory
	num1, file1, err := Reserve(dir, ReserveOptions{})
	if err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}
	if num1 != "001" || file1 != "001-reserved.md" {
		t.Fatalf("Reserve on empty dir got %q, %q; want 001, 001-reserved.md", num1, file1)
	}

	// Verify content of reserved file
	content1, err := os.ReadFile(filepath.Join(dir, file1))
	if err != nil {
		t.Fatalf("ReadFile %s: %v", file1, err)
	}
	title1, status1, has1 := ParseIssueFile(string(content1))
	if !has1 || status1 != "Draft" || title1 != "001 — Reserved" {
		t.Errorf("reserved file metadata mismatch: title=%q, status=%q, has=%v", title1, status1, has1)
	}

	// 2. Second reservation with custom title
	num2, file2, err := Reserve(dir, ReserveOptions{Title: "Fix load balancer"})
	if err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}
	if num2 != "002" || file2 != "002-fix-load-balancer.md" {
		t.Fatalf("Reserve got %q, %q; want 002, 002-fix-load-balancer.md", num2, file2)
	}

	// 3. Sequential third reservation
	num3, file3, err := Reserve(dir, ReserveOptions{})
	if err != nil {
		t.Fatalf("Reserve failed: %v", err)
	}
	if num3 != "003" || file3 != "003-reserved.md" {
		t.Fatalf("Reserve got %q, %q; want 003, 003-reserved.md", num3, file3)
	}
}

// TestRewriteStatus covers issue 232's requirement that rewriting the
// Status line must not disturb surrounding content, including
// [[wikilink]]-style references and the label text/prefix around the
// value itself.
func TestRewriteStatus(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		newStatus   string
		wantContent string
		wantChanged bool
		wantErr     bool
	}{
		{
			name:        "standard bold status line, preserves rest of file",
			content:     "# 042 — Example\n\n**Status**: Open\n**Priority**: P2 (Medium)\n**Related**: [[041-other-ticket]]\n",
			newStatus:   "Closed — resolved",
			wantContent: "# 042 — Example\n\n**Status**: Closed — resolved\n**Priority**: P2 (Medium)\n**Related**: [[041-other-ticket]]\n",
			wantChanged: true,
		},
		{
			name:        "status line inside body text is untouched, only header line rewritten",
			content:     "**Status**: Open\n\n---\n\nSee also: mentions of \"status\" and **Status** later in prose, not a header.\n",
			newStatus:   "Draft",
			wantContent: "**Status**: Draft\n\n---\n\nSee also: mentions of \"status\" and **Status** later in prose, not a header.\n",
			wantChanged: true,
		},
		{
			name:        "identical new status is a no-op",
			content:     "**Status**: Open\n**Category**: Bug\n",
			newStatus:   "Open",
			wantContent: "**Status**: Open\n**Category**: Bug\n",
			wantChanged: false,
		},
		{
			name:      "no status line present is an error",
			content:   "# 042 — Example\n\nNo status header here.\n",
			newStatus: "Closed",
			wantErr:   true,
		},
		{
			name:        "leading list-bullet prefix preserved",
			content:     "- **Status**: In Progress\n",
			newStatus:   "Blocked — waiting on upstream",
			wantContent: "- **Status**: Blocked — waiting on upstream\n",
			wantChanged: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, err := RewriteStatus(tc.content, tc.newStatus)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got content: %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if changed != tc.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tc.wantChanged)
			}
			if got != tc.wantContent {
				t.Errorf("content mismatch:\ngot:  %q\nwant: %q", got, tc.wantContent)
			}
		})
	}
}


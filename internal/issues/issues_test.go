package issues

import (
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

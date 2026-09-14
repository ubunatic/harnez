// export.go implements issue 204's sanitized JSON export of tool_calls
// telemetry for external visualization tools (e.g. ubunatic.com
// dashboards). It transforms already-loaded ToolCall rows into a scrubbed,
// dataviz-friendly shape — no path, account, or hostname data survives in
// the output at the default privacy level.
//
// Scope note (issue 204, first pass): JSON only. A SQLite export format is
// deliberately deferred — see the ticket's "Progress / Scope Note" section.
//
// Scope note (issue 204, privacy-levels pass): BuildExport/ExportAll keep
// their original two-arg signatures and now mean exactly "Level 1 /
// public" — every existing caller/test keeps its exact prior behavior
// unmodified. BuildExportLevel/ExportAllLevel are the new entry points
// that take an explicit privacy.Level (see internal/privacy) and, for
// LevelAgentSanitized, a privacy.NoteSanitizer.
//
// Scope note (issue 212): ExportToolCall includes ActivityCategory
// ("activity_category"), populated via the multi-tier taxonomic classification
// pipeline. ActivityCategory is safe across all privacy levels (including public),
// as it is a closed enum never leaking raw text.
package telemetry

import (
	"context"
	"path/filepath"
	"time"

	"ubunatic.com/harnez/internal/privacy"
)

// ExportToolCall is one scrubbed tool_calls row, safe for external
// publication at the default privacy level. Compared to ToolCall:
//   - WorkingDir is dropped entirely and replaced by ProjectDir, which
//     holds only the final path component (e.g. "harnez", never
//     "/home/uwe/projects/harnez") — see normalizeProjectPath.
//   - ProjectName and TicketID are run through the same normalization,
//     in case either one was ever populated with an absolute path rather
//     than a short identifier (issue 204's explicit requirement).
//   - SessionID is kept as-is: it is a resolve.Session()-produced opaque
//     identifier (see internal/resolve), not derived from username,
//     hostname, or email, so it carries no PII on its own. It is still
//     worth keeping an eye on if resolve.Session's derivation ever
//     changes to embed anything identifying.
//   - Note's presence depends on the privacy.Level passed to
//     BuildExportLevel: dropped at LevelPublic (the default — free text
//     has no safe automatic scrub, so it is omitted rather than risk a
//     leak), LLM-sanitized at LevelAgentSanitized, regex-scrubbed in
//     place at LevelInternal (see privacy.ScrubText), and passed through
//     verbatim at LevelRaw.
//   - ActivityCategory (issue 212) categorizes the action taken into a
//     canonical closed enum ("test", "build", "edit", "inspection", "git",
//     "debug", "workflow", "config", "other") safe for public dashboards.
type ExportToolCall struct {
	CreatedAt        time.Time        `json:"created_at"`
	SessionID        string           `json:"session_id,omitempty"`
	TicketID         string           `json:"ticket_id,omitempty"`
	ProjectName      string           `json:"project_name,omitempty"`
	ProjectDir       string           `json:"project_dir,omitempty"`
	AgentID          string           `json:"agent_id"`
	ToolName         string           `json:"tool_name"`
	CallType         string           `json:"call_type"`
	Score            *int             `json:"score,omitempty"`
	Note             string           `json:"note,omitempty"`
	ActivityCategory ActivityCategory `json:"activity_category"`
	ExitCode         *int             `json:"exit_code,omitempty"`
	DurationMs       int64            `json:"duration_ms"`
	RawBytes         int64            `json:"raw_bytes"`
	DistilledBytes   *int64           `json:"distilled_bytes,omitempty"`
}

// Export is the top-level JSON payload for `harnez usage export`'s
// telemetry half.
type Export struct {
	GeneratedAt time.Time        `json:"generated_at"`
	ToolCalls   []ExportToolCall `json:"tool_calls"`
}

// normalizeProjectPath reduces any path-shaped identifier down to its
// final component, so an absolute path never survives into an export
// (issue 204). A value that is already a short identifier (e.g. a ticket
// number "204" or a bare project name "harnez") passes through unchanged,
// since filepath.Base of a value with no path separators is that value
// itself.
func normalizeProjectPath(p string) string {
	if p == "" {
		return ""
	}
	return filepath.Base(p)
}

// BuildExport transforms rows into the scrubbed Export payload at
// privacy.LevelPublic — the original, pre-issue-204-v2 behavior. Existing
// callers/tests are unaffected by the addition of privacy levels.
func BuildExport(rows []ToolCall, now time.Time) Export {
	return BuildExportLevel(rows, now, privacy.LevelPublic, nil)
}

// BuildExportLevel is BuildExport's privacy-level-aware form. sanitized is
// only consulted at privacy.LevelAgentSanitized: it must map every
// non-empty ToolCall.Note appearing in rows to its already-sanitized text
// (see SanitizeNotes) — a note with no entry in sanitized is treated as
// unsanitized and dropped rather than leaked raw.
// Categories for each row are classified via Tier 1 deterministic rules
// if not already provided.
func BuildExportLevel(rows []ToolCall, now time.Time, level privacy.Level, sanitized map[string]string) Export {
	return BuildExportWithCategories(rows, now, level, sanitized, nil)
}

// BuildExportWithCategories allows passing pre-classified ActivityCategory for rows.
// If categories is nil or shorter than rows, missing entries are classified via Tier 1 rules.
func BuildExportWithCategories(rows []ToolCall, now time.Time, level privacy.Level, sanitized map[string]string, categories []ActivityCategory) Export {
	out := Export{GeneratedAt: now}
	for i, r := range rows {
		cat := CategoryOther
		if i < len(categories) && categories[i].IsValid() {
			cat = categories[i]
		} else {
			cat = ClassifyTier1(r.ToolName, r.Note, r.ExitCode)
		}

		etc := ExportToolCall{
			CreatedAt:        r.CreatedAt,
			SessionID:        r.SessionID,
			TicketID:         normalizeProjectPath(r.TicketID),
			ProjectName:      normalizeProjectPath(r.ProjectName),
			ProjectDir:       normalizeProjectPath(r.WorkingDir),
			AgentID:          r.AgentID,
			ToolName:         CanonicalToolName(r.ToolName),
			CallType:         r.CallType,
			Score:            r.Score,
			ActivityCategory: cat,
			ExitCode:         r.ExitCode,
			DurationMs:       r.DurationMs,
			RawBytes:         r.RawBytes,
			DistilledBytes:   r.DistilledBytes,
		}
		switch level {
		case privacy.LevelPublic:
			// Note stays "" (dropped).
		case privacy.LevelAgentSanitized:
			if r.Note != "" {
				if clean, ok := sanitized[r.Note]; ok {
					etc.Note = clean
				}
				// No cache/sanitizer entry for this note: leave it
				// unset rather than fall back to raw text.
			}
		case privacy.LevelInternal:
			etc.Note = privacy.ScrubText(r.Note)
		case privacy.LevelRaw:
			etc.Note = r.Note
		}
		out.ToolCalls = append(out.ToolCalls, etc)
	}
	return out
}

// ExportAll queries every tool_calls row (Filter{}, unfiltered) from db
// and returns the scrubbed Export payload at privacy.LevelPublic. Existing
// callers/tests are unaffected by the addition of privacy levels.
func ExportAll(db *DB, now time.Time) (Export, error) {
	return ExportAllLevel(context.Background(), db, now, privacy.LevelPublic, nil)
}

// ExportAllLevel is ExportAll's privacy-level-aware form. At
// privacy.LevelAgentSanitized, sanitizer must be non-nil whenever any row
// has a non-empty, not-yet-cached Note (see SanitizeNotes); at every other
// level sanitizer is unused and may be nil.
func ExportAllLevel(ctx context.Context, db *DB, now time.Time, level privacy.Level, sanitizer privacy.NoteSanitizer) (Export, error) {
	return ExportAllWithClassifier(ctx, db, now, level, sanitizer, nil)
}

// ExportAllWithClassifier performs full export with opt-in batch classifier
// for Tier 3 resolution of unmatched notes.
func ExportAllWithClassifier(ctx context.Context, db *DB, now time.Time, level privacy.Level, sanitizer privacy.NoteSanitizer, classifier NoteBatchClassifier) (Export, error) {
	rows, err := db.Query(Filter{})
	if err != nil {
		return Export{}, err
	}

	var sanitized map[string]string
	if level == privacy.LevelAgentSanitized {
		sanitized, err = SanitizeNotes(ctx, db, sanitizer, distinctNotes(rows), now)
		if err != nil {
			return Export{}, err
		}
	}

	categories, err := ClassifyNotes(ctx, db, rows, classifier, now)
	if err != nil {
		return Export{}, err
	}

	return BuildExportWithCategories(rows, now, level, sanitized, categories), nil
}

// distinctNotes collects every non-empty ToolCall.Note across rows for
// SanitizeNotes to resolve. SanitizeNotes itself also dedupes, so
// duplicates here are harmless, just slightly wasteful — kept simple.
func distinctNotes(rows []ToolCall) []string {
	notes := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Note != "" {
			notes = append(notes, r.Note)
		}
	}
	return notes
}

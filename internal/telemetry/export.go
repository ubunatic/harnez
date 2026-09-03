// export.go implements issue 204's sanitized JSON export of tool_calls
// telemetry for external visualization tools (e.g. ubunatic.com
// dashboards). It transforms already-loaded ToolCall rows into a scrubbed,
// dataviz-friendly shape — no path, account, or hostname data survives in
// the output.
//
// Scope note (issue 204, first pass): JSON only. A SQLite export format is
// deliberately deferred — see the ticket's "Progress / Scope Note" section.
package telemetry

import (
	"path/filepath"
	"time"
)

// ExportToolCall is one scrubbed tool_calls row, safe for external
// publication. Compared to ToolCall:
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
//   - Note is dropped: it is free-text written by whatever called
//     `harnez rate`/`harnez exec` and could contain anything, including
//     paths or account details typed by a human — there is no safe way
//     to scrub free text automatically, so it is omitted rather than
//     risk a leak.
type ExportToolCall struct {
	CreatedAt      time.Time `json:"created_at"`
	SessionID      string    `json:"session_id,omitempty"`
	TicketID       string    `json:"ticket_id,omitempty"`
	ProjectName    string    `json:"project_name,omitempty"`
	ProjectDir     string    `json:"project_dir,omitempty"`
	AgentID        string    `json:"agent_id"`
	ToolName       string    `json:"tool_name"`
	CallType       string    `json:"call_type"`
	Score          *int      `json:"score,omitempty"`
	ExitCode       *int      `json:"exit_code,omitempty"`
	DurationMs     int64     `json:"duration_ms"`
	RawBytes       int64     `json:"raw_bytes"`
	DistilledBytes *int64    `json:"distilled_bytes,omitempty"`
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

// BuildExport transforms rows (as returned by DB.Query) into the scrubbed
// Export payload. It is a pure function over its input so it can be unit
// tested without a real database (see export_test.go).
func BuildExport(rows []ToolCall, now time.Time) Export {
	out := Export{GeneratedAt: now}
	for _, r := range rows {
		out.ToolCalls = append(out.ToolCalls, ExportToolCall{
			CreatedAt:      r.CreatedAt,
			SessionID:      r.SessionID,
			TicketID:       normalizeProjectPath(r.TicketID),
			ProjectName:    normalizeProjectPath(r.ProjectName),
			ProjectDir:     normalizeProjectPath(r.WorkingDir),
			AgentID:        r.AgentID,
			ToolName:       r.ToolName,
			CallType:       r.CallType,
			Score:          r.Score,
			ExitCode:       r.ExitCode,
			DurationMs:     r.DurationMs,
			RawBytes:       r.RawBytes,
			DistilledBytes: r.DistilledBytes,
		})
	}
	return out
}

// ExportAll queries every tool_calls row (Filter{}, unfiltered) from db and
// returns the scrubbed Export payload.
func ExportAll(db *DB, now time.Time) (Export, error) {
	rows, err := db.Query(Filter{})
	if err != nil {
		return Export{}, err
	}
	return BuildExport(rows, now), nil
}

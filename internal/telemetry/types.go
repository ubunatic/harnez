package telemetry

import "time"

// ToolCall is one row of the tool_calls table. Field set mirrors the
// columns in schemaDDL (schema.go) exactly — see that file's comment for
// why the shape lives there, not here.
type ToolCall struct {
	ID             int64 // assigned by SQLite (AUTOINCREMENT); ignored on Insert
	CreatedAt      time.Time
	SessionID      string
	TicketID       string
	ProjectName    string
	WorkingDir     string
	AgentID        string
	ToolName       string
	CallType       string // e.g. "internal" (117) or "shell" (118)
	Score          *int   // 1-5, optional; range enforced by the DB CHECK constraint
	Note           string
	ExitCode       *int // optional; nil for call types without a process exit code
	DurationMs     int64
	RawBytes       int64
	DistilledBytes int64
}

package telemetry

import "time"

// ToolCall is one row of the tool_calls table. Field set mirrors the
// columns in schemaDDL (schema.go) exactly — see that file's comment for
// why the shape lives there, not here.
type ToolCall struct {
	ID                     int64 // assigned by SQLite (AUTOINCREMENT); ignored on Insert
	CreatedAt              time.Time
	SessionID              string
	TicketID               string
	ProjectName            string
	WorkingDir             string
	AgentID                string
	ToolName               string
	CallType               string // e.g. "internal" (117) or "shell" (118)
	Score                  *int   // 1-5, optional; range enforced by the DB CHECK constraint
	Note                   string
	ExitCode               *int // optional; nil for call types without a process exit code
	DurationMs             int64
	RawBytes               int64
	DistilledBytes         *int64 // populated only when distillation ran; nil (SQL NULL) otherwise
	OutputBytes            *int64 `json:"output_bytes,omitempty"`
	ActualTokens           *int64 `json:"actual_tokens,omitempty"`
	InputTokens            *int64 `json:"input_tokens,omitempty"`
	CachedInputTokens      *int64 `json:"cached_input_tokens,omitempty"`
	OutputTokens           *int64 `json:"output_tokens,omitempty"`
	ReasoningTokens        *int64 `json:"reasoning_tokens,omitempty"`
	TotalTokens            *int64 `json:"total_tokens,omitempty"`
	PotentialSavingsTokens *int64 `json:"potential_savings_tokens,omitempty"`
	PotentialSavingsBytes  *int64 `json:"potential_savings_bytes,omitempty"`
}

// CLIInvocation is one row of the cli_invocations table (issue 326): a
// single `harnez <subcommand>` run. Field set mirrors the columns in
// schemaDDL (schema.go) exactly, same rule as ToolCall above.
//
// Unlike ToolCall this carries json tags: `harnez log --json` (issue 327)
// emits the row slice directly, so the wire names are part of that
// command's contract rather than an accident of Go field naming.
type CLIInvocation struct {
	ID            int64     `json:"id"` // assigned by SQLite (AUTOINCREMENT); ignored on insert
	CreatedAt     time.Time `json:"created_at"`
	SessionID     string    `json:"session_id"`
	AgentID       string    `json:"agent_id"` // issue 328's vocabulary: "agent:<id>" / "human" / "unknown"
	Command       string    `json:"command"`  // full cobra path without the root, e.g. "issues new"
	Args          string    `json:"args"`     // redacted at write time; never raw argv
	ProjectName   string    `json:"project_name"`
	WorkingDir    string    `json:"working_dir"`
	TicketID      string    `json:"ticket_id"`
	ExitCode      *int      `json:"exit_code"` // nil only for a row whose command never returned (not written today)
	DurationMs    int64     `json:"duration_ms"`
	HarnezVersion string    `json:"harnez_version"`
}

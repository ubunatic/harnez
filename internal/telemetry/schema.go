package telemetry

// schemaDDL is the single source of truth for the tool_calls table shape
// (per docs/other/Spec.md's "spec files are the single source of truth"
// principle, applied here to SQL DDL rather than a YAML spec file — Go
// code in this package must not hardcode a parallel copy of the field
// list or constraints below; it reads/writes generically off this shape
// and lets SQLite itself enforce constraints like the score range at
// write time). Idempotent: CREATE TABLE/INDEX IF NOT EXISTS, run on every
// Open — no migration framework, per this repo's "just change the code"
// bias (docs/other/Spec.md / docs/lang/Go.md).
const schemaDDL = `
CREATE TABLE IF NOT EXISTS tool_calls (
	id              INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at      TEXT    NOT NULL,
	session_id      TEXT    NOT NULL,
	ticket_id       TEXT    NOT NULL DEFAULT '',
	project_name    TEXT    NOT NULL DEFAULT '',
	working_dir     TEXT    NOT NULL DEFAULT '',
	agent_id        TEXT    NOT NULL,
	tool_name       TEXT    NOT NULL,
	call_type       TEXT    NOT NULL,
	score           INTEGER CHECK (score IS NULL OR (score BETWEEN 1 AND 5)),
	note            TEXT    NOT NULL DEFAULT '',
	exit_code       INTEGER,
	duration_ms     INTEGER NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
	raw_bytes       INTEGER NOT NULL DEFAULT 0 CHECK (raw_bytes >= 0),
	distilled_bytes INTEGER CHECK (distilled_bytes IS NULL OR distilled_bytes >= 0)
);

CREATE INDEX IF NOT EXISTS idx_tool_calls_session_id  ON tool_calls (session_id);
CREATE INDEX IF NOT EXISTS idx_tool_calls_ticket_id   ON tool_calls (ticket_id);
CREATE INDEX IF NOT EXISTS idx_tool_calls_tool_name   ON tool_calls (tool_name);
CREATE INDEX IF NOT EXISTS idx_tool_calls_agent_id    ON tool_calls (agent_id);
CREATE INDEX IF NOT EXISTS idx_tool_calls_created_at  ON tool_calls (created_at);
`

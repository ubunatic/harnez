package telemetry

// schemaVersion is bumped whenever schemaDDL's shape changes in a way
// CREATE TABLE/INDEX IF NOT EXISTS can't apply to an already-existing
// file (e.g. issue 118's distilled_bytes NOT NULL -> nullable change,
// which silently did nothing against a pre-existing db until the file
// was manually deleted). Stored via SQLite's built-in PRAGMA user_version
// (see Open in telemetry.go) rather than a metadata table, so there's
// nothing to create/migrate for this check itself. This is deliberately
// NOT a migration framework — still no migration framework, per this
// repo's "just change the code" bias — it only turns a confusing runtime
// constraint error (or worse, a silent schema mismatch) into one clear
// message telling the user to delete the file, since a single-user local
// telemetry cache has nothing worth an automated migration path for.
//
// 1: issue 116's original shape (distilled_bytes INTEGER NOT NULL DEFAULT 0).
// 2: issue 118's fix (distilled_bytes made nullable) — the current shape.
const schemaVersion = 2

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

-- note_sanitization_cache backs issue 204's Level 2 (agent-sanitized)
-- export privacy level: sha256(raw note text) -> LLM-sanitized text, so a
-- given note is only ever sent to the NoteSanitizer once, no matter how
-- many export runs reference it. This is purely additive relative to
-- schemaVersion 2's shape (a new table, no change to tool_calls itself),
-- so it does not require a schemaVersion bump — CREATE TABLE IF NOT
-- EXISTS already applies it to a pre-existing database file on next Open.
CREATE TABLE IF NOT EXISTS note_sanitization_cache (
	raw_hash    TEXT PRIMARY KEY,
	clean_text  TEXT NOT NULL,
	created_at  TEXT NOT NULL
);

-- note_category_cache backs issue 212's Tier 2 persistent content-hash cache:
-- sha256(raw note text) -> activity_category enum string, avoiding re-evaluating
-- seen notes. Purely additive table (no schemaVersion bump needed).
CREATE TABLE IF NOT EXISTS note_category_cache (
	raw_hash    TEXT PRIMARY KEY,
	category    TEXT NOT NULL,
	created_at  TEXT NOT NULL
);
`

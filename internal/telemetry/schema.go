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
// 2: issue 118's fix (distilled_bytes made nullable).
// 3-4: additive provider token columns used by Codex telemetry.
// 5: additive compaction_events and session_boundaries tables.
// 6: ordered per-turn and cumulative provider token snapshots.
// 7: versioned pricing inputs and immutable compaction economics results.
const schemaVersion = 7

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
	distilled_bytes INTEGER CHECK (distilled_bytes IS NULL OR distilled_bytes >= 0),
	output_bytes INTEGER,
	actual_tokens INTEGER,
	input_tokens INTEGER,
	cached_input_tokens INTEGER,
	output_tokens INTEGER,
	reasoning_tokens INTEGER,
	total_tokens INTEGER,
	potential_savings_tokens INTEGER,
	potential_savings_bytes INTEGER
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

-- issue_status_snapshots backs issue 228: an append-only history of
-- open/closed/draft/unknown ticket counts per project, one row per
-- 'harnez index' run whose counts differ from that project's most recent
-- prior row (see InsertIssueSnapshot's dedupe check in issuesnapshot.go --
-- the dedupe itself is a Go-side read-then-compare, not a DB constraint,
-- since "identical to the latest row for this project" isn't expressible
-- as a single-row UNIQUE/CHECK constraint). Purely additive table (no
-- schemaVersion bump needed), reusing this same telemetry DB rather than a
-- separate issues/.status-history.jsonl file so all of this repo's SQL/
-- storage-format ownership stays in one place, per issue 120's "keep SQL
-- out of the CLI" convention.
CREATE TABLE IF NOT EXISTS issue_status_snapshots (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at    TEXT    NOT NULL,
	project_name  TEXT    NOT NULL,
	open_count    INTEGER NOT NULL DEFAULT 0 CHECK (open_count >= 0),
	closed_count  INTEGER NOT NULL DEFAULT 0 CHECK (closed_count >= 0),
	draft_count   INTEGER NOT NULL DEFAULT 0 CHECK (draft_count >= 0),
	unknown_count INTEGER NOT NULL DEFAULT 0 CHECK (unknown_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_issue_status_snapshots_project     ON issue_status_snapshots (project_name);
CREATE INDEX IF NOT EXISTS idx_issue_status_snapshots_created_at  ON issue_status_snapshots (created_at);

-- cli_invocations backs issue 326: one row per harnez CLI invocation --
-- which subcommand ran, when, in which project, by whom, and whether it
-- succeeded -- written from main() around root.Execute() (cmd/harnez's
-- executeAndRecord) and read back by harnez log (issue 327). It is
-- deliberately a separate table from tool_calls rather than a fifth
-- call_type on it: tool_calls rows are *deliberate* agent-authored
-- telemetry (a rating, a wrapped shell command) whose aggregates
-- (GroupStats.FailureCount, UnratedFailureCount, the rate-overhead report)
-- are tuned around that assumption. Folding an automatic per-invocation
-- row into the same table would silently change every one of those numbers.
--
-- Purely additive table (no schemaVersion bump needed) -- CREATE TABLE IF
-- NOT EXISTS applies it to a pre-existing database file on next Open, same
-- reasoning as the three tables above.
--
-- args is redacted at write time (see cmd/harnez/clilog.go's redactArgs):
-- flag names are kept, flag values and unsafe-looking positionals are
-- replaced with a placeholder. Raw argv is never stored -- it can carry
-- absolute paths, hostnames (usage --host), and ticket titles.
CREATE TABLE IF NOT EXISTS cli_invocations (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at     TEXT    NOT NULL,
	session_id     TEXT    NOT NULL,
	agent_id       TEXT    NOT NULL,
	command        TEXT    NOT NULL,
	args           TEXT    NOT NULL DEFAULT '',
	project_name   TEXT    NOT NULL DEFAULT '',
	working_dir    TEXT    NOT NULL DEFAULT '',
	ticket_id      TEXT    NOT NULL DEFAULT '',
	exit_code      INTEGER,
	duration_ms    INTEGER NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
	harnez_version TEXT    NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_cli_invocations_created_at ON cli_invocations (created_at);
CREATE INDEX IF NOT EXISTS idx_cli_invocations_project    ON cli_invocations (project_name);
CREATE INDEX IF NOT EXISTS idx_cli_invocations_session_id ON cli_invocations (session_id);
CREATE INDEX IF NOT EXISTS idx_cli_invocations_command    ON cli_invocations (command);

-- compaction_events stores the provider-visible boundary and optional token
-- snapshot for Codex PreCompact/PostCompact hooks (issue 423).
CREATE TABLE IF NOT EXISTS compaction_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at TEXT NOT NULL,
	session_id TEXT NOT NULL,
	event_type TEXT NOT NULL,
	turn_id TEXT NOT NULL DEFAULT '',
	trigger TEXT NOT NULL DEFAULT '',
	reason TEXT NOT NULL DEFAULT '',
	input_tokens INTEGER,
	cached_input_tokens INTEGER,
	output_tokens INTEGER,
	reasoning_tokens INTEGER,
	total_tokens INTEGER
);
CREATE INDEX IF NOT EXISTS idx_compaction_events_session_id ON compaction_events (session_id);
CREATE INDEX IF NOT EXISTS idx_compaction_events_created_at ON compaction_events (created_at);

-- session_boundaries records lifecycle and compaction boundaries independently
-- of tool_calls so sessions remain queryable when no tool call was emitted.
CREATE TABLE IF NOT EXISTS session_boundaries (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at TEXT NOT NULL,
	session_id TEXT NOT NULL,
	boundary_type TEXT NOT NULL,
	compaction_event_id INTEGER
);
CREATE INDEX IF NOT EXISTS idx_session_boundaries_session_id ON session_boundaries (session_id);
CREATE INDEX IF NOT EXISTS idx_session_boundaries_created_at ON session_boundaries (created_at);

-- token_snapshots preserves every provider snapshot observed around a
-- boundary. This is append-only so a later tool event cannot overwrite the
-- compaction or session-exit evidence needed for reconciliation.
CREATE TABLE IF NOT EXISTS token_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at TEXT NOT NULL,
	session_id TEXT NOT NULL,
	source TEXT NOT NULL,
	boundary_id INTEGER,
	input_tokens INTEGER,
	cached_input_tokens INTEGER,
	uncached_input_tokens INTEGER,
	output_tokens INTEGER,
	reasoning_tokens INTEGER,
	total_tokens INTEGER,
	last_input_tokens INTEGER,
	last_cached_input_tokens INTEGER,
	last_output_tokens INTEGER,
	last_reasoning_tokens INTEGER,
	last_total_tokens INTEGER
);
CREATE INDEX IF NOT EXISTS idx_token_snapshots_session_id ON token_snapshots (session_id);
CREATE INDEX IF NOT EXISTS idx_token_snapshots_created_at ON token_snapshots (created_at);

-- compaction_economics preserves the exact pricing revision and rates used
-- for each estimate. Values are integer micro-USD to avoid floating point
-- drift when pricing or the report is recomputed later.
CREATE TABLE IF NOT EXISTS compaction_economics (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at TEXT NOT NULL,
	session_id TEXT NOT NULL,
	compaction_event_id INTEGER,
	model TEXT NOT NULL DEFAULT '',
	pricing_revision TEXT NOT NULL DEFAULT '',
	cached_input_micros_per_million INTEGER,
	uncached_input_micros_per_million INTEGER,
	output_micros_per_million INTEGER,
	reasoning_micros_per_million INTEGER,
	status TEXT NOT NULL,
	compaction_cost_micros INTEGER,
	post_compaction_cost_micros INTEGER,
	baseline_cost_micros INTEGER,
	savings_micros INTEGER,
	note TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_compaction_economics_session_id ON compaction_economics (session_id);
CREATE INDEX IF NOT EXISTS idx_compaction_economics_created_at ON compaction_economics (created_at);
`

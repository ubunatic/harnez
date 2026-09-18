package telemetry

import (
	"fmt"
	"strings"
	"time"
)

// Filter narrows a Query/Aggregate call. Zero-value fields are ignored
// (unfiltered). This is deliberately just what a WHERE-filtered aggregate
// query needs today (issue 120's harnez stats consumes this) — no
// generic query-builder abstraction.
type Filter struct {
	ToolName  string
	AgentID   string
	TicketID  string
	SessionID string
	CallType  string
	Project   string    // filters on project_name (issue 227)
	Since     time.Time // rows with created_at >= Since, if non-zero
	Until     time.Time // rows with created_at < Until, if non-zero

	// Command and FailedOnly apply only to QueryCLIInvocations (issue 326):
	// cli_invocations has a command column where tool_calls has tool_name,
	// and `harnez log --failed` needs a non-string predicate that no
	// tool_calls consumer wants. They live on this shared Filter (rather
	// than in a second filter type) per issue 326's scope note, and are
	// ignored by whereClause below — the tool_calls query path never sees
	// them.
	Command    string
	FailedOnly bool // rows with a non-NULL, non-zero exit_code
}

// whereClause builds a "WHERE ..." SQL fragment (or "" if unfiltered) and
// its bound args, shared by Query and Aggregate.
func (f Filter) whereClause() (string, []any) {
	var clauses []string
	var args []any

	add := func(col, val string) {
		if val != "" {
			clauses = append(clauses, col+" = ?")
			args = append(args, val)
		}
	}
	add("tool_name", f.ToolName)
	add("agent_id", f.AgentID)
	add("ticket_id", f.TicketID)
	add("session_id", f.SessionID)
	add("call_type", f.CallType)
	add("project_name", f.Project)
	if !f.Since.IsZero() {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, f.Since.Format(time.RFC3339Nano))
	}
	if !f.Until.IsZero() {
		clauses = append(clauses, "created_at < ?")
		args = append(args, f.Until.Format(time.RFC3339Nano))
	}

	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

// cliWhereClause is whereClause's cli_invocations counterpart: the same
// "zero-value fields are ignored" contract, restricted to the columns that
// table actually has. It is a separate builder rather than a flag on
// whereClause because the two tables genuinely differ (no tool_name /
// call_type here; no command / exit-code predicate there) — splicing a
// `command = ?` clause into a tool_calls query would be a runtime SQL
// error, so the split is enforced structurally.
func (f Filter) cliWhereClause() (string, []any) {
	var clauses []string
	var args []any

	add := func(col, val string) {
		if val != "" {
			clauses = append(clauses, col+" = ?")
			args = append(args, val)
		}
	}
	add("agent_id", f.AgentID)
	add("ticket_id", f.TicketID)
	add("session_id", f.SessionID)
	add("project_name", f.Project)
	add("command", f.Command)
	if f.FailedOnly {
		clauses = append(clauses, "exit_code IS NOT NULL AND exit_code != 0")
	}
	if !f.Since.IsZero() {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, f.Since.Format(time.RFC3339Nano))
	}
	if !f.Until.IsZero() {
		clauses = append(clauses, "created_at < ?")
		args = append(args, f.Until.Format(time.RFC3339Nano))
	}

	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

// QueryCLIInvocations returns the cli_invocations rows matching f, newest
// first, capped at limit (a limit <= 0 means uncapped — `harnez log --all`).
// All filtering is pushed into SQL: issue 327 explicitly forbids fetching
// an unbounded result set and post-filtering it in Go.
func (d *DB) QueryCLIInvocations(f Filter, limit int) ([]CLIInvocation, error) {
	where, args := f.cliWhereClause()
	query := `
		SELECT id, created_at, session_id, agent_id, command, args, project_name,
		       working_dir, ticket_id, exit_code, duration_ms, harnez_version
		FROM cli_invocations` + where + `
		ORDER BY created_at DESC, id DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	ctx, cancel := defaultContext()
	defer cancel()

	rows, err := d.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query cli invocations: %w", err)
	}
	defer rows.Close()

	var out []CLIInvocation
	for rows.Next() {
		var c CLIInvocation
		var createdAt string
		if err := rows.Scan(
			&c.ID, &createdAt, &c.SessionID, &c.AgentID, &c.Command, &c.Args,
			&c.ProjectName, &c.WorkingDir, &c.TicketID, &c.ExitCode,
			&c.DurationMs, &c.HarnezVersion,
		); err != nil {
			return nil, fmt.Errorf("telemetry: scan cli invocation row: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("telemetry: parse created_at %q: %w", createdAt, err)
		}
		c.CreatedAt = parsed
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("telemetry: query cli invocation rows: %w", err)
	}
	return out, nil
}

// CLIInvocationCounts returns how many cli_invocations rows match f,
// grouped by command — the per-subcommand call counts issue 328 hands to
// internal/sessionstate at the sessionTipHook boundary so that package can
// stop maintaining its own duplicate counter without gaining a DB
// dependency. The total is the sum of the returned map's values.
func (d *DB) CLIInvocationCounts(f Filter) (map[string]int, error) {
	where, args := f.cliWhereClause()
	ctx, cancel := defaultContext()
	defer cancel()

	rows, err := d.sql.QueryContext(ctx, `
		SELECT command, COUNT(*)
		FROM cli_invocations`+where+`
		GROUP BY command`, args...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: cli invocation counts: %w", err)
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var command string
		var count int
		if err := rows.Scan(&command, &count); err != nil {
			return nil, fmt.Errorf("telemetry: scan cli invocation count: %w", err)
		}
		out[command] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("telemetry: cli invocation count rows: %w", err)
	}
	return out, nil
}

// Query returns the tool_calls rows matching f, newest first.
func (d *DB) Query(f Filter) ([]ToolCall, error) {
	where, args := f.whereClause()
	ctx, cancel := defaultContext()
	defer cancel()

	rows, err := d.sql.QueryContext(ctx, `
		SELECT id, created_at, session_id, ticket_id, project_name, working_dir,
		       agent_id, tool_name, call_type, score, note, exit_code,
		       duration_ms, raw_bytes, distilled_bytes, output_bytes, actual_tokens,
		       potential_savings_tokens, potential_savings_bytes
		FROM tool_calls`+where+`
		ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: query: %w", err)
	}
	defer rows.Close()

	var out []ToolCall
	for rows.Next() {
		var tc ToolCall
		var createdAt string
		if err := rows.Scan(
			&tc.ID, &createdAt, &tc.SessionID, &tc.TicketID, &tc.ProjectName,
			&tc.WorkingDir, &tc.AgentID, &tc.ToolName, &tc.CallType, &tc.Score,
			&tc.Note, &tc.ExitCode, &tc.DurationMs, &tc.RawBytes, &tc.DistilledBytes,
			&tc.OutputBytes, &tc.ActualTokens, &tc.PotentialSavingsTokens,
			&tc.PotentialSavingsBytes,
		); err != nil {
			return nil, fmt.Errorf("telemetry: scan row: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("telemetry: parse created_at %q: %w", createdAt, err)
		}
		tc.CreatedAt = parsed
		out = append(out, tc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("telemetry: query rows: %w", err)
	}
	return out, nil
}

// Stats is an aggregate summary over a Filter's matching rows — the shape
// issue 120's `harnez stats` needs: how many calls, how well they scored,
// how often they failed, and how many bytes distillation saved.
type Stats struct {
	Count          int64
	AvgScore       float64 // 0 if no scored rows
	ScoredCount    int64   // rows with a non-NULL score, denominator for AvgScore
	FailureCount   int64   // rows with exit_code != 0
	ExitCodedCount int64   // rows with a non-NULL exit_code, denominator for failure rate
	TotalRawBytes  int64
	TotalDistilled int64
	AvgDurationMs  float64
}

// GroupStats is one group's row in a grouped aggregate (AggregateByTool /
// AggregateByAgent) — call frequency, average score, and failure rate for
// one tool_name or agent_id value, the shape issue 120's `harnez stats`
// needs. It deliberately does not embed/reuse Stats: FailureCount here is
// issue 120's own definition (exit_code != 0 OR score <= 2), broader than
// Stats.FailureCount's exit_code-only definition used by the pre-existing
// ungrouped Aggregate, so the two are kept as distinct types rather than
// risking the same field name silently meaning two different things.
type GroupStats struct {
	Key                         string  `json:"key"`
	Count                       int64   `json:"count"`
	AvgScore                    float64 `json:"avg_score"`
	ScoredCount                 int64   `json:"scored_count"`
	// FailureCount counts rows with exit_code != 0 OR score <= 2, excluding
	// rows with call_type == ExpectedFailureCallType (issue 226): a shell
	// command an agent ran expecting it to fail is real telemetry (its true
	// exit_code is still stored) but isn't "failed agent/tool behavior" for
	// this quality signal's purposes.
	FailureCount                int64   `json:"failure_count"`
	TotalRawBytes               int64   `json:"total_raw_bytes"`
	TotalDistilled              int64   `json:"total_distilled"`
	AvgDurationMs               float64 `json:"avg_duration_ms"`
	AvgActualTokens             float64 `json:"avg_actual_tokens"`
	TotalPotentialSavingsTokens int64   `json:"potential_savings_tokens"`
	TotalPotentialSavingsBytes  int64   `json:"potential_savings_bytes"`
}

// aggregateGroupedBy summarizes tool_calls rows matching f, one row per
// distinct value of the given column, ordered by call count descending
// (busiest group first) then by key for determinism. column is always an
// internal literal (not caller input) — see AggregateByTool/AggregateByAgent,
// the only callers — so it's safe to splice directly into the query.
func (d *DB) aggregateGroupedBy(column string, f Filter) ([]GroupStats, error) {
	where, args := f.whereClause()
	ctx, cancel := defaultContext()
	defer cancel()

	// The FailureCount CASE excludes ExpectedFailureCallType rows (issue
	// 226): an intentionally-expected shell failure still has its true
	// exit_code stored, but must not count toward this quality signal — see
	// GroupStats.FailureCount's doc comment. args needs the bound param
	// spliced in before the shared where args, matching the CASE's position
	// in the SQL text.
	queryArgs := append([]any{ExpectedFailureCallType}, args...)
	rows, err := d.sql.QueryContext(ctx, `
		SELECT
			`+column+`,
			COUNT(*),
			AVG(score),
			COUNT(score),
			COUNT(CASE WHEN call_type != ?
			           AND ((exit_code IS NOT NULL AND exit_code != 0)
			                OR (score IS NOT NULL AND score <= 2)) THEN 1 END),
			COALESCE(SUM(raw_bytes), 0),
			COALESCE(SUM(distilled_bytes), 0),
			AVG(duration_ms),
			AVG(actual_tokens),
			COALESCE(SUM(potential_savings_tokens), 0),
			COALESCE(SUM(potential_savings_bytes), 0)
		FROM tool_calls`+where+`
		GROUP BY `+column+`
		ORDER BY COUNT(*) DESC, `+column+` ASC`, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: aggregate grouped by %s: %w", column, err)
	}
	defer rows.Close()

	var out []GroupStats
	for rows.Next() {
		var g GroupStats
		var avgScore, avgDuration, avgActualTokens *float64
		if err := rows.Scan(
			&g.Key, &g.Count, &avgScore, &g.ScoredCount, &g.FailureCount,
			&g.TotalRawBytes, &g.TotalDistilled, &avgDuration, &avgActualTokens,
			&g.TotalPotentialSavingsTokens, &g.TotalPotentialSavingsBytes,
		); err != nil {
			return nil, fmt.Errorf("telemetry: scan grouped row: %w", err)
		}
		if avgScore != nil {
			g.AvgScore = *avgScore
		}
		if avgDuration != nil {
			g.AvgDurationMs = *avgDuration
		}
		if avgActualTokens != nil {
			g.AvgActualTokens = *avgActualTokens
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("telemetry: aggregate grouped rows: %w", err)
	}
	return out, nil
}

// AggregateByTool summarizes tool_calls rows matching f, one GroupStats
// row per distinct tool_name — the per-tool breakdown issue 120's
// `harnez stats` needs (call frequency, average score, failure rate,
// byte savings, all scoped to one tool).
func (d *DB) AggregateByTool(f Filter) ([]GroupStats, error) {
	return d.aggregateGroupedBy("tool_name", f)
}

// AggregateByAgent summarizes tool_calls rows matching f, one GroupStats
// row per distinct agent_id — the per-agent breakdown issue 120's
// `harnez stats` needs.
func (d *DB) AggregateByAgent(f Filter) ([]GroupStats, error) {
	return d.aggregateGroupedBy("agent_id", f)
}

// AggregateByProject summarizes tool_calls rows matching f, one GroupStats
// row per distinct project_name — the per-project breakdown issue 227's
// `harnez stats` needs (call frequency, average score, and failure rate
// broken down per project, for users working across multiple checkouts).
// Grouped on project_name rather than working_dir per issue 227's Notes:
// project_name is the more stable identity across relocations of a
// checkout.
func (d *DB) AggregateByProject(f Filter) ([]GroupStats, error) {
	return d.aggregateGroupedBy("project_name", f)
}

// Aggregate summarizes the tool_calls rows matching f.
func (d *DB) Aggregate(f Filter) (Stats, error) {
	where, args := f.whereClause()
	ctx, cancel := defaultContext()
	defer cancel()

	var s Stats
	var avgScore, avgDuration *float64
	row := d.sql.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			AVG(score),
			COUNT(score),
			COUNT(CASE WHEN exit_code IS NOT NULL AND exit_code != 0 THEN 1 END),
			COUNT(exit_code),
			COALESCE(SUM(raw_bytes), 0),
			COALESCE(SUM(distilled_bytes), 0),
			AVG(duration_ms)
		FROM tool_calls`+where, args...)
	if err := row.Scan(
		&s.Count, &avgScore, &s.ScoredCount, &s.FailureCount, &s.ExitCodedCount,
		&s.TotalRawBytes, &s.TotalDistilled, &avgDuration,
	); err != nil {
		return Stats{}, fmt.Errorf("telemetry: aggregate: %w", err)
	}
	if avgScore != nil {
		s.AvgScore = *avgScore
	}
	if avgDuration != nil {
		s.AvgDurationMs = *avgDuration
	}
	return s, nil
}

// DistillationSavings summarizes distillation byte savings over tool_calls
// rows matching f, restricted to rows where distilled_bytes IS NOT NULL
// (rows distillation never ran on don't belong in the ratio — see issue
// 120's Scope). RawBytes/DistilledBytes are the raw sums (over that same
// restricted row set, so they're directly comparable); Ratio is
// 1 - (DistilledBytes / RawBytes), or 0 when Count is 0 or RawBytes is 0
// (nothing to divide by — the caller, cmd/harnez/stats.go, is expected to
// check Count before treating Ratio as meaningful).
type DistillationSavings struct {
	Count          int64   `json:"count"`
	RawBytes       int64   `json:"raw_bytes"`
	DistilledBytes int64   `json:"distilled_bytes"`
	Ratio          float64 `json:"ratio"`
}

// rateCallType is the call_type value a failure/unexpected-outcome
// `harnez rate` call writes (see cmd/harnez/rate.go) — it uniquely
// identifies per-tool ratings in the tool_calls table without needing a
// separate flag column.
const rateCallType = "internal"

// HeartbeatCallType is the call_type value `harnez rate --ok` writes
// (issue 179) — a lean "N calls since the last check were fine"
// confirmation. Deliberately distinct from rateCallType so a heartbeat's
// NULL score/exit_code (see cmd/harnez/rate.go's runRateOk) never mixes
// into failure-rating aggregates under an ambiguous shared call_type, and
// so RateCallOverhead/HeartbeatStats can each select their own rows
// cleanly.
const HeartbeatCallType = "heartbeat"

// ExpectedFailureCallType is the call_type value `harnez exec` writes for a
// shell command an agent ran expecting it to fail (issue 226's direction
// 2 — e.g. probing whether a server is down, deliberately reproducing a
// bug) instead of the normal "shell" — see cmd/harnez/exec.go's
// HARNEZ_EXPECT_FAILURE convention. The row still carries the command's
// real exit_code (never faked) so ad-hoc queries/exports keep seeing the
// truth; only the aggregate signals below (GroupStats.FailureCount,
// UnratedFailureCount) treat it as excluded from "genuinely failed,"
// mirroring rateCallType/HeartbeatCallType's own exclusion.
//
// Reusing call_type for this — rather than a new boolean column — was a
// deliberate issue 226 design call: this schema has no migration
// framework (schema.go's schemaVersion doc comment: a version bump forces
// a "delete the file" reset, no in-place ALTER TABLE), so a new column
// would force every existing user's local telemetry DB to be discarded on
// upgrade. call_type is already the established "how was this row meant
// to be read" discriminator (internal/heartbeat/shell), so a fourth value
// composes with the existing exclusion pattern for free.
const ExpectedFailureCallType = "shell-expected"

// RateCallOverhead summarizes the measured per-call cost attributable to
// `harnez rate` calls matching f — both failure ratings (call_type
// "internal") and --ok heartbeats (call_type "heartbeat") count toward
// this, since both are the same CLI command's overhead (issue 142); use
// HeartbeatStats for heartbeat-specific reporting (issue 179). It is real
// measured data (RawBytes, populated by cmd/harnez/rate.go's
// rateCallPayloadBytes/heartbeatCallPayloadBytes), not a token estimate;
// EstimateTokens converts it to a labeled estimate for reporting.
type RateCallOverhead struct {
	Count          int64
	TotalCallBytes int64
	AvgCallBytes   float64
}

// RateCallOverhead computes the rate-call overhead aggregate matching f,
// summed across both rateCallType and HeartbeatCallType rows. Any CallType
// already set on f is ignored — this report is specifically about
// `harnez rate` calls (in either mode), not a general filter escape hatch.
func (d *DB) RateCallOverhead(f Filter) (RateCallOverhead, error) {
	var o RateCallOverhead
	for _, ct := range []string{rateCallType, HeartbeatCallType} {
		cf := f
		cf.CallType = ct
		s, err := d.Aggregate(cf)
		if err != nil {
			return RateCallOverhead{}, fmt.Errorf("rate call overhead: %w", err)
		}
		o.Count += s.Count
		o.TotalCallBytes += s.TotalRawBytes
	}
	if o.Count > 0 {
		o.AvgCallBytes = float64(o.TotalCallBytes) / float64(o.Count)
	}
	return o, nil
}

// EstimateTokens converts a byte count to an ESTIMATED token count using a
// ~4-bytes-per-token heuristic (a commonly cited rough average for English
// text under BPE-style tokenizers). This is explicitly NOT a
// provider-reported token count — harnez has no access to the calling
// model's real tokenizer or usage numbers — callers must label any output
// derived from this as an estimate (see issue 142).
func EstimateTokens(bytes int64) int64 {
	return bytes / 4
}

// DistillationSavings computes the byte-savings aggregate matching f.
func (d *DB) DistillationSavings(f Filter) (DistillationSavings, error) {
	where, args := f.whereClause()
	restrict := " WHERE distilled_bytes IS NOT NULL"
	if where != "" {
		restrict = where + " AND distilled_bytes IS NOT NULL"
	}
	ctx, cancel := defaultContext()
	defer cancel()

	var ds DistillationSavings
	row := d.sql.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(raw_bytes), 0),
			COALESCE(SUM(distilled_bytes), 0)
		FROM tool_calls`+restrict, args...)
	if err := row.Scan(&ds.Count, &ds.RawBytes, &ds.DistilledBytes); err != nil {
		return DistillationSavings{}, fmt.Errorf("telemetry: distillation savings: %w", err)
	}
	if ds.Count > 0 && ds.RawBytes > 0 {
		ds.Ratio = 1 - (float64(ds.DistilledBytes) / float64(ds.RawBytes))
	}
	return ds, nil
}

// HeartbeatInfo summarizes a session's `harnez rate --ok` heartbeat
// history matching f (issue 179): how many heartbeats were recorded, when
// the most recent one landed, and how many tool_calls rows of any
// call_type have landed since — a rough "confirmed-clean streak" length
// for `harnez stats` to surface, since silence alone can't distinguish
// "everything's been fine" from "the agent forgot the protocol."
type HeartbeatInfo struct {
	Count      int64     `json:"count"`
	LastAt     time.Time `json:"last_at,omitempty"` // zero if Count == 0
	CallsSince int64     `json:"calls_since"`       // tool_calls rows (any call_type) matching f with created_at > LastAt; 0 if Count == 0
}

// UnratedFailureCount computes issue 188's unrated-failure signal for f
// (normally scoped to SessionID): how many genuinely-failed tool calls
// (exit_code != 0 OR score <= 2 — GroupStats.FailureCount's exact
// definition, reused rather than reinvented) have gone unrated since the
// last `harnez rate` failure-rating call (call_type "internal") matching f.
//
// Linkage heuristic: the schema has no column linking a rating row back to
// the specific tool call(s) it covers, and adding one (plus updating every
// rate-call site to populate it) is real surface area for a v1. Instead
// this uses a session-window proxy explicitly sanctioned by issue 188: the
// most recent "internal" rate call marks the point up to which the agent
// is presumed to have addressed prior failures, so only failures *after*
// that point (or, if no rate call has ever fired, all of them) count as
// unrated. This is not perfect per-call linkage — an agent could rate one
// failure while leaving an earlier concurrent one unaddressed — but it is
// far simpler and correct in the common case (fix-or-rate, then move on)
// that issue 188 targets. call_type "internal"/"heartbeat" rows themselves
// are excluded from the failure count: a rate call's own row is never the
// failure being reported on. call_type "shell-expected"
// (ExpectedFailureCallType) rows are excluded too (issue 226): a shell
// command an agent ran expecting it to fail shouldn't trip this nag just
// because it composes fine with GroupStats.FailureCount's plain mechanical
// definition.
func (d *DB) UnratedFailureCount(f Filter) (int64, error) {
	rateFilter := f
	rateFilter.CallType = rateCallType
	where, args := rateFilter.whereClause()
	ctx, cancel := defaultContext()
	defer cancel()

	var lastRateAt *string
	row := d.sql.QueryRowContext(ctx, `SELECT MAX(created_at) FROM tool_calls`+where, args...)
	if err := row.Scan(&lastRateAt); err != nil {
		return 0, fmt.Errorf("telemetry: unrated failure count: last rate call: %w", err)
	}

	failureFilter := f
	failureFilter.CallType = ""
	if lastRateAt != nil {
		t, err := time.Parse(time.RFC3339Nano, *lastRateAt)
		if err != nil {
			return 0, fmt.Errorf("telemetry: unrated failure count: parse last rate created_at %q: %w", *lastRateAt, err)
		}
		failureFilter.Since = t.Add(time.Nanosecond) // strictly after the rate call itself
	}
	fWhere, fArgs := failureFilter.whereClause()
	extra := "call_type NOT IN (?, ?, ?) AND ((exit_code IS NOT NULL AND exit_code != 0) OR (score IS NOT NULL AND score <= 2))"
	if fWhere == "" {
		fWhere = " WHERE " + extra
	} else {
		fWhere += " AND " + extra
	}
	fArgs = append(fArgs, rateCallType, HeartbeatCallType, ExpectedFailureCallType)

	ctx2, cancel2 := defaultContext()
	defer cancel2()

	var count int64
	err := d.sql.QueryRowContext(ctx2, `SELECT COUNT(*) FROM tool_calls`+fWhere, fArgs...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("telemetry: unrated failure count: %w", err)
	}
	return count, nil
}

// HeartbeatStats computes HeartbeatInfo for the rows matching f. Any
// CallType already set on f is ignored for the heartbeat lookup itself
// (overridden to HeartbeatCallType) but still applies to the CallsSince
// follow-up count, which intentionally spans every call_type to reflect
// real activity since the last confirmed-clean checkpoint.
func (d *DB) HeartbeatStats(f Filter) (HeartbeatInfo, error) {
	hbFilter := f
	hbFilter.CallType = HeartbeatCallType
	where, args := hbFilter.whereClause()
	ctx, cancel := defaultContext()
	defer cancel()

	var info HeartbeatInfo
	var lastAt *string
	row := d.sql.QueryRowContext(ctx, `
		SELECT COUNT(*), MAX(created_at)
		FROM tool_calls`+where, args...)
	if err := row.Scan(&info.Count, &lastAt); err != nil {
		return HeartbeatInfo{}, fmt.Errorf("telemetry: heartbeat stats: %w", err)
	}
	if info.Count == 0 || lastAt == nil {
		return info, nil
	}
	t, err := time.Parse(time.RFC3339Nano, *lastAt)
	if err != nil {
		return HeartbeatInfo{}, fmt.Errorf("telemetry: parse heartbeat created_at %q: %w", *lastAt, err)
	}
	info.LastAt = t

	sinceFilter := f
	sinceFilter.CallType = "" // count all call types since the last heartbeat
	sinceFilter.Since = t
	sinceWhere, sinceArgs := sinceFilter.whereClause()
	ctx2, cancel2 := defaultContext()
	defer cancel2()
	var callsSince int64
	if err := d.sql.QueryRowContext(ctx2, `SELECT COUNT(*) FROM tool_calls`+sinceWhere, sinceArgs...).Scan(&callsSince); err != nil {
		return HeartbeatInfo{}, fmt.Errorf("telemetry: heartbeat calls-since: %w", err)
	}
	// Since.whereClause() uses ">=", so the heartbeat row itself (created_at
	// == t) is included once; subtract it back out.
	if callsSince > 0 {
		callsSince--
	}
	info.CallsSince = callsSince
	return info, nil
}

package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// Claude compaction rows (issue 1303 M1). Fixtures use the transcript shape
// verified on the host: a system compact_boundary line with compactMetadata.
const claudeCompactSummarySentinel = "SENTINEL-SUMMARY-1303"
const claudeCompactInstructionsSentinel = "SENTINEL-INSTR-1303"

func claudeAssistantLine(ts time.Time, model string) string {
	return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"message":{"model":%q,"usage":{"input_tokens":10,"cache_read_input_tokens":900,"cache_creation_input_tokens":50,"output_tokens":20}}}`, ts.UTC().Format(time.RFC3339Nano), model)
}

func claudeBoundaryLine(ts time.Time, pre, post int64) string {
	return fmt.Sprintf(`{"type":"system","subtype":"compact_boundary","timestamp":%q,"compactMetadata":{"trigger":"manual","preTokens":%d,"postTokens":%d,"durationMs":1000}}`, ts.UTC().Format(time.RFC3339Nano), pre, post)
}

func claudeHookLine(event, transcript string, withSecrets bool) string {
	extra := ""
	if withSecrets {
		extra = fmt.Sprintf(`,"compact_summary":%q,"custom_instructions":%q`, claudeCompactSummarySentinel, claudeCompactInstructionsSentinel)
	}
	return fmt.Sprintf(`{"session_id":"claude-compact","transcript_path":%q,"cwd":"/tmp","hook_event_name":%q,"trigger":"manual"%s}`, transcript, event, extra)
}

func appendLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, line := range lines {
		if _, err := f.WriteString(line + "\n"); err != nil {
			t.Fatal(err)
		}
	}
}

func openRawCompactDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { rawDB.Close() })
	return rawDB
}

func TestClaudeCompactFillsRowsModelAndEconomics(t *testing.T) {
	// Issue 1303 M3: a PreCompact with no boundary arms a fill, so the state dir and
	// the runtime socket must be the scratch ones.
	isolateHookEnv(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "telemetry.sqlite")
	transcript := filepath.Join(dir, "session.jsonl")
	appendLines(t, transcript, claudeAssistantLine(time.Now(), "claude-opus-5-5"))

	if err := runCodexTelemetryAt(strings.NewReader(claudeHookLine("PreCompact", transcript, false)), dbPath); err != nil {
		t.Fatalf("PreCompact: %v", err)
	}
	// The boundary is written after PreCompact, so its timestamp is not before the PreCompact row.
	appendLines(t, transcript, claudeBoundaryLine(time.Now(), 166948, 27143))
	if err := runCodexTelemetryAt(strings.NewReader(claudeHookLine("PostCompact", transcript, false)), dbPath); err != nil {
		t.Fatalf("PostCompact: %v", err)
	}

	rawDB := openRawCompactDB(t, dbPath)
	var input, total sql.NullInt64
	var model string
	if err := rawDB.QueryRow(`SELECT input_tokens, total_tokens, model FROM compaction_events WHERE session_id = ? AND event_type = 'postcompact'`, "claude-compact").Scan(&input, &total, &model); err != nil {
		t.Fatalf("postcompact row: %v", err)
	}
	if !total.Valid || total.Int64 != 27143 || !input.Valid || input.Int64 != 27143 {
		t.Errorf("postcompact tokens = %v/%v, want 27143", input, total)
	}
	if model != "claude-opus-5-5" {
		t.Errorf("postcompact model = %q, want claude-opus-5-5", model)
	}

	var preTotal int64
	if err := rawDB.QueryRow(`SELECT total_tokens FROM token_snapshots WHERE session_id = ? AND source = 'precompact' ORDER BY id DESC LIMIT 1`, "claude-compact").Scan(&preTotal); err != nil {
		t.Fatalf("precompact snapshot: %v", err)
	}
	if preTotal != 166948 {
		t.Errorf("precompact snapshot total = %d, want 166948", preTotal)
	}

	var status, note string
	var compactCost, postCost sql.NullInt64
	if err := rawDB.QueryRow(`SELECT status, note, compaction_cost_micros, post_compaction_cost_micros FROM compaction_economics WHERE session_id = ?`, "claude-compact").Scan(&status, &note, &compactCost, &postCost); err != nil {
		t.Fatalf("economics row: %v", err)
	}
	if !compactCost.Valid || !postCost.Valid {
		t.Errorf("economics costs = %v/%v, want both priced (status %q)", compactCost, postCost, status)
	}
	// Claude has no SessionEnd snapshot, so no no-compaction baseline: savings stay withheld.
	if status != "insufficient_data" || !strings.Contains(note, "baseline") {
		t.Errorf("economics status = %q note = %q, want insufficient_data for the missing baseline", status, note)
	}
}

func TestClaudeCompactNegativeCasesWriteEmptyRows(t *testing.T) {
	// Issue 1303 M3: the no-boundary cases arm fills; keep them off the real state dir.
	isolateHookEnv(t)
	cases := []struct {
		name       string
		setup      func(t *testing.T, transcript string)
		transcript func(t *testing.T, dir string) string
	}{
		{
			name: "no boundary yet",
			setup: func(t *testing.T, transcript string) {
				appendLines(t, transcript, claudeAssistantLine(time.Now(), "claude-opus-5-5"))
			},
		},
		{
			name: "only an older boundary",
			setup: func(t *testing.T, transcript string) {
				appendLines(t, transcript, claudeBoundaryLine(time.Now().Add(-time.Hour), 900, 90))
			},
		},
		{
			name:       "missing transcript_path",
			setup:      func(t *testing.T, transcript string) {},
			transcript: func(t *testing.T, dir string) string { return "" },
		},
		{
			name:       "unreadable transcript",
			setup:      func(t *testing.T, transcript string) {},
			transcript: func(t *testing.T, dir string) string { return dir },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			dbPath := filepath.Join(dir, "telemetry.sqlite")
			transcript := filepath.Join(dir, "session.jsonl")
			tc.setup(t, transcript)
			if tc.transcript != nil {
				transcript = tc.transcript(t, dir)
			}
			if err := runCodexTelemetryAt(strings.NewReader(claudeHookLine("PreCompact", transcript, false)), dbPath); err != nil {
				t.Fatalf("PreCompact: %v", err)
			}
			if err := runCodexTelemetryAt(strings.NewReader(claudeHookLine("PostCompact", transcript, false)), dbPath); err != nil {
				t.Fatalf("PostCompact must exit 0: %v", err)
			}
			rawDB := openRawCompactDB(t, dbPath)
			var input, total sql.NullInt64
			var model string
			if err := rawDB.QueryRow(`SELECT input_tokens, total_tokens, model FROM compaction_events WHERE session_id = ? AND event_type = 'postcompact'`, "claude-compact").Scan(&input, &total, &model); err != nil {
				t.Fatalf("postcompact row: %v", err)
			}
			if input.Valid || total.Valid || model != "" {
				t.Errorf("postcompact row filled (%v, %v, %q), want empty", input, total, model)
			}
		})
	}
}

func TestClaudeCompactPayloadSecretsNeverPersist(t *testing.T) {
	isolateHookEnv(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "telemetry.sqlite")
	transcript := filepath.Join(dir, "session.jsonl")
	appendLines(t, transcript, claudeAssistantLine(time.Now(), "claude-opus-5-5"))
	if err := runCodexTelemetryAt(strings.NewReader(claudeHookLine("PreCompact", transcript, true)), dbPath); err != nil {
		t.Fatalf("PreCompact: %v", err)
	}
	appendLines(t, transcript, claudeBoundaryLine(time.Now(), 500, 50))
	if err := runCodexTelemetryAt(strings.NewReader(claudeHookLine("PostCompact", transcript, true)), dbPath); err != nil {
		t.Fatalf("PostCompact: %v", err)
	}
	// Close the handle so the WAL is checkpointed before the file scan.
	rawDB := openRawCompactDB(t, dbPath)
	if err := rawDB.Close(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dbPath, dbPath + "-wal"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, sentinel := range []string{claudeCompactSummarySentinel, claudeCompactInstructionsSentinel} {
			if bytes.Contains(data, []byte(sentinel)) {
				t.Errorf("%s holds %s", filepath.Base(path), sentinel)
			}
		}
	}
}

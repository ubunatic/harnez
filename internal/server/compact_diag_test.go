package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
)

// Issue 1303 M1c: each Claude PostCompact writes one diagnostic note beside the
// telemetry DB. The note records the outcome reason and the payload key names
// only. These tests use scratch dirs and a fake fill timer, so they never sleep
// and never reach the live collector.

const (
	diagSummarySentinel      = "DIAG-SUMMARY-SENTINEL-7f3a"
	diagInstructionsSentinel = "DIAG-INSTRUCTIONS-SENTINEL-9c1d"
	diagSessionID            = "claude-wait"
)

// isolateCompactEnv points HOME and every XDG base at a temp dir, clears the
// inherited broker FD, asserts that the telemetry socket stays inside that dir,
// and replaces the fill timer with a fake (issue 1303 M1b, M1e). A scratch HOME
// alone does not isolate hook telemetry (issue 1303 M1b).
func isolateCompactEnv(t *testing.T) *fakeFills {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "run"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	t.Setenv(cfg.RunnerChannel.FDEnv, "")
	if RunnerChannelStatus(cfg) == nil {
		t.Fatalf("broker FD still valid after clearing %s", cfg.RunnerChannel.FDEnv)
	}
	sock, err := cfg.Socket()
	if err != nil {
		t.Fatalf("Socket: %v", err)
	}
	if !strings.HasPrefix(sock, root) {
		t.Fatalf("telemetry socket %q is outside the scratch root %q", sock, root)
	}
	return installFakeFills(t)
}

func claudePreCompactRaw(path string) []byte {
	return []byte(fmt.Sprintf(`{"session_id":%q,"transcript_path":%q,"hook_event_name":"PreCompact","trigger":"manual"}`, diagSessionID, path))
}

// compactDiagFromEvidence decodes the hook-side diagnostic from a prepared payload.
func compactDiagFromEvidence(t *testing.T, prepared []byte) *compactDiag {
	t.Helper()
	var hook struct {
		Context codexEvidence `json:"harnez_context"`
	}
	if err := json.Unmarshal(prepared, &hook); err != nil {
		t.Fatal(err)
	}
	return hook.Context.Diag
}

// compactDiagNotes reads every note from the file beside the telemetry DB.
func compactDiagNotes(t *testing.T, dbPath string) []compactDiagNote {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(filepath.Dir(dbPath), compactDiagFile))
	if err != nil {
		t.Fatalf("read diagnostic notes: %v", err)
	}
	var notes []compactDiagNote
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var note compactDiagNote
		if err := json.Unmarshal([]byte(line), &note); err != nil {
			t.Fatalf("decode note %q: %v", line, err)
		}
		notes = append(notes, note)
	}
	return notes
}

// processPrepared runs one prepared-or-raw hook payload through the collector writer.
func processPrepared(t *testing.T, db *telemetry.DB, raw []byte) {
	t.Helper()
	prepared, err := PrepareCodexTelemetry(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := ProcessCodexTelemetryTx(db, prepared); err != nil {
		t.Fatal(err)
	}
}

// The hook side reports why its one transcript read did or did not produce a
// boundary. It never judges whether the boundary is this compaction's: the
// collector does that against the PreCompact row (issue 1303 M1e).
func TestCompactDiagClientReasons(t *testing.T) {
	isolateCompactEnv(t)
	start := time.Now().UTC()
	cases := []struct {
		name   string
		setup  func(t *testing.T) string
		reason string
	}{
		{"no_transcript", func(t *testing.T) string { return "" }, reasonNoTranscript},
		{"read_error", func(t *testing.T) string { return t.TempDir() }, reasonReadError},
		{"no_boundary", func(t *testing.T) string {
			return writeTranscript(t, claudeAssistantLineFixture)
		}, reasonNoBoundary},
		{"old_boundary_left_to_collector", func(t *testing.T) string {
			old := start.Add(-2 * time.Hour).Format(time.RFC3339Nano)
			return writeTranscript(t, claudeAssistantLineFixture, fmt.Sprintf(claudeBoundaryLineFmt, old, 900, 90))
		}, ""},
		{"boundary_found", func(t *testing.T) string {
			now := start.Format(time.RFC3339Nano)
			return writeTranscript(t, claudeAssistantLineFixture, fmt.Sprintf(claudeBoundaryLineFmt, now, 500, 50))
		}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.setup(t)
			prepared, err := PrepareCodexTelemetry(claudePostCompactRaw(path, true))
			if err != nil {
				t.Fatal(err)
			}
			diag := compactDiagFromEvidence(t, prepared)
			if diag == nil {
				t.Fatal("PostCompact carried no diagnostic")
			}
			if diag.Reason != tc.reason {
				t.Errorf("reason = %q, want %q", diag.Reason, tc.reason)
			}
			if diag.Side != "hook" || diag.PID != os.Getpid() {
				t.Errorf("side %q pid %d, want hook pid %d", diag.Side, diag.PID, os.Getpid())
			}
			if diag.TranscriptSet != (path != "") {
				t.Errorf("transcript_set = %v, want %v", diag.TranscriptSet, path != "")
			}
			if !slices.Contains(diag.Keys, "compact_summary") {
				t.Errorf("keys %v lack compact_summary", diag.Keys)
			}
		})
	}
}

// The collector writes one note per PostCompact. Its reason is the hook's reason
// when the hook found none, and otherwise the DB-side outcome. A stale or missing
// boundary arms a fill; an applied or unmatched one does not.
func TestCompactDiagCollectorReasons(t *testing.T) {
	cases := []struct {
		name       string
		precompact bool
		boundaryAt func(preAt time.Time) time.Time
		reason     string
		armed      bool
	}{
		{"no_precompact", false, func(time.Time) time.Time { return time.Now().UTC() }, reasonNoPrecompact, false},
		{"stale_boundary", true, func(pre time.Time) time.Time { return pre.Add(-time.Second) }, reasonStaleBoundary, true},
		{"applied", true, func(pre time.Time) time.Time { return pre.Add(time.Second) }, reasonApplied, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fills := isolateCompactEnv(t)
			dbPath, db := openCompactDB(t)
			transcript := writeTranscript(t, claudeAssistantLineFixture)
			preAt := time.Now().UTC()
			if tc.precompact {
				processPrepared(t, db, claudePreCompactRaw(transcript))
				var ok bool
				var err error
				preAt, ok, err = db.LatestCompactionEventTime(diagSessionID, "precompact")
				if err != nil || !ok {
					t.Fatalf("precompact row: ok=%v err=%v", ok, err)
				}
			}
			boundaryAt := tc.boundaryAt(preAt)
			appendTranscriptLine(t, transcript, fmt.Sprintf(claudeBoundaryLineFmt, boundaryAt.Format(time.RFC3339Nano), 500, 50))
			processPrepared(t, db, claudePostCompactRaw(transcript, true))

			last := lastNote(t, dbPath)
			if last.Kind != "postcompact" || last.Reason != tc.reason {
				t.Errorf("note kind %q reason %q, want postcompact %q", last.Kind, last.Reason, tc.reason)
			}
			if last.Hook == nil || last.Hook.Reason != "" {
				t.Errorf("hook reason = %+v, want none: the hook found a boundary", last.Hook)
			}
			if last.CollectorPID != os.Getpid() {
				t.Errorf("collector_pid = %d, want %d", last.CollectorPID, os.Getpid())
			}
			if (last.PreCompactAt != nil) != tc.precompact {
				t.Errorf("precompact_at = %v, want set=%v", last.PreCompactAt, tc.precompact)
			}
			if last.BoundaryAt == nil {
				t.Errorf("boundary_at is empty; the boundary was read")
			}
			if armed := len(fills.armed) > 0; armed != tc.armed {
				t.Errorf("fill armed = %v, want %v (armed %v)", armed, tc.armed, fills.armed)
			}
		})
	}
}

// When the hook found no boundary, its reason is the note's reason, and the
// collector does not overwrite it. The fill is still armed, because Claude writes
// the boundary after PostCompact returns (issue 1303 M1e).
func TestCompactDiagHookReasonWinsOverCollector(t *testing.T) {
	fills := isolateCompactEnv(t)
	dbPath, db := openCompactDB(t)
	processPrepared(t, db, claudePreCompactRaw(writeTranscript(t, claudeAssistantLineFixture)))
	processPrepared(t, db, claudePostCompactRaw(writeTranscript(t, claudeAssistantLineFixture), true))

	last := lastNote(t, dbPath)
	if last.Reason != reasonNoBoundary {
		t.Errorf("note reason = %q, want %q", last.Reason, reasonNoBoundary)
	}
	if last.Hook == nil || last.Hook.Reason != reasonNoBoundary {
		t.Errorf("hook reason = %+v, want %q", last.Hook, reasonNoBoundary)
	}
	if len(fills.armed) != 1 {
		t.Errorf("armed fills %v, want one retry", fills.armed)
	}
}

// The note names payload keys only. The sentinel values in compact_summary and
// custom_instructions never reach the file.
func TestCompactDiagNoteNeverHoldsPayloadValues(t *testing.T) {
	isolateCompactEnv(t)
	start := time.Now().UTC()
	dbPath, db := openCompactDB(t)
	transcript := writeTranscript(t, claudeAssistantLineFixture, fmt.Sprintf(claudeBoundaryLineFmt, start.Format(time.RFC3339Nano), 500, 50))
	raw := []byte(fmt.Sprintf(`{"session_id":%q,"transcript_path":%q,"hook_event_name":"PostCompact","trigger":"manual","compact_summary":%q,"custom_instructions":%q}`,
		diagSessionID, transcript, diagSummarySentinel, diagInstructionsSentinel))
	processPrepared(t, db, raw)

	data, err := os.ReadFile(filepath.Join(filepath.Dir(dbPath), compactDiagFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, sentinel := range []string{diagSummarySentinel, diagInstructionsSentinel} {
		if strings.Contains(string(data), sentinel) {
			t.Errorf("diagnostic note holds payload value %s", sentinel)
		}
	}
	if last := lastNote(t, dbPath); last.Hook == nil || !slices.Contains(last.Hook.Keys, "custom_instructions") {
		t.Errorf("hook keys %+v, want custom_instructions recorded as a name", last.Hook)
	}
}

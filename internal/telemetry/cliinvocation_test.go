package telemetry

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

// TestInsertCLIInvocation_RoundTrip covers issue 326's basic write/read
// contract: a row goes in with every column populated and comes back out of
// QueryCLIInvocations intact.
func TestInsertCLIInvocation_RoundTrip(t *testing.T) {
	db := openTestDB(t)

	at := time.Date(2026, 9, 13, 10, 22, 41, 0, time.UTC)
	if err := db.InsertCLIInvocation(CLIInvocation{
		CreatedAt:     at,
		SessionID:     "sess-1",
		AgentID:       "agent:claude",
		Command:       "issues new",
		Args:          "issues new --json",
		ProjectName:   "harnez",
		WorkingDir:    "/home/u/projects/harnez",
		TicketID:      "harnez/326",
		ExitCode:      intPtr(0),
		DurationMs:    41,
		HarnezVersion: "v0.1.8",
	}); err != nil {
		t.Fatalf("InsertCLIInvocation: %v", err)
	}

	got, err := db.QueryCLIInvocations(Filter{}, 10)
	if err != nil {
		t.Fatalf("QueryCLIInvocations: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d", len(got))
	}
	row := got[0]
	if row.Command != "issues new" {
		t.Errorf("Command = %q, want %q", row.Command, "issues new")
	}
	if row.ExitCode == nil || *row.ExitCode != 0 {
		t.Errorf("ExitCode = %v, want 0", row.ExitCode)
	}
	if row.DurationMs != 41 {
		t.Errorf("DurationMs = %d, want 41", row.DurationMs)
	}
	if !row.CreatedAt.Equal(at) {
		t.Errorf("CreatedAt = %v, want %v", row.CreatedAt, at)
	}
	if row.AgentID != "agent:claude" || row.ProjectName != "harnez" || row.TicketID != "harnez/326" {
		t.Errorf("unexpected row identity fields: %+v", row)
	}
	if row.HarnezVersion != "v0.1.8" {
		t.Errorf("HarnezVersion = %q, want v0.1.8", row.HarnezVersion)
	}
}

// seedInvocations writes n rows one minute apart, oldest first, so ordering
// and limit assertions have a deterministic timeline to work against.
func seedInvocations(t *testing.T, db *DB, rows []CLIInvocation) {
	t.Helper()
	base := time.Date(2026, 9, 13, 9, 0, 0, 0, time.UTC)
	for i, r := range rows {
		if r.CreatedAt.IsZero() {
			r.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		}
		if r.SessionID == "" {
			r.SessionID = "sess"
		}
		if r.AgentID == "" {
			r.AgentID = "unknown"
		}
		if err := db.InsertCLIInvocation(r); err != nil {
			t.Fatalf("seed row %d: %v", i, err)
		}
	}
}

// TestQueryCLIInvocations_OrderAndLimit asserts newest-first ordering and
// that the limit is applied in SQL (issue 327's bounded default).
func TestQueryCLIInvocations_OrderAndLimit(t *testing.T) {
	db := openTestDB(t)
	seedInvocations(t, db, []CLIInvocation{
		{Command: "apply", ExitCode: intPtr(0)},
		{Command: "index", ExitCode: intPtr(0)},
		{Command: "status", ExitCode: intPtr(0)},
	})

	got, err := db.QueryCLIInvocations(Filter{}, 2)
	if err != nil {
		t.Fatalf("QueryCLIInvocations: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows under limit, got %d", len(got))
	}
	if got[0].Command != "status" || got[1].Command != "index" {
		t.Errorf("expected newest-first [status index], got [%s %s]", got[0].Command, got[1].Command)
	}

	all, err := db.QueryCLIInvocations(Filter{}, 0)
	if err != nil {
		t.Fatalf("QueryCLIInvocations (uncapped): %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("limit <= 0 should be uncapped, got %d rows", len(all))
	}
}

// TestQueryCLIInvocations_Filters covers the Filter fields issue 327's
// flags map onto, including that they combine with AND.
func TestQueryCLIInvocations_Filters(t *testing.T) {
	db := openTestDB(t)
	seedInvocations(t, db, []CLIInvocation{
		{Command: "index", ProjectName: "harnez", AgentID: "agent:claude", ExitCode: intPtr(0)},
		{Command: "index", ProjectName: "smarthome", AgentID: "human", ExitCode: intPtr(1)},
		{Command: "apply", ProjectName: "harnez", AgentID: "human", ExitCode: intPtr(1)},
	})

	cases := []struct {
		name   string
		filter Filter
		want   int
	}{
		{"command", Filter{Command: "index"}, 2},
		{"project", Filter{Project: "harnez"}, 2},
		{"agent", Filter{AgentID: "human"}, 2},
		{"failed", Filter{FailedOnly: true}, 2},
		{"command+project", Filter{Command: "index", Project: "harnez"}, 1},
		{"failed+project", Filter{FailedOnly: true, Project: "harnez"}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := db.QueryCLIInvocations(tc.filter, 0)
			if err != nil {
				t.Fatalf("QueryCLIInvocations: %v", err)
			}
			if len(got) != tc.want {
				t.Fatalf("got %d rows, want %d", len(got), tc.want)
			}
			if tc.filter.FailedOnly {
				for _, r := range got {
					if r.ExitCode == nil || *r.ExitCode == 0 {
						t.Errorf("FailedOnly returned a successful row: %+v", r)
					}
				}
			}
		})
	}
}

// TestInsertCLIInvocation_PrunesPastRowCap asserts issue 326's retention
// bound: writing past the cap drops the oldest rows, keeping exactly the
// newest cap rows.
func TestInsertCLIInvocation_PrunesPastRowCap(t *testing.T) {
	db := openTestDB(t)

	const rowCap = 5
	base := time.Date(2026, 9, 13, 9, 0, 0, 0, time.UTC)
	for i := 0; i < rowCap+3; i++ {
		err := db.insertCLIInvocation(CLIInvocation{
			CreatedAt: base.Add(time.Duration(i) * time.Minute),
			SessionID: "sess",
			AgentID:   "unknown",
			Command:   "status",
			Args:      "status",
			ExitCode:  intPtr(i), // doubles as the row's identity below
		}, rowCap)
		if err != nil {
			t.Fatalf("insertCLIInvocation %d: %v", i, err)
		}
	}

	got, err := db.QueryCLIInvocations(Filter{}, 0)
	if err != nil {
		t.Fatalf("QueryCLIInvocations: %v", err)
	}
	if len(got) != rowCap {
		t.Fatalf("expected table pruned to %d rows, got %d", rowCap, len(got))
	}
	// Rows 0,1,2 are the oldest and must be the ones dropped.
	for _, r := range got {
		if r.ExitCode == nil || *r.ExitCode < 3 {
			t.Errorf("expected only rows 3..7 to survive, found %v", r.ExitCode)
		}
	}
}

// TestCLIInvocationCounts groups by command for issue 328's sessionstate
// hand-off.
func TestCLIInvocationCounts(t *testing.T) {
	db := openTestDB(t)
	seedInvocations(t, db, []CLIInvocation{
		{Command: "index", SessionID: "a", ExitCode: intPtr(0)},
		{Command: "index", SessionID: "a", ExitCode: intPtr(0)},
		{Command: "find", SessionID: "a", ExitCode: intPtr(0)},
		{Command: "find", SessionID: "b", ExitCode: intPtr(0)},
	})

	counts, err := db.CLIInvocationCounts(Filter{SessionID: "a"})
	if err != nil {
		t.Fatalf("CLIInvocationCounts: %v", err)
	}
	if counts["index"] != 2 || counts["find"] != 1 {
		t.Fatalf("unexpected counts: %v", counts)
	}
	if len(counts) != 2 {
		t.Fatalf("expected 2 command groups for session a, got %v", counts)
	}
}

// TestCLIInvocations_AdditiveOnPreexistingSchemaV2 asserts issue 326's
// "purely additive, no schemaVersion bump" claim against a database file
// that already exists in the schemaVersion-2 shape *without* the new table:
// reopening it must create cli_invocations rather than fail a version check.
func TestCLIInvocations_AdditiveOnPreexistingSchemaV2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tool_catalog.sqlite")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open (create): %v", err)
	}
	db.Close()

	// Simulate a database written before this ticket landed: same
	// schemaVersion stamp, but no cli_invocations table.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := raw.Exec(`DROP TABLE cli_invocations`); err != nil {
		raw.Close()
		t.Fatalf("drop cli_invocations: %v", err)
	}
	var version int
	if err := raw.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		raw.Close()
		t.Fatalf("read user_version: %v", err)
	}
	raw.Close()
	if version != schemaVersion {
		t.Fatalf("user_version = %d, want %d (additive change must not bump it)", version, schemaVersion)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("Open (pre-existing v%d db missing cli_invocations): %v", schemaVersion, err)
	}
	defer reopened.Close()

	if err := reopened.InsertCLIInvocation(CLIInvocation{
		SessionID: "sess", AgentID: "human", Command: "status", ExitCode: intPtr(0),
	}); err != nil {
		t.Fatalf("InsertCLIInvocation after additive create: %v", err)
	}
}

package telemetry

import (
	"testing"
	"time"
)

// TestInsertIssueSnapshot_RecordsAcrossTwoDistinctPoints seeds two
// snapshots for the same project with different counts and verifies
// QueryIssueSnapshots returns both, oldest first -- issue 228's read-path
// acceptance criterion.
func TestInsertIssueSnapshot_RecordsAcrossTwoDistinctPoints(t *testing.T) {
	db := openTestDB(t)

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	inserted, err := db.InsertIssueSnapshot(IssueStatusSnapshot{
		CreatedAt: t1, ProjectName: "harnez", OpenCount: 10, ClosedCount: 5,
	})
	if err != nil {
		t.Fatalf("InsertIssueSnapshot (1): %v", err)
	}
	if !inserted {
		t.Fatal("expected first snapshot to be inserted")
	}

	inserted, err = db.InsertIssueSnapshot(IssueStatusSnapshot{
		CreatedAt: t2, ProjectName: "harnez", OpenCount: 8, ClosedCount: 8,
	})
	if err != nil {
		t.Fatalf("InsertIssueSnapshot (2): %v", err)
	}
	if !inserted {
		t.Fatal("expected second snapshot (different counts) to be inserted")
	}

	got, err := db.QueryIssueSnapshots("harnez")
	if err != nil {
		t.Fatalf("QueryIssueSnapshots: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 snapshots, got %d: %+v", len(got), got)
	}
	if got[0].OpenCount != 10 || got[0].ClosedCount != 5 {
		t.Errorf("expected oldest-first ordering, got[0] = %+v", got[0])
	}
	if got[1].OpenCount != 8 || got[1].ClosedCount != 8 {
		t.Errorf("expected oldest-first ordering, got[1] = %+v", got[1])
	}
	if !got[0].CreatedAt.Equal(t1) || !got[1].CreatedAt.Equal(t2) {
		t.Errorf("unexpected timestamps: got[0]=%v got[1]=%v", got[0].CreatedAt, got[1].CreatedAt)
	}
}

// TestInsertIssueSnapshot_DedupesIdenticalConsecutiveSnapshots verifies
// that repeated `harnez index` runs against an unchanged issues/ directory
// -- i.e. repeated InsertIssueSnapshot calls with identical counts for the
// same project -- do not grow the history unboundedly (issue 228's
// idempotency acceptance criterion).
func TestInsertIssueSnapshot_DedupesIdenticalConsecutiveSnapshots(t *testing.T) {
	db := openTestDB(t)

	snap := IssueStatusSnapshot{ProjectName: "harnez", OpenCount: 3, ClosedCount: 7, DraftCount: 1, UnknownCount: 0}

	inserted, err := db.InsertIssueSnapshot(snap)
	if err != nil {
		t.Fatalf("InsertIssueSnapshot (1): %v", err)
	}
	if !inserted {
		t.Fatal("expected first snapshot to be inserted")
	}

	for i := 0; i < 3; i++ {
		inserted, err := db.InsertIssueSnapshot(snap)
		if err != nil {
			t.Fatalf("InsertIssueSnapshot (repeat %d): %v", i, err)
		}
		if inserted {
			t.Fatalf("expected repeat %d with identical counts to be deduped (not inserted)", i)
		}
	}

	got, err := db.QueryIssueSnapshots("harnez")
	if err != nil {
		t.Fatalf("QueryIssueSnapshots: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 snapshot after deduped repeats, got %d", len(got))
	}

	// A genuinely different count set for the same project must still insert.
	changed := snap
	changed.OpenCount = 2
	inserted, err = db.InsertIssueSnapshot(changed)
	if err != nil {
		t.Fatalf("InsertIssueSnapshot (changed): %v", err)
	}
	if !inserted {
		t.Fatal("expected a snapshot with different counts to be inserted")
	}
	got, err = db.QueryIssueSnapshots("harnez")
	if err != nil {
		t.Fatalf("QueryIssueSnapshots: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 snapshots after a genuine count change, got %d", len(got))
	}
}

// TestQueryIssueSnapshots_FiltersByProject verifies the --project filter
// mirrors issue 227's harnez stats --project idiom: unfiltered returns
// every project's rows, filtered returns only the matching project's.
func TestQueryIssueSnapshots_FiltersByProject(t *testing.T) {
	db := openTestDB(t)

	if _, err := db.InsertIssueSnapshot(IssueStatusSnapshot{ProjectName: "harnez", OpenCount: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertIssueSnapshot(IssueStatusSnapshot{ProjectName: "voxi", OpenCount: 2}); err != nil {
		t.Fatal(err)
	}

	all, err := db.QueryIssueSnapshots("")
	if err != nil {
		t.Fatalf("QueryIssueSnapshots(\"\"): %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 unfiltered snapshots, got %d", len(all))
	}

	harnezOnly, err := db.QueryIssueSnapshots("harnez")
	if err != nil {
		t.Fatalf("QueryIssueSnapshots(harnez): %v", err)
	}
	if len(harnezOnly) != 1 || harnezOnly[0].ProjectName != "harnez" {
		t.Fatalf("expected 1 harnez-only snapshot, got %+v", harnezOnly)
	}
}

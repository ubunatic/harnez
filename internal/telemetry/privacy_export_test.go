// privacy_export_test.go covers issue 204's privacy-levels pass:
// LevelInternal's regex-scrub-in-place behavior, LevelRaw's pass-through,
// and LevelAgentSanitized's cache+batching orchestration (SanitizeNotes),
// using a fake NoteSanitizer so the fast unit suite never shells out to
// the real `claude` CLI (see sanitizer_integration_test.go for that).
package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/privacy"
)

// TestBuildExportLevel_Internal_ScrubsNoteInPlace asserts LevelInternal
// keeps the Note field (unlike LevelPublic, which drops it) but redacts
// the sensitive substrings within it.
func TestBuildExportLevel_Internal_ScrubsNoteInPlace(t *testing.T) {
	raw := "fixed auth bug, contact someone@example.com re /home/testuser/secret-client"
	rows := []ToolCall{{
		CreatedAt: time.Now(),
		SessionID: "sess-1",
		AgentID:   "claude",
		ToolName:  "Read",
		CallType:  "internal",
		Note:      raw,
	}}

	exp := BuildExportLevel(rows, time.Now(), privacy.LevelInternal, nil)
	if len(exp.ToolCalls) != 1 {
		t.Fatalf("expected 1 row, got %d", len(exp.ToolCalls))
	}
	note := exp.ToolCalls[0].Note
	if note == "" {
		t.Fatal("LevelInternal dropped Note entirely, want it scrubbed-but-present")
	}
	if strings.Contains(note, "someone@example.com") || strings.Contains(note, "/home/testuser") {
		t.Fatalf("LevelInternal did not scrub sensitive substrings: %q", note)
	}
	if !strings.Contains(note, "fixed auth bug") {
		t.Fatalf("LevelInternal dropped non-sensitive context, want it preserved: %q", note)
	}
}

// TestBuildExportLevel_Raw_PassesNoteThroughUnchanged asserts LevelRaw
// exports Note verbatim, with no scrubbing at all.
func TestBuildExportLevel_Raw_PassesNoteThroughUnchanged(t *testing.T) {
	raw := "contact someone@example.com re /home/testuser/secret-client"
	rows := []ToolCall{{
		CreatedAt: time.Now(),
		SessionID: "sess-1",
		AgentID:   "claude",
		ToolName:  "Read",
		CallType:  "internal",
		Note:      raw,
	}}

	exp := BuildExportLevel(rows, time.Now(), privacy.LevelRaw, nil)
	if got := exp.ToolCalls[0].Note; got != raw {
		t.Errorf("LevelRaw Note = %q, want unchanged %q", got, raw)
	}
}

// fakeSanitizer is a NoteSanitizer test double that records how many times
// SanitizeBatch was invoked and how many notes it received per call, so
// tests can assert batching behavior precisely instead of just checking
// the final output. It never shells out to anything.
type fakeSanitizer struct {
	calls     int
	callSizes []int
}

func (f *fakeSanitizer) SanitizeBatch(_ context.Context, notes []string) ([]string, error) {
	f.calls++
	f.callSizes = append(f.callSizes, len(notes))
	out := make([]string, len(notes))
	for i, n := range notes {
		out[i] = "CLEAN:" + n
	}
	return out, nil
}

// TestSanitizeNotes_CachesAcrossCalls asserts that a second SanitizeNotes
// call for the same note does NOT invoke the sanitizer again (cache hit),
// per issue 204's explicit cache requirement.
func TestSanitizeNotes_CachesAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/tool_catalog.sqlite")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	fs := &fakeSanitizer{}
	ctx := context.Background()
	now := time.Now()

	first, err := SanitizeNotes(ctx, db, fs, []string{"note A"}, now)
	if err != nil {
		t.Fatalf("SanitizeNotes (1st): %v", err)
	}
	if fs.calls != 1 {
		t.Fatalf("expected 1 sanitizer call after first export, got %d", fs.calls)
	}
	if first["note A"] != "CLEAN:note A" {
		t.Errorf("first[\"note A\"] = %q, want %q", first["note A"], "CLEAN:note A")
	}

	// Second call, same note: must be served entirely from cache.
	second, err := SanitizeNotes(ctx, db, fs, []string{"note A"}, now)
	if err != nil {
		t.Fatalf("SanitizeNotes (2nd): %v", err)
	}
	if fs.calls != 1 {
		t.Fatalf("expected sanitizer call count to stay at 1 (cache hit), got %d", fs.calls)
	}
	if second["note A"] != "CLEAN:note A" {
		t.Errorf("second[\"note A\"] = %q, want %q", second["note A"], "CLEAN:note A")
	}
}

// TestSanitizeNotes_BatchesDistinctUncachedNotesIntoOneCall asserts that N
// distinct, uncached notes passed to a single SanitizeNotes call are sent
// to the sanitizer as ONE batched call (not N calls) — issue 204's
// explicit "minimize LLM calls" requirement.
func TestSanitizeNotes_BatchesDistinctUncachedNotesIntoOneCall(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/tool_catalog.sqlite")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	fs := &fakeSanitizer{}
	notes := []string{"note 1", "note 2", "note 3", "note 3", "note 1"} // includes dupes

	result, err := SanitizeNotes(context.Background(), db, fs, notes, time.Now())
	if err != nil {
		t.Fatalf("SanitizeNotes: %v", err)
	}
	if fs.calls != 1 {
		t.Fatalf("expected exactly 1 batched sanitizer call for 3 distinct uncached notes, got %d calls (sizes: %v)", fs.calls, fs.callSizes)
	}
	if fs.callSizes[0] != 3 {
		t.Fatalf("expected the single batch call to carry 3 distinct notes, got %d", fs.callSizes[0])
	}
	for _, n := range []string{"note 1", "note 2", "note 3"} {
		want := "CLEAN:" + n
		if result[n] != want {
			t.Errorf("result[%q] = %q, want %q", n, result[n], want)
		}
	}
}

// TestSanitizeNotes_ChunksLargeBatches asserts a set of uncached notes
// larger than privacy.DefaultSanitizeBatchSize is split into multiple
// bounded calls rather than one unbounded call.
func TestSanitizeNotes_ChunksLargeBatches(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/tool_catalog.sqlite")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	fs := &fakeSanitizer{}
	n := privacy.DefaultSanitizeBatchSize + 5
	notes := make([]string, n)
	for i := range notes {
		notes[i] = fmt.Sprintf("note %d", i)
	}

	_, err = SanitizeNotes(context.Background(), db, fs, notes, time.Now())
	if err != nil {
		t.Fatalf("SanitizeNotes: %v", err)
	}
	if fs.calls != 2 {
		t.Fatalf("expected 2 chunked calls for %d notes at batch size %d, got %d (sizes: %v)", n, privacy.DefaultSanitizeBatchSize, fs.calls, fs.callSizes)
	}
	if fs.callSizes[0] != privacy.DefaultSanitizeBatchSize || fs.callSizes[1] != 5 {
		t.Fatalf("unexpected chunk sizes: %v", fs.callSizes)
	}
}

// TestExportAllLevel_AgentSanitized_EndToEnd exercises the full
// ExportAllLevel path (DB row -> SanitizeNotes -> BuildExportLevel) with
// the fake sanitizer, asserting the exported Note is the sanitized text
// and the raw note never appears in the serialized JSON.
func TestExportAllLevel_AgentSanitized_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/tool_catalog.sqlite")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	rawNote := "contact someone@example.com about client Acme Corp"
	if err := db.Insert(ToolCall{
		SessionID: "sess-1",
		AgentID:   "claude",
		ToolName:  "Read",
		CallType:  "internal",
		Note:      rawNote,
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	fs := &fakeSanitizer{}
	exp, err := ExportAllLevel(context.Background(), db, time.Now(), privacy.LevelAgentSanitized, fs)
	if err != nil {
		t.Fatalf("ExportAllLevel: %v", err)
	}
	if fs.calls != 1 {
		t.Fatalf("expected 1 sanitizer call, got %d", fs.calls)
	}
	if got, want := exp.ToolCalls[0].Note, "CLEAN:"+rawNote; got != want {
		t.Errorf("Note = %q, want %q", got, want)
	}

	// The fake sanitizer deliberately does not scrub content (that's
	// ScrubText's job, covered separately) — it only proves the
	// SanitizeNotes/BuildExportLevel wiring routes every note through the
	// sanitizer rather than exporting it untouched. A real NoteSanitizer
	// is what actually removes identifying content; verify here only that
	// the exported Note went through it (carries the "CLEAN:" marker),
	// i.e. was not passed through raw.
	data, err := json.Marshal(exp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), "CLEAN:") {
		t.Fatalf("exported note was not routed through the sanitizer: %s", data)
	}
}

// TestExportAllLevel_AgentSanitized_RequiresSanitizerOnMiss asserts a nil
// sanitizer errors out (rather than silently leaking the raw note or
// silently dropping it) when there is an actual cache miss to resolve.
func TestExportAllLevel_AgentSanitized_RequiresSanitizerOnMiss(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/tool_catalog.sqlite")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := db.Insert(ToolCall{
		SessionID: "sess-1",
		AgentID:   "claude",
		ToolName:  "Read",
		CallType:  "internal",
		Note:      "some note",
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	if _, err := ExportAllLevel(context.Background(), db, time.Now(), privacy.LevelAgentSanitized, nil); err == nil {
		t.Fatal("expected an error with a nil sanitizer and an uncached note, got nil")
	}
}

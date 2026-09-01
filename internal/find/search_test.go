package find

import (
	"testing"
	"testing/fstest"

	"ubunatic.com/harnez/internal/issues"
)

// fixtureFS is the shared "temporary tracker fixture" required by issue
// 158's verification plan: open, closed, blocked, in-progress, draft, and
// archived tickets; a body-only hit; a metadata field that must NOT match;
// two title-exact ties for stable-ordering; and a status suffix.
func fixtureFS() fstest.MapFS {
	return fstest.MapFS{
		"100-open-vram.md": &fstest.MapFile{Data: []byte(
			"# 100 — VRAM Load Panel\n\n**Status**: Open\n\n---\n\nDiscusses vram usage tracking in the load panel.\n")},
		"101-closed-gtt.md": &fstest.MapFile{Data: []byte(
			"# 101 — GTT Memory Cleanup\n\n**Status:** Closed — resolved in abc123\n\n---\n\nHandles gtt memory allocation.\n")},
		"102-blocked.md": &fstest.MapFile{Data: []byte(
			"# 102 — Some Blocked Ticket\n\n**Status:** Blocked — waiting for upstream\n\n---\n\nNo special keywords here.\n")},
		"103-inprogress-body-vram.md": &fstest.MapFile{Data: []byte(
			"# 103 — In Progress Ticket\n\n**Status:** In Progress\n\n---\n\nContains vram deep in the body only.\n")},
		"104-metadata-trap.md": &fstest.MapFile{Data: []byte(
			"# 104 — Metadata Trap\n\n**Status:** Open\n**Related:** vram-mentioned-here-only\n\n---\n\nNothing relevant here.\n")},
		"105-draft.md": &fstest.MapFile{Data: []byte(
			"# 105 — Draft Idea\n\n**Status:** Draft\n\n---\n\nvram draft notes.\n")},
		"106-vram-a.md": &fstest.MapFile{Data: []byte(
			"# 106 — VRAM Tie A\n\n**Status:** Open\n\n---\n\nplain body.\n")},
		"107-vram-b.md": &fstest.MapFile{Data: []byte(
			"# 107 — VRAM Tie B\n\n**Status:** Open\n\n---\n\nplain body.\n")},
		"archive/050-archived-vram.md": &fstest.MapFile{Data: []byte(
			"# 050 — Archived VRAM Ticket\n\n**Status:** Closed\n\n---\n\nvram content here too.\n")},
	}
}

func scanFixture(t *testing.T) []issues.IssueFile {
	t.Helper()
	files, err := issues.ScanFS(fixtureFS(), ".")
	if err != nil {
		t.Fatalf("ScanFS: %v", err)
	}
	return files
}

func mustParse(t *testing.T, q string) *Query {
	t.Helper()
	parsed, err := ParseQuery(q)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", q, err)
	}
	return parsed
}

func numbers(results []Result) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Number
	}
	return out
}

func TestSearch_TitleAndBodyHitsIncludingArchive(t *testing.T) {
	files := scanFixture(t)
	results := Search(files, mustParse(t, "vram"))

	got := numbers(results)
	want := map[string]bool{"100": true, "103": true, "105": true, "106": true, "107": true, "050": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want members of %v", got, want)
	}
	for _, n := range got {
		if !want[n] {
			t.Errorf("unexpected match %q in %v", n, got)
		}
	}
}

func TestSearch_MetadataFieldMustNotMatch(t *testing.T) {
	files := scanFixture(t)
	results := Search(files, mustParse(t, "mentioned-here-only"))
	if len(results) != 0 {
		t.Fatalf("expected metadata-only text to never match, got %v", numbers(results))
	}
}

func TestSearch_TitleRanksAboveBody(t *testing.T) {
	files := scanFixture(t)
	results := Search(files, mustParse(t, "vram"))
	if len(results) == 0 {
		t.Fatal("expected matches")
	}
	// 100 (title exact) must rank ahead of 103 (body-only exact).
	pos := make(map[string]int)
	for i, r := range results {
		pos[r.Number] = i
	}
	if pos["100"] >= pos["103"] {
		t.Errorf("expected title match 100 to rank above body-only match 103; order was %v", numbers(results))
	}
}

func TestSearch_StableTieOrderByNumberThenPath(t *testing.T) {
	files := scanFixture(t)
	results := Search(files, mustParse(t, "vram tie"))
	got := numbers(results)
	if len(got) != 2 || got[0] != "106" || got[1] != "107" {
		t.Fatalf("expected stable [106 107] order, got %v", got)
	}

	// Re-running must reproduce the exact same order (determinism).
	again := numbers(Search(files, mustParse(t, "vram tie")))
	if got[0] != again[0] || got[1] != again[1] {
		t.Fatalf("non-deterministic ordering: %v vs %v", got, again)
	}
}

func TestSearch_ZeroMatches(t *testing.T) {
	files := scanFixture(t)
	results := Search(files, mustParse(t, "zzzznonexistentxx"))
	if len(results) != 0 {
		t.Fatalf("expected zero matches, got %v", numbers(results))
	}
}

func TestSearch_StatusFilter_OpenGroupIncludesInProgressAndBlocked(t *testing.T) {
	files := scanFixture(t)
	results := Search(files, mustParse(t, "status:open vram"))
	got := numbers(results)
	// 100 (Open), 103 (In Progress) both mention vram and qualify for
	// is:open's unresolved group; 106/107 (Open, "vram tie" titles)
	// qualify too. 050 is Closed and 105 is Draft, so they must be
	// excluded even though they mention vram.
	want := map[string]bool{"100": true, "103": true, "106": true, "107": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want members of %v", got, want)
	}
	for _, n := range got {
		if !want[n] {
			t.Errorf("status:open leaked non-open ticket %q into %v", n, got)
		}
	}
}

func TestSearch_StatusFilter_InProgressAndBlockedAreNarrow(t *testing.T) {
	files := scanFixture(t)

	inProgress := numbers(Search(files, mustParse(t, "is:in-progress")))
	if len(inProgress) != 1 || inProgress[0] != "103" {
		t.Fatalf("is:in-progress: got %v, want [103]", inProgress)
	}

	blocked := numbers(Search(files, mustParse(t, "is:blocked")))
	if len(blocked) != 1 || blocked[0] != "102" {
		t.Fatalf("is:blocked: got %v, want [102]", blocked)
	}
}

func TestSearch_StatusFilter_ClosedIgnoresSuffixAndCase(t *testing.T) {
	files := scanFixture(t)
	results := Search(files, mustParse(t, "status:closed"))
	got := numbers(results)
	want := map[string]bool{"101": true, "050": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want members of %v", got, want)
	}
	for _, n := range got {
		if !want[n] {
			t.Errorf("unexpected closed match %q in %v", n, got)
		}
	}
}

func TestSearch_StatusFilter_Draft(t *testing.T) {
	files := scanFixture(t)
	got := numbers(Search(files, mustParse(t, "is:draft")))
	if len(got) != 1 || got[0] != "105" {
		t.Fatalf("is:draft: got %v, want [105]", got)
	}
}

func TestSearch_AndDoesNotRelaxToOrOnZeroResults(t *testing.T) {
	files := scanFixture(t)
	// "gtt" only appears in 101's body; "zzznotfound" appears nowhere. An
	// AND of the two must return nothing -- never silently degrade to an
	// OR that would surface 101 via the "gtt" half alone.
	results := Search(files, mustParse(t, "is:open zzznotfound"))
	if len(results) != 0 {
		t.Fatalf("AND query must not relax to OR: got %v", numbers(results))
	}

	results2 := Search(files, mustParse(t, "gtt zzznotfound"))
	if len(results2) != 0 {
		t.Fatalf("AND query must not relax to OR: got %v", numbers(results2))
	}
}

func TestFormatTSV(t *testing.T) {
	r := Result{Number: "100", RawStatus: "Open", PlainTitle: "VRAM Load Panel", Path: "issues/100-open-vram.md"}
	got := FormatTSV(r)
	want := "100\tOpen\tVRAM Load Panel\tissues/100-open-vram.md"
	if got != want {
		t.Errorf("FormatTSV = %q, want %q", got, want)
	}
}

func TestFormatTSV_SanitizesEmbeddedTabsAndNewlines(t *testing.T) {
	r := Result{Number: "100", RawStatus: "Open\nnote", PlainTitle: "Title\twith\ttabs", Path: "issues/100.md"}
	got := FormatTSV(r)
	want := "100\tOpen note\tTitle with tabs\tissues/100.md"
	if got != want {
		t.Errorf("FormatTSV = %q, want %q", got, want)
	}
}

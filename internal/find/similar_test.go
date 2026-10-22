package find

import (
	"fmt"
	"path/filepath"
	"testing"

	"ubunatic.com/harnez/internal/issues"
)

func TestSimilar_ORTitleWeightedAcrossStatuses(t *testing.T) {
	files := []issues.IssueFile{
		{Number: "010", Title: "010 — agent release playbook", RawStatus: "Open", Body: "release"},
		{Number: "011", Title: "011 — unrelated", RawStatus: "Backlog", Body: "agent"},
		{Number: "012", Title: "012 — agent tooling", RawStatus: "Closed — done"},
	}

	got, err := Similar(files, "agent release playbook")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("Similar returned %d results, want 3: %+v", len(got), got)
	}
	if got[0].Number != "010" || got[0].Score != 1 || !got[0].Strong {
		t.Fatalf("exact title match = %+v, want strongest result", got[0])
	}
	if got[1].Number != "012" || got[1].RawStatus != "Closed — done" {
		t.Fatalf("all-status result was lost or status omitted: %+v", got)
	}
	if got[2].Number != "011" || got[1].titleMatches <= got[2].titleMatches {
		t.Fatalf("title match should win equal-score tie against body-only result: %+v", got)
	}
}

func TestSimilar_RareQueryTokenOutweighsCommonToken(t *testing.T) {
	files := []issues.IssueFile{
		{Number: "001", Title: "001 — rareterm widget"},
		{Number: "002", Title: "002 — common widget"},
		{Number: "003", Title: "003 — other", Body: "common"},
		{Number: "004", Title: "004 — unrelated"},
	}

	got, err := Similar(files, "rareterm common")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 || got[0].Number != "001" || got[1].Number != "002" {
		t.Fatalf("rare-word match should outrank common-word match: %+v", got)
	}
	if got[0].Score <= got[1].Score {
		t.Fatalf("rare-word score %.3f should exceed common-word score %.3f", got[0].Score, got[1].Score)
	}
}

func TestSimilar_RecallLiveReleaseGuidancePair697698(t *testing.T) {
	files, err := issues.Scan(filepath.Join("..", "..", "issues"))
	if err != nil {
		t.Fatal(err)
	}
	var source *issues.IssueFile
	for i := range files {
		if files[i].Number == "697" {
			source = &files[i]
			break
		}
	}
	if source == nil {
		t.Fatal("live issue 697 is missing")
	}
	results, err := Similar(files, issues.StripTicketNumber(source.Title))
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if result.Number == "698" {
			if !result.Strong {
				t.Fatalf("live issue 698 scored %.3f, below the strong threshold", result.Score)
			}
			return
		}
	}
	t.Fatalf("live issue 698 was not recalled in the top results: %+v", results)
}

func TestSimilar_RejectsShortAndStopwordOnlyQueries(t *testing.T) {
	for _, query := range []string{"release", "the and of", "x & y"} {
		t.Run(query, func(t *testing.T) {
			if _, err := Similar(nil, query); err == nil {
				t.Fatalf("Similar(%q) succeeded, want insufficient meaningful terms", query)
			}
		})
	}
}

func TestSimilar_UsesSpecResultLimit(t *testing.T) {
	policy, err := issues.DuplicateGateSettings()
	if err != nil {
		t.Fatal(err)
	}
	files := make([]issues.IssueFile, 9)
	for i := range files {
		files[i] = issues.IssueFile{Number: fmt.Sprintf("%03d", i+1), Title: "Agent release guide"}
	}
	got, err := Similar(files, "agent release playbook")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != policy.MaxResults {
		t.Fatalf("Similar returned %d results, want spec limit %d", len(got), policy.MaxResults)
	}
}

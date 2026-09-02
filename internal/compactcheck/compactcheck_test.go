package compactcheck

import (
	"strings"
	"testing"
)

func TestCompare_FlagsDroppedIdentifier(t *testing.T) {
	before := `
We are debugging internal/apply/apply.go and the failure is in
internal/apply/apply.go's handleSection function. Ran internal/apply/apply.go
three times to confirm. Separately looked at cmd/harnez/rate.go once.
`
	after := `
Investigated a bug in the apply path and confirmed the fix works.
`
	r := Compare(before, after)

	if r.BeforeBytes != len(before) || r.AfterBytes != len(after) {
		t.Fatalf("unexpected byte counts: before=%d after=%d", r.BeforeBytes, r.AfterBytes)
	}
	if r.ReductionRatio <= 0 {
		t.Errorf("expected a positive reduction ratio, got %f", r.ReductionRatio)
	}

	found := false
	for _, id := range r.DroppedIdentifiers {
		if strings.Contains(id, "apply.go") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected internal/apply/apply.go (mentioned 3x) to be flagged as dropped, got: %v", r.DroppedIdentifiers)
	}

	// cmd/harnez/rate.go was mentioned only once in before, below
	// minMentions, so it must NOT be flagged even though it also vanished.
	for _, id := range r.DroppedIdentifiers {
		if strings.Contains(id, "rate.go") {
			t.Errorf("did not expect a once-mentioned identifier to be flagged, got: %v", r.DroppedIdentifiers)
		}
	}
}

func TestCompare_NothingDroppedWhenIdentifierSurvives(t *testing.T) {
	before := "check internal/apply/apply.go and internal/apply/apply.go again"
	after := "summary: fixed internal/apply/apply.go"

	r := Compare(before, after)
	if len(r.DroppedIdentifiers) != 0 {
		t.Errorf("expected no dropped identifiers when the identifier survives, got: %v", r.DroppedIdentifiers)
	}
}

func TestCompare_EmptyBefore(t *testing.T) {
	r := Compare("", "something")
	if r.ReductionRatio != 0 {
		t.Errorf("expected ReductionRatio 0 for empty before text, got %f", r.ReductionRatio)
	}
	if len(r.DroppedIdentifiers) != 0 {
		t.Errorf("expected no dropped identifiers for empty before text, got: %v", r.DroppedIdentifiers)
	}
}

func TestFormatReport_ListsDropped(t *testing.T) {
	r := Report{BeforeBytes: 100, AfterBytes: 20, ReductionRatio: 0.8, DroppedIdentifiers: []string{"internal/apply/apply.go"}}
	out := FormatReport(r)
	if !strings.Contains(out, "internal/apply/apply.go") {
		t.Errorf("expected formatted report to list the dropped identifier, got:\n%s", out)
	}
	if !strings.Contains(out, "80%") {
		t.Errorf("expected formatted report to state the reduction percentage, got:\n%s", out)
	}
}

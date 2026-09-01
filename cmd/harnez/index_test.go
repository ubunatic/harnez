package main

import (
	"bytes"
	"strings"
	"testing"
)

// printUnifiedDiff is what makes `harnez index --check` show the specific
// drift inline (not just "would update <path>"), so an agent running the
// command in-session can see exactly what changed and act on it directly.
func TestPrintUnifiedDiff_ShowsChanges(t *testing.T) {
	var out bytes.Buffer
	old := []byte("| 148 | ... | old title | Open |\n")
	new_ := []byte("| 148 | ... | new title | Open |\n")

	if err := printUnifiedDiff(&out, "issues/README.md", old, new_); err != nil {
		t.Fatalf("printUnifiedDiff: %v", err)
	}

	got := out.String()
	for _, want := range []string{"-| 148 | ... | old title | Open |", "+| 148 | ... | new title | Open |", "issues/README.md"} {
		if !strings.Contains(got, want) {
			t.Errorf("diff output missing %q, got:\n%s", want, got)
		}
	}
}

func TestPrintUnifiedDiff_NoChangesNoOutput(t *testing.T) {
	var out bytes.Buffer
	same := []byte("identical content\n")

	if err := printUnifiedDiff(&out, "docs/README.md", same, same); err != nil {
		t.Fatalf("printUnifiedDiff: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no diff output for identical content, got:\n%s", out.String())
	}
}

package markdown

import (
	"strings"
	"testing"
)

func TestReconcileSectionsPreservesUserText(t *testing.T) {
	block := "<!-- harnez:begin kept -->\nstale\n<!-- harnez:end kept -->\n"
	user := "<!-- unrelated:begin kept -->\nuser\n<!-- unrelated:end kept -->\n"
	got, err := ReconcileSections("before\n"+block+user+block+"after", []ManagedSection{{Name: "kept", Content: "fresh"}})
	want := "before\n" + strings.Replace(block, "stale", "fresh", 1) + user + "after"
	if err != nil || got != want {
		t.Fatalf("reconcile = %q, %v; want %q", got, err, want)
	}
	malformed := "before\n<!-- harnez:begin incomplete -->\nuser text"
	got, err = ReconcileSections(malformed, nil)
	if err != nil || got != malformed {
		t.Fatalf("malformed block changed: %q, %v", got, err)
	}
	sections := []ManagedSection{{Name: "new", Content: "new content"}}
	first, err := ReconcileSections(malformed, sections)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReconcileSections(first, sections)
	if err != nil || second != first {
		t.Fatalf("malformed block caused repeated additions: %q, %v", second, err)
	}
}

func TestReconcileSectionsKeepsUnknownHarnezBlocks(t *testing.T) {
	const existing = "<!-- harnez:begin Instructions Hierarchy -->\nlegacy hierarchy\n<!-- harnez:end Instructions Hierarchy -->\n" +
		"<!-- harnez:begin Issue Tracker Discovery -->\nlegacy tracker\n<!-- harnez:end Issue Tracker Discovery -->\n" +
		"<!-- harnez:begin Agent-Filed Feedback -->\nlegacy feedback\n<!-- harnez:end Agent-Filed Feedback -->\n"
	got, err := ReconcileSections(existing, []ManagedSection{{Name: "Process hygiene", Content: "new rule"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, existing) || strings.Count(got, "<!-- harnez:begin ") != 4 {
		t.Fatalf("unknown blocks changed or were deleted: %q", got)
	}
}

func TestReconcileSectionsRejectsInvalidConfig(t *testing.T) {
	for _, sections := range [][]ManagedSection{
		{{Name: ""}},
		{{Name: "bad\nname"}},
		{{Name: "same"}, {Name: "same"}},
		{{Name: "nested", Content: "<!-- harnez:begin another -->"}},
	} {
		if _, err := ReconcileSections("user text", sections); err == nil {
			t.Fatalf("accepted invalid sections: %+v", sections)
		}
	}
}

package assess

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAssessPathDeterministic(t *testing.T) {
	root := t.TempDir()
	// Repeated basenames and suffixes must keep independent full paths. Counts
	// deliberately differ so neither sorting by count nor merging directories works.
	dirs := []struct {
		path  string
		count int
	}{
		{".", 3},
		{"alpha/nested", 4},
		{"assets/book", 3},
		{"website/book", 5},
		{"website/book/assets/book", 4},
		{"zeta", 3},
	}
	write := func(name, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	var wantWarnings []string
	for _, dir := range dirs {
		for i := range dir.count {
			write(fmt.Sprintf("%s/file%d.go", dir.path, i), "package fixture\n")
		}
		wantWarnings = append(wantWarnings, fmt.Sprintf("Directory %s contains %d code files but no test files", dir.path, dir.count))
	}
	// Equal-sized language groups exercise the category sort's tie-breaker.
	write("one.py", "print('hello')\n")
	write("one.rs", "fn main() {}\n")
	wantWarnings[0] = "Directory . contains 5 code files but no test files"
	// A test in a sibling with the same basename must only suppress that sibling.
	for i := range 3 {
		write(fmt.Sprintf("tested/book/file%d.go", i), "package fixture\n")
	}
	write("tested/book/file_test.go", "package fixture\n")
	write("small/one.go", "package fixture\n")

	var firstText, firstJSON string
	for run := range 50 {
		report, err := AssessPath(root)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(report.Warnings, wantWarnings) {
			t.Fatalf("run %d warnings = %v; want %v", run, report.Warnings, wantWarnings)
		}
		for i := 1; i < len(report.Categories); i++ {
			prev, curr := report.Categories[i-1], report.Categories[i]
			if prev.Lines < curr.Lines || (prev.Lines == curr.Lines && prev.Name >= curr.Name) {
				t.Fatalf("categories out of order: %v, %v", prev, curr)
			}
		}
		text := RenderText(report)
		jsonText, err := RenderJSON(report)
		if err != nil {
			t.Fatal(err)
		}
		if run == 0 {
			firstText, firstJSON = text, jsonText
		} else if text != firstText || jsonText != firstJSON {
			t.Fatalf("run %d differs from first assessment (text: %t, JSON: %t)", run, text != firstText, jsonText != firstJSON)
		}
	}
}

func TestStackedBarDeterministicTies(t *testing.T) {
	// Unknown categories enter through a map too; include them in the tie.
	for run := range 50 {
		got := RenderStackedBar(map[string]int{"root": 10, "docs": 10, "custom": 10}, 30, 6, false)
		if want := "●●◆◆■■"; got != want {
			t.Fatalf("run %d bar = %q; want %q", run, got, want)
		}
	}
}

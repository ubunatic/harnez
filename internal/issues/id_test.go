package issues

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeID(t *testing.T) {
	for arg, want := range map[string]string{"GH42": "GH42", "gh42": "GH42", "42": "042", "042": "042", "0": "000"} {
		got, err := NormalizeID(arg)
		if err != nil || got != want {
			t.Errorf("%q: %q %v", arg, got, err)
		}
	}
	for _, arg := range []string{"GH", "GH0", "GH042", "gh-42", "-42", "42x"} {
		if _, err := NormalizeID(arg); err == nil {
			t.Errorf("accepted %q", arg)
		}
	}
}

func TestGHReservationDuplicateAndLint(t *testing.T) {
	dir := t.TempDir()
	num, file, err := Reserve(dir, ReserveOptions{Title: "Cloud", RemoteID: "GH42"})
	if err != nil || num != "GH42" || file != "GH42-cloud.md" {
		t.Fatalf("reserve: %s %s %v", num, file, err)
	}
	if _, _, err := Reserve(dir, ReserveOptions{Title: "Another title", RemoteID: "GH42"}); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate reservation: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil || ParseHeaderNumber(string(body)) != "GH42" || StripTicketNumber("GH42 — Cloud") != "Cloud" {
		t.Fatalf("header: %s %v", body, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "GH42-other.md"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Issues\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Lint(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range report.Diagnostics {
		if d.Kind == DiagDuplicateNumber && d.IssueNum == "GH42" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing GH duplicate: %+v", report.Diagnostics)
	}
	files, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if next := NextNumberFromFiles(files); next != "001" {
		t.Fatalf("GH-only next %s", next)
	}
}

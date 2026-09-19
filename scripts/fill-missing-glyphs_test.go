package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFillFileAppendsMissingGlyphsAndPreservesComments(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "glyphs.yaml")
	bdfPath := filepath.Join(dir, "font.bdf")
	original := "# keep this comment\nglyphs:\n  '?':\n  - 'a''b'\n  A:\n  - existing\n"
	if err := os.WriteFile(specPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bdfPath, []byte("ENCODING 66\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := fillFile(specPath, bdfPath, "?A'B", false)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("fillFile reported no change")
	}
	got, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	want := original + "  '''':\n  - 'a''b'\n"
	if string(got) != want {
		t.Fatalf("file =\n%s\nwant\n%s", got, want)
	}
	if !strings.HasPrefix(string(got), original) {
		t.Fatal("existing YAML was not preserved byte-for-byte")
	}

	var parsed glyphSpec
	if err := yaml.Unmarshal(got, &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed.Glyphs["B"]; ok {
		t.Fatal("upstream glyph B should not be added")
	}
	if got := parsed.Glyphs["'"]; len(got) != 1 || got[0] != "a'b" {
		t.Fatalf("apostrophe glyph = %#v", got)
	}
}

func TestFillFileReportsMissingUpstream(t *testing.T) {
	path := filepath.Join(t.TempDir(), "glyphs.yaml")
	if err := os.WriteFile(path, []byte("glyphs:\n  '?':\n  - 'x'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := fillFile(path, filepath.Join(t.TempDir(), "missing.bdf"), "A", false)
	if err == nil {
		t.Fatal("fillFile succeeded with a missing upstream BDF")
	}
}

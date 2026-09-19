package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestImportBDF(t *testing.T) {
	dir := t.TempDir()
	spec := "charset: AB\nglyphs:\n  A:\n    3x5:\n      - old\n  B:\n    3x5:\n      - old\n"
	bdf := "STARTFONT 2.1\nCHARS 3\nSTARTCHAR A\nENCODING 65\nBBX 3 3 0 2\nBITMAP\n80\nA0\nE0\nENDCHAR\nSTARTCHAR B\nENCODING 66\nBBX 3 2 0 3\nBITMAP\nE0\nA0\nENDCHAR\nSTARTCHAR C\nENCODING 67\nBBX 3 1 0 0\nBITMAP\nE0\nENDCHAR\nENDFONT\n"
	specPath := filepath.Join(dir, "glyphs.yaml")
	bdfPath := filepath.Join(dir, "fixture.bdf")
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bdfPath, []byte(bdf), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "import-bdf-font.go", "-input", bdfPath, "-output", specPath, "-size", "3x5")
	cmd.Dir = "."
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("importer failed: %v\n%s", err, output)
	}
	got, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Glyphs map[string]map[string][]string `yaml:"glyphs"`
	}
	if err := yaml.Unmarshal(got, &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed.Glyphs["C"]; ok {
		t.Error("glyph C is outside the spec charset and must not be imported")
	}
	want := map[string][]string{
		"A": {"1  ", "1 1", "111", "   ", "   "},
		"B": {"111", "1 1", "   ", "   ", "   "},
	}
	for glyph, rows := range want {
		gotRows := parsed.Glyphs[glyph]["3x5"]
		if len(gotRows) != len(rows) {
			t.Errorf("%s rows = %#v, want %#v", glyph, gotRows, rows)
			continue
		}
		for i := range rows {
			if gotRows[i] != rows[i] {
				t.Errorf("%s row %d = %q, want %q", glyph, i, gotRows[i], rows[i])
			}
		}
	}
}

// TestImportBDFAnchorsBaselineAndKeepsFile checks that glyphs are placed via
// FONTBOUNDINGBOX and that untouched entries and comments survive byte-for-byte.
func TestImportBDFAnchorsBaselineAndKeepsFile(t *testing.T) {
	dir := t.TempDir()
	keep := "# header comment\ncharset: \"AB\"\nglyphs:\n  \"B\":\n    3x5:\n      - \"1  \"\n"
	specPath := filepath.Join(dir, "glyphs.yaml")
	bdfPath := filepath.Join(dir, "fixture.bdf")
	// Cell 3x4, baseline 1 row above the cell bottom: ascent = 4 + (-1) = 3.
	bdf := "STARTFONT 2.1\nFONTBOUNDINGBOX 3 4 0 -1\nSTARTCHAR A\nENCODING 65\nBBX 3 2 0 0\nBITMAP\nE0\nA0\nENDCHAR\nENDFONT\n"
	if err := os.WriteFile(specPath, []byte(keep), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bdfPath, []byte(bdf), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "import-bdf-font.go", "-input", bdfPath, "-output", specPath, "-size", "3x4")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("importer failed: %v\n%s", err, output)
	}
	got, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	// A sits on the baseline: rows 1-2 of the 4-row cell.
	want := keep + "  \"A\":\n    3x4:\n      - \"   \"\n      - \"111\"\n      - \"1 1\"\n      - \"   \"\n"
	if string(got) != want {
		t.Errorf("spec =\n%s\nwant\n%s", got, want)
	}
}

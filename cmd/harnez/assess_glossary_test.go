package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssessGlossaryCommand(t *testing.T) {
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	docFile := filepath.Join(docsDir, "Guide.md")
	if err := os.WriteFile(docFile, []byte("Avoid saying context used in docs.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("text output on directory", func(t *testing.T) {
		var out bytes.Buffer
		cmd := newAssessCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--glossary", dir})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		got := out.String()
		if !strings.Contains(got, "Glossary sources: spec/glossary.yaml") {
			t.Errorf("output missing sources line:\n%s", got)
		}
		if !strings.Contains(got, "Active terms (4):") {
			t.Errorf("output missing active terms line:\n%s", got)
		}
		if !strings.Contains(got, `docs/Guide.md:1:14: forbidden term "context used"`) {
			t.Errorf("output missing finding line:\n%s", got)
		}
	})

	t.Run("json output on directory", func(t *testing.T) {
		var out bytes.Buffer
		cmd := newAssessCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--glossary", "--json", dir})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		var rep AssessGlossaryReport
		if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
			t.Fatalf("json.Unmarshal error = %v\nraw:\n%s", err, out.String())
		}
		if rep.TermCount != 4 {
			t.Errorf("TermCount = %d, want 4", rep.TermCount)
		}
		if len(rep.Findings) != 1 || rep.Findings[0].Phrase != "context used" {
			t.Errorf("Findings = %+v, want 1 finding for 'context used'", rep.Findings)
		}
	})
}

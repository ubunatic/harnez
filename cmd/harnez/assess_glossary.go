package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/glossary"
)

// AssessGlossaryReport is the structured output of `harnez assess --glossary`.
type AssessGlossaryReport struct {
	Path      string             `json:"path"`
	Sources   []string           `json:"sources"`
	TermCount int                `json:"term_count"`
	Terms     []string           `json:"terms"`
	Findings  []glossary.Finding `json:"findings"`
}

func buildAssessGlossaryReport(targetPath string) (*AssessGlossaryReport, error) {
	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, err
	}

	repoDir := targetPath
	if !info.IsDir() {
		repoDir = findRepoRootForFile(targetPath)
	}

	g, err := glossary.LoadForRepo(repoDir)
	if err != nil {
		return nil, err
	}

	var findings []glossary.Finding
	if info.IsDir() {
		findings, err = g.CheckDir(targetPath)
		if err != nil {
			return nil, err
		}
	} else {
		content, err := os.ReadFile(targetPath)
		if err != nil {
			return nil, err
		}
		rel, relErr := filepath.Rel(repoDir, targetPath)
		if relErr != nil || strings.HasPrefix(rel, "..") {
			rel = targetPath
		}
		findings = g.CheckFile(rel, content)
		for i := range findings {
			findings[i].File = targetPath
		}
	}

	if findings == nil {
		findings = []glossary.Finding{}
	}

	terms := make([]string, 0, len(g.Terms))
	for _, t := range g.Terms {
		if t.IsEnabled() {
			terms = append(terms, t.Term)
		}
	}

	return &AssessGlossaryReport{
		Path:      targetPath,
		Sources:   g.Sources,
		TermCount: len(terms),
		Terms:     terms,
		Findings:  findings,
	}, nil
}

func findRepoRootForFile(filePath string) string {
	dir := filepath.Dir(filePath)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".harnez")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Dir(filePath)
		}
		dir = parent
	}
}

func runAssessGlossary(out io.Writer, targetPath string, asJSON bool) error {
	rep, err := buildAssessGlossaryReport(targetPath)
	if err != nil {
		return fmt.Errorf("assess --glossary %s: %w", targetPath, err)
	}
	if asJSON {
		data, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return fmt.Errorf("render json: %w", err)
		}
		fmt.Fprintln(out, string(data))
		return nil
	}
	fmt.Fprint(out, renderAssessGlossaryText(rep))
	return nil
}

func renderAssessGlossaryText(rep *AssessGlossaryReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Glossary sources: %s\n", strings.Join(rep.Sources, ", "))
	fmt.Fprintf(&b, "Active terms (%d): %s\n", rep.TermCount, strings.Join(rep.Terms, ", "))
	if len(rep.Findings) == 0 {
		b.WriteString("Findings: 0\n")
		return b.String()
	}
	fmt.Fprintf(&b, "Findings (%d):\n", len(rep.Findings))
	for _, f := range rep.Findings {
		fmt.Fprintf(&b, "  %s\n", f.FormatUnix())
	}
	return b.String()
}

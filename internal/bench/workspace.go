package bench

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/readcard"
)

// Condition is the documentation delivery a run is measured under.
type Condition struct {
	Docs  string // "full" or "lite"
	Cards bool   // deliver docs as PNG context cards instead of Markdown
}

// Label is the stable text form recorded in the bench DB.
func (c Condition) Label() string {
	if c.Cards {
		return c.Docs + "+cards"
	}
	return c.Docs
}

// ParseDocs validates a --docs value.
func ParseDocs(s string) (string, error) {
	switch s {
	case "full", "lite":
		return s, nil
	}
	return "", fmt.Errorf("bench: unknown docs mode %q (expected full or lite)", s)
}

// docPath resolves a doc under repoRoot, preferring the .lite variant in lite mode.
func docPath(repoRoot, rel, docs string) (string, error) {
	if docs == "lite" {
		lite := strings.TrimSuffix(rel, filepath.Ext(rel)) + ".lite" + filepath.Ext(rel)
		if _, err := os.Stat(filepath.Join(repoRoot, lite)); err == nil {
			rel = lite
		}
	}
	p := filepath.Join(repoRoot, filepath.FromSlash(rel))
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("bench: doc %s: %w", rel, err)
	}
	return p, nil
}

// StageWorkspace writes AGENTS.md and CLAUDE.md into dir, plus each doc as
// Markdown or a `harnez read -I` style PNG card. It returns the delivered
// file names so callers can record what the agent was given.
func StageWorkspace(dir, repoRoot string, spec *Spec, task Task, cond Condition) ([]string, error) {
	rels := append(append([]string{}, spec.BaseDocs...), task.Docs...)
	var delivered []string
	var refs strings.Builder
	for _, rel := range rels {
		src, err := docPath(repoRoot, rel, cond.Docs)
		if err != nil {
			return nil, err
		}
		names := []string{filepath.Base(src)}
		if cond.Cards {
			base := strings.TrimSuffix(names[0], filepath.Ext(names[0])) + ".png"
			if names, err = renderCards(src, filepath.Join(dir, base)); err != nil {
				return nil, err
			}
		} else {
			data, err := os.ReadFile(src)
			if err != nil {
				return nil, err
			}
			if err := os.WriteFile(filepath.Join(dir, names[0]), data, 0o644); err != nil {
				return nil, err
			}
		}
		for _, name := range names {
			delivered = append(delivered, name)
			fmt.Fprintf(&refs, "@%s\n", name)
		}
	}
	body := strings.TrimRight(spec.Preamble, "\n") + "\n\n" + refs.String()
	for _, f := range []string{"AGENTS.md", "CLAUDE.md"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(body), 0o644); err != nil {
			return nil, err
		}
	}
	return delivered, nil
}

// renderCards renders src to one or more PNG pages and returns their file names in dir.
func renderCards(src, out string) ([]string, error) {
	res, err := readcard.ReadFile(src, readcard.TextOptions{})
	if err != nil {
		return nil, fmt.Errorf("bench: read %s: %w", src, err)
	}
	rendered, err := readcard.RenderFileToCards(res.Lines, src, readcard.RenderOptions{
		ShowLineNumbers: true, OutputPath: out, Title: filepath.Base(src),
		SourceLines: res.SourceLines, StartLine: res.StartLine, SourceTokens: res.TokenStats.TextTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("bench: render %s: %w", src, err)
	}
	if len(rendered.Files) == 0 {
		return nil, fmt.Errorf("bench: render %s produced no cards", src)
	}
	names := make([]string, len(rendered.Files))
	for i, f := range rendered.Files {
		names[i] = filepath.Base(f)
	}
	return names, nil
}

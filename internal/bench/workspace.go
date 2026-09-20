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
	Read  string // "", or how fixtures are read: native, text or auto
	Yaml  bool   // read conditions: deliver the fixture as YAML instead of Markdown
	Multi int    // read conditions: split the fixture into this many files (<2: one file)
	Card  string // auto read: extra `harnez read` card flags the agent is told to use, e.g. "--style=compact"
}

// Label is the stable text form recorded in the bench DB.
func (c Condition) Label() string {
	if c.Read != "" {
		return "read:" + c.ReadVariant()
	}
	if c.Cards {
		return c.Docs + "+cards"
	}
	return c.Docs
}

// ReadVariant names the read mode plus its fixture shape, e.g. "auto+yaml+multi5".
// It is what the bench DB records in read_mode.
func (c Condition) ReadVariant() string {
	v := c.Read
	if c.Yaml {
		v += "+yaml"
	}
	if c.Multi >= 2 {
		v += fmt.Sprintf("+multi%d", c.Multi)
	}
	if c.Card != "" {
		v += "+" + strings.ReplaceAll(strings.ReplaceAll(c.Card, "--", ""), " ", ",")
	}
	return v
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
	if cond.Read != "" {
		return stageFixtures(dir, spec, task, cond)
	}
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

// stageFixtures writes the task's fixtures to dir/docs and an AGENTS.md and
// CLAUDE.md holding only the read-mode instruction: the agent sees no other docs.
func stageFixtures(dir string, spec *Spec, task Task, cond Condition) ([]string, error) {
	mode := cond.Read
	if len(task.Fixtures) == 0 {
		return nil, fmt.Errorf("bench: task %q has no fixtures for read mode %q", task.ID, mode)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		return nil, err
	}
	var delivered []string
	var body strings.Builder
	card := ""
	if cond.Card != "" {
		card = " " + cond.Card
	}
	instr := strings.ReplaceAll(spec.ReadModes[mode], "{{card}}", card)
	body.WriteString(strings.TrimRight(instr, "\n") + "\n\nDocs:\n")
	for _, name := range task.Fixtures {
		files, _ := fixtureFiles(name, cond.Yaml, cond.Multi)
		for _, f := range files {
			if err := os.WriteFile(filepath.Join(dir, "docs", f.Name), []byte(f.Content), 0o644); err != nil {
				return nil, err
			}
			delivered = append(delivered, "docs/"+f.Name)
			fmt.Fprintf(&body, "- docs/%s\n", f.Name)
		}
	}
	for _, f := range []string{"AGENTS.md", "CLAUDE.md"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(body.String()), 0o644); err != nil {
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

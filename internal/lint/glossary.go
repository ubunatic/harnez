package lint

import (
	"ubunatic.com/harnez/internal/glossary"
)

// GlossaryRule checks files against the Harnez machine-readable glossary.
type GlossaryRule struct {
	glossary *glossary.Glossary
}

// NewGlossaryRule constructs a GlossaryRule for the given Glossary.
// If g is nil, it loads the glossary for the current working directory (falling back to embedded base).
func NewGlossaryRule(g *glossary.Glossary) *GlossaryRule {
	if g == nil {
		loaded, err := glossary.LoadForRepo(".")
		if err != nil {
			loaded, _ = glossary.Load()
		}
		g = loaded
	}
	return &GlossaryRule{glossary: g}
}

func (r *GlossaryRule) ID() string {
	return glossary.RuleID
}

func (r *GlossaryRule) Description() string {
	return "Check wording and comments against the repository glossary"
}

func (r *GlossaryRule) Languages() []Language {
	return []Language{LangMarkdown, LangGo}
}

func (r *GlossaryRule) Check(file string, content []byte) []Finding {
	if r == nil || r.glossary == nil {
		return nil
	}
	gf := r.glossary.CheckFile(file, content)
	if len(gf) == 0 {
		return nil
	}
	out := make([]Finding, 0, len(gf))
	for _, f := range gf {
		out = append(out, Finding{
			File:     file,
			Line:     f.Line,
			Col:      f.Col,
			RuleID:   glossary.RuleID,
			Severity: SeverityError,
			Message:  f.FormatMessage(),
		})
	}
	return out
}

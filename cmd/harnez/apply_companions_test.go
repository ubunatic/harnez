package main

import (
	"testing"

	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/companion"
)

// Every profile in spec/tools.yaml must name real config docs and components.
func TestProfilesReferenceKnownDocsAndComponents(t *testing.T) {
	cfg, _, err := claude.OpenConfig("")
	if err != nil {
		t.Fatal(err)
	}
	spec, err := companion.LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	for name, p := range spec.Profiles {
		for _, d := range p.Docs {
			if _, ok := cfg.AgentsMD.Languages[d]; !ok {
				t.Errorf("profile %s: unknown doc %q", name, d)
			}
		}
		if len(p.Components) > 0 {
			if _, err := resolveComponentSelection(cfg, p.Components, false); err != nil {
				t.Errorf("profile %s: components %v: %v", name, p.Components, err)
			}
		}
	}
}

func TestResolveCompanionsWithoutProfilesKeepsFlags(t *testing.T) {
	sel, err := resolveCompanions(nil, []string{"canary"}, nil, []string{"telemetry"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sel.Docs) != 1 || sel.Docs[0] != "canary" || len(sel.Components) != 1 || len(sel.Tools) != 0 {
		t.Errorf("sel = %+v", sel)
	}
}

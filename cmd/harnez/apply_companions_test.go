package main

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strings"
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

// The apply wiring: profiles feed docs and components, and install runs the
// resolved tools through the env (issue 1190 review).
func TestCompanionPlanProfilesAndInstall(t *testing.T) {
	plan, err := resolveCompanions([]string{"go", "codeberg"}, []string{"canary"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.Docs, []string{"canary", "golang", "make", "agit"}) {
		t.Errorf("docs = %v", plan.Docs)
	}
	var ran []string
	env := companion.Env{
		LookPath: func(b string) (string, error) {
			if b == "go" {
				return "/bin/go", nil
			}
			return "", errors.New("missing")
		},
		Run: func(argv []string, _ io.Writer) error { ran = append(ran, strings.Join(argv, " ")); return nil },
	}
	var out, warn bytes.Buffer
	if n := plan.install(env, &out, &warn); n != 1 {
		t.Errorf("warnings = %d, want 1 (cargo missing for fj); warn %q", n, warn.String())
	}
	if !slices.Equal(ran, []string{"go install ubunatic.com/gokay@latest"}) {
		t.Errorf("ran = %v", ran)
	}
	if !strings.Contains(warn.String(), "apply continued") {
		t.Errorf("warn lacks summary: %q", warn.String())
	}
}

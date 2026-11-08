package main

import (
	"fmt"
	"io"

	"ubunatic.com/harnez/internal/companion"
)

// companionPlan is the resolved --profiles/--tools selection plus the spec it
// came from, so apply loads spec/tools.yaml once.
type companionPlan struct {
	companion.Selection
	spec *companion.Spec
}

// resolveCompanions merges --profiles with --docs, --tools and --components
// (issue 1190). With no profiles and no tools it does not load the spec.
func resolveCompanions(profiles, docs, tools, components []string) (companionPlan, error) {
	if len(profiles) == 0 && len(tools) == 0 {
		return companionPlan{Selection: companion.Selection{Docs: docs, Components: components}}, nil
	}
	spec, err := companion.LoadSpec()
	if err != nil {
		return companionPlan{}, err
	}
	sel, err := spec.Resolve(profiles, docs, tools, components)
	if err != nil {
		return companionPlan{}, err
	}
	return companionPlan{Selection: sel, spec: &spec}, nil
}

// install installs the plan's tools after the rest of apply; failures warn.
func (p companionPlan) install(env companion.Env, out, warn io.Writer) int {
	if len(p.Tools) == 0 || p.spec == nil {
		return 0
	}
	fmt.Fprintln(out, "Installing companion tools")
	n := p.spec.Install(p.Tools, env, out, warn)
	if n > 0 {
		fmt.Fprintf(warn, "companion tools: %d warning(s); apply continued\n", n)
	}
	return n
}

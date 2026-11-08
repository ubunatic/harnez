package main

import (
	"fmt"
	"io"

	"ubunatic.com/harnez/internal/companion"
)

// resolveCompanions merges --profiles with --docs, --tools and --components
// (issue 1190). With no profiles and no tools it does not load the spec.
func resolveCompanions(profiles, docs, tools, components []string) (companion.Selection, error) {
	if len(profiles) == 0 && len(tools) == 0 {
		return companion.Selection{Docs: docs, Components: components}, nil
	}
	spec, err := companion.LoadSpec()
	if err != nil {
		return companion.Selection{}, err
	}
	return spec.Resolve(profiles, docs, tools, components)
}

// installCompanionTools installs tools after the rest of apply; failures warn.
func installCompanionTools(tools []string, upgrade bool, out, warn io.Writer) {
	if len(tools) == 0 {
		return
	}
	spec, err := companion.LoadSpec()
	if err != nil {
		fmt.Fprintf(warn, "warning: companion tools: %v\n", err)
		return
	}
	env := companion.DefaultEnv()
	env.Upgrade = upgrade
	fmt.Fprintln(out, "Installing companion tools")
	if n := spec.Install(tools, env, out, warn); n > 0 {
		fmt.Fprintf(warn, "companion tools: %d warning(s); apply continued\n", n)
	}
}

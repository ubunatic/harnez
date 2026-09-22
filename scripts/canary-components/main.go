// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

// canary-components prints what `harnez apply` would install, remove or skip
// under a component selection (issue 490, docs/HarnezComponents.md §8). It is
// read-only: it plans against the config and never touches the filesystem.
//
//	go run ./scripts/canary-components docs-only
//	go run ./scripts/canary-components -c config.yaml telemetry-only,usage
package main

import (
	"flag"
	"fmt"
	"os"

	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/components"
)

func main() {
	configPath := flag.String("c", "", "config YAML (default: embedded)")
	flag.Parse()

	cfg, name, err := claude.OpenConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config:", err)
		os.Exit(1)
	}
	set, err := components.Resolve(flag.Args(), nil, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Printf("config: %s\ncomponents: %s\n\n", name, set)
	for _, s := range components.Plan(cfg, set) {
		comp := string(s.Component)
		if comp == "" {
			comp = "-"
		}
		fmt.Printf("  %-8s %-10s %-46s %s\n", s.Action, comp, s.Phase, s.Detail)
	}
}

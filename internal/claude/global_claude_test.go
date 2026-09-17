// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"regexp"
	"testing"
)

// TestGlobalClaudeSectionsHaveNoEagerDocIncludes asserts that no section under
// agents_md.global.sections in config.yaml contains eager include directives
// matching (?:^|\s)@docs/\S+ (issue 386). Global instructions must remain
// lightweight glue and must not trigger eager inlining of whole doc files into
// every session globally.
func TestGlobalClaudeSectionsHaveNoEagerDocIncludes(t *testing.T) {
	cfg, err := LoadConfigEmbedded()
	if err != nil {
		t.Fatalf("LoadConfigEmbedded() failed: %v", err)
	}

	eagerDocRegex := regexp.MustCompile(`(?m)(?:^|\s)@docs/\S+`)

	if len(cfg.AgentsMD.Global.Sections) == 0 {
		t.Fatal("cfg.AgentsMD.Global.Sections is empty; expected global sections in embedded config.yaml")
	}

	for _, sec := range cfg.AgentsMD.Global.Sections {
		t.Run(sec.Name, func(t *testing.T) {
			if matches := eagerDocRegex.FindAllString(sec.Content, -1); len(matches) > 0 {
				t.Errorf("global section %q contains eager doc includes %v; remove '@' prefix so Claude Code does not inline full doc files globally (issue 386)", sec.Name, matches)
			}
		})
	}

	if matches := eagerDocRegex.FindAllString(cfg.AgentsMD.Global.Content, -1); len(matches) > 0 {
		t.Errorf("global template content contains eager doc includes %v", matches)
	}
}

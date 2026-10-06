package handoff

import (
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"
)

// RenderMarkdown renders the profiles and agents of spec/handoff.yaml in fsys
// as the agent-facts section of the /harnez-handoff skill.
func RenderMarkdown(fsys fs.FS) (string, error) {
	s, err := LoadSpecFS(fsys)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, profile := range slices.Sorted(maps.Keys(s.Profiles)) {
		p := s.Profiles[profile]
		fmt.Fprintf(&b, "### Profile `%s`\n\n%s\n\nRules for every `%s` prompt:\n\n", profile, p.Description, profile)
		for _, rule := range p.Rules {
			fmt.Fprintf(&b, "- %s\n", rule)
		}
		var agents []*Agent
		for _, a := range s.Agents {
			if a.Profile == profile {
				agents = append(agents, a)
			}
		}
		slices.SortFunc(agents, func(x, y *Agent) int { return strings.Compare(x.Key, y.Key) })
		if len(agents) > 0 {
			fmt.Fprintf(&b, "\nKnown `%s` agents:\n\n", profile)
		}
		for _, a := range agents {
			fmt.Fprintf(&b, "- **%s** (matches: %s)\n", a.Name, strings.Join(a.Aliases, ", "))
			for _, fact := range a.Facts {
				fmt.Fprintf(&b, "  - %s\n", fact)
			}
			if len(a.Hosts) > 0 {
				fmt.Fprintf(&b, "  - Reaches remotes on: %s\n", strings.Join(a.Hosts, ", "))
			}
			if a.Result != "" {
				fmt.Fprintf(&b, "  - Result: %s\n", a.Result)
			}
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

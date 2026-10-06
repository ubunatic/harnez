package claude

import (
	"fmt"
	"io/fs"
	"strings"

	"ubunatic.com/harnez/internal/handoff"
)

// Skill bodies may hold a line `<!-- harnez:render <name> -->`; `harnez apply`
// replaces it with the output of the named renderer, so spec facts land in
// the installed skill and no agent has to read a spec at runtime (issue 726).
const (
	renderMarkerPrefix = "<!-- harnez:render "
	renderMarkerSuffix = " -->"
)

var skillRenderers = map[string]func(fs.FS) (string, error){
	"handoff-agents": handoff.RenderMarkdown,
}

func renderSkillBody(skill, body string, fsys fs.FS) (string, error) {
	if !strings.Contains(body, renderMarkerPrefix) {
		return body, nil
	}
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, renderMarkerPrefix) || !strings.HasSuffix(trimmed, renderMarkerSuffix) {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(trimmed, renderMarkerPrefix), renderMarkerSuffix)
		render, ok := skillRenderers[name]
		if !ok {
			return "", fmt.Errorf("skill %s: unknown renderer %q", skill, name)
		}
		out, err := render(fsys)
		if err != nil {
			return "", fmt.Errorf("skill %s: render %s: %w", skill, name, err)
		}
		lines[i] = out
	}
	return strings.Join(lines, "\n"), nil
}

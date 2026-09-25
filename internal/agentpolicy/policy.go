// Package agentpolicy manages the repository-local subagent dispatch policy.
package agentpolicy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ubunatic.com/harnez/internal/fsutil"
	"ubunatic.com/harnez/internal/markdown"
	"ubunatic.com/harnez/internal/subagent"
)

const Section = "Subagent Policy"

const policyBody = `# subagent_mode: %s
- native: spawn subagents of your own vendor natively. For another provider's model, or when
  the user names a model (e.g. "ask luna:low"), run it through ` + "`harnez agent --model <spec> -p ...`" + `
  (see ` + "`harnez agent models`" + `).
- harnez: dispatch every subagent through ` + "`harnez agent`" + `.
- Preserve the repository's instructions and report the provider and session used.`

// State describes the effective policy and the files that contributed to it.
type State struct {
	Mode      string `json:"mode"`
	Source    string `json:"source,omitempty"`
	Conflict  bool   `json:"conflict,omitempty"`
	MainMode  string `json:"main_mode,omitempty"`
	LocalMode string `json:"local_mode,omitempty"`
}

// Configure writes the managed policy block, preserving every other section.
func Configure(dir, mode string, persist bool) (string, bool, error) {
	if mode != "harnez" && mode != "native" {
		return "", false, fmt.Errorf("unsupported subagent mode %q", mode)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false, err
	}
	if _, err := fsutil.EnsureGitExclude(abs, "AGENTS.local.md"); err != nil {
		return "", false, fmt.Errorf("ensure AGENTS.local.md exclusion: %w", err)
	}
	name := "AGENTS.local.md"
	if persist {
		name = "AGENTS.md"
	}
	path := filepath.Join(abs, name)
	changed := false
	if persist {
		localPath := filepath.Join(abs, "AGENTS.local.md")
		_, readErr := policyMode(localPath)
		if readErr != nil {
			return path, false, readErr
		}
		removed, cleaned, cleanErr := markdown.Clean(localPath, Section)
		if cleanErr != nil {
			return path, false, fmt.Errorf("clean local policy: %w", cleanErr)
		}
		changed = removed || cleaned
	}
	updated, _, err := markdown.Apply(path, Section, fmt.Sprintf(policyBody, mode))
	if err != nil {
		return path, false, fmt.Errorf("update %s: %w", path, err)
	}
	return path, changed || updated, nil
}

// Resolve reads local policy first. A conflicting main policy is reported rather
// than hidden, so init and status cannot silently suggest a different policy.
func Resolve(dir string) (State, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return State{}, err
	}
	local, err := policyMode(filepath.Join(abs, "AGENTS.local.md"))
	if err != nil {
		return State{}, err
	}
	main, err := policyMode(filepath.Join(abs, "AGENTS.md"))
	if err != nil {
		return State{}, err
	}
	state := State{Mode: "unset", MainMode: main, LocalMode: local}
	if local != "" {
		state.Mode, state.Source = local, "./AGENTS.local.md"
	} else if main != "" {
		state.Mode, state.Source = main, "./AGENTS.md"
	}
	state.Conflict = local != "" && main != "" && local != main
	return state, nil
}

func policyMode(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read policy %s: %w", path, err)
	}
	content := string(data)
	begin := "<!-- harnez:begin " + Section + " -->"
	end := "<!-- harnez:end " + Section + " -->"
	start := strings.Index(content, begin)
	if start < 0 {
		return "", nil
	}
	finish := strings.Index(content[start+len(begin):], end)
	if finish < 0 {
		return "", nil
	}
	block := content[start : start+len(begin)+finish]
	for _, mode := range []string{"harnez", "native"} {
		if strings.Contains(block, "subagent_mode: "+mode) {
			return mode, nil
		}
	}
	return "", nil
}

// RepoSessions returns active and recent sessions whose working directory is
// inside dir. Ordering is stable and newest activity is shown first.
func RepoSessions(dir string, sessions []*subagent.Session) []*subagent.Session {
	abs, _ := filepath.Abs(dir)
	filtered := make([]*subagent.Session, 0, len(sessions))
	for _, sess := range sessions {
		if sess == nil || sess.WorkingDir == "" {
			continue
		}
		work, err := filepath.Abs(sess.WorkingDir)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(abs, work)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			filtered = append(filtered, sess)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].LastActiveAt.Equal(filtered[j].LastActiveAt) {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].LastActiveAt.After(filtered[j].LastActiveAt)
	})
	return filtered
}

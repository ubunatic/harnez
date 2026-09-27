// Package agentpolicy manages the repository-local subagent dispatch policy.
package agentpolicy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/fsutil"
	"ubunatic.com/harnez/internal/markdown"
	"ubunatic.com/harnez/internal/subagent"
)

const Section = "Subagent Policy"

const policyBody = `# subagent_mode: %s
- A requested model such as ` + "`terra:low`" + ` or ` + "`luna`" + ` is a Harnez agent model (see ` + "`harnez agent models`" + `);
  dispatch it with ` + "`harnez agent start --model <name>`" + `, regardless of ` + "`subagent_mode`" + `.
- native: spawn subagents of your own vendor natively; this mode governs only your own subagent choice.
- harnez: dispatch every subagent through ` + "`harnez agent`" + `.
- When Harnez MCP tools are exposed, use ` + "`harnez_spawn_agent`" + ` for a structured result and
  lifecycle, or ` + "`harnez_command`" + ` when the host should run the returned command in Bash.
  Otherwise use ` + "`harnez agent --model <spec> --role advisor -p <prompt>`" + ` via Bash.
- Supported lifecycle tools include ` + "`harnez_wait_agent`" + `, ` + "`harnez_list_agents`" + `,
  ` + "`harnez_agent_status`" + `, ` + "`harnez_resume_agent`" + `, and ` + "`harnez_stop_agent`" + `.
  The CLI forms are ` + "`harnez agent list`" + `, ` + "`harnez agent status --name <session>`" + `,
  ` + "`harnez agent wait <session>`" + `, ` + "`harnez agent resume --name <session> <prompt>`" + `,
  and ` + "`harnez agent stop --name <session>`" + `.
  ` + "`wait <session>`" + ` takes its session positionally; resume requires ` + "`--name`" + ` because
  a positional name is treated as the prompt, and resume has no ` + "`--detach`" + ` flag.
- There is no ` + "`harnez advisor`" + ` command.
- Before broad shell searches, use ` + "`harnez find code|docs`" + ` or MCP ` + "`harnez_find`" + `; see ` + "`@docs/Search.md`" + `.
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
func Configure(dir, mode string, _ bool) (string, bool, error) {
	if mode != "harnez" && mode != "native" {
		return "", false, fmt.Errorf("unsupported subagent mode %q", mode)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false, err
	}
	if _, err := fsutil.EnsureGitExclude(abs, filepath.Join(".harnez", "rules", "Local.md")); err != nil {
		return "", false, fmt.Errorf("ensure Local.md exclusion: %w", err)
	}
	migrated, err := claude.MigrateLegacyLocalRules(abs)
	if err != nil {
		return "", false, fmt.Errorf("migrate legacy local rules: %w", err)
	}
	path := filepath.Join(abs, ".harnez", "rules", "Local.md")
	updated, _, err := markdown.Apply(path, Section, fmt.Sprintf(policyBody, mode))
	if err != nil {
		return path, false, fmt.Errorf("update %s: %w", path, err)
	}
	return path, migrated || updated, nil
}

// Resolve reads policy from Local.md, migrating recognized legacy local sections first.
func Resolve(dir string) (State, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return State{}, err
	}
	if _, err := claude.MigrateLegacyLocalRules(abs); err != nil {
		return State{}, fmt.Errorf("migrate legacy local rules: %w", err)
	}
	local, err := policyMode(filepath.Join(abs, ".harnez", "rules", "Local.md"))
	if err != nil {
		return State{}, err
	}
	state := State{Mode: "unset", LocalMode: local}
	if local != "" {
		state.Mode, state.Source = local, "./.harnez/rules/Local.md"
	}
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
	block := content
	if start >= 0 {
		finish := strings.Index(content[start+len(begin):], end)
		if finish < 0 {
			return "", nil
		}
		block = content[start : start+len(begin)+finish]
	}
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

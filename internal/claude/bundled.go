package claude

import (
	"fmt"
	"io/fs"
	"path"
	"slices"

	"ubunatic.com/harnez/internal/skillreg"
)

// BundledSkills resolves config.yaml bundled_skills against the config
// filesystem. A bundled name must not collide with a harnez-owned skill.
func BundledSkills(cfg *Config) ([]skillreg.Bundled, error) {
	var out []skillreg.Bundled
	seen := map[string]bool{}
	for _, src := range cfg.BundledSkills {
		for _, name := range src.Skills {
			if !skillreg.NameOK(name) {
				return nil, fmt.Errorf("bundled skill %q: not a safe directory name", name)
			}
			if seen[name] || slices.ContainsFunc(cfg.Skills, func(c Command) bool { return c.Name == name }) {
				return nil, fmt.Errorf("bundled skill %q: name already used by another skill; rename one of them", name)
			}
			seen[name] = true
			if cfg.FS == nil {
				return nil, fmt.Errorf("bundled skill %q: no config filesystem", name)
			}
			sub, err := fs.Sub(cfg.FS, path.Join(src.Dir, name))
			if err != nil {
				return nil, err
			}
			if _, err := fs.Stat(sub, "SKILL.md"); err != nil {
				return nil, fmt.Errorf("bundled skill %q: %w (run 'harnez skill vendor')", name, err)
			}
			out = append(out, skillreg.Bundled{Name: name, URL: src.URL, Commit: src.Commit, Auto: src.Auto, FS: sub})
		}
	}
	return out, nil
}

// syncBundledSkills installs bundled skills into every agent skill dir.
func syncBundledSkills(cfg *Config) (int, []string, error) {
	skills, err := BundledSkills(cfg)
	if err != nil {
		return 0, nil, err
	}
	var targets []skillreg.Target
	for _, t := range SkillTargetsByAgent(cfg) {
		targets = append(targets, skillreg.Target{Agent: t.Agent, Dir: t.Dir})
	}
	changed, notes, err := skillreg.SyncBundled(targets, skills)
	for _, c := range changed {
		fmt.Printf("  %s\n", c)
	}
	for _, n := range notes {
		fmt.Printf("  note: %s\n", n)
	}
	var names []string
	for _, s := range skills {
		names = append(names, s.Name)
	}
	return len(changed), names, err
}

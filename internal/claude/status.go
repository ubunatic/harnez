package claude

import (
	"fmt"
	"os"
	"path/filepath"

	"ubunatic.com/claudeconfig/internal/fsutil"
	"ubunatic.com/claudeconfig/internal/jsonc"
	"ubunatic.com/claudeconfig/internal/markdown"
)

func hasSettingsKey(path, key string) bool {
	m := jsonc.Read(path)
	_, ok := m[key]
	return ok
}

// RunStatus prints the current applied status of configuration parameters.
func RunStatus(configPath string, cfg *Config, target string) error {
	fmt.Println("Config:")
	fmt.Printf("  %-14s %s\n", "file:", configPath)
	fmt.Printf("  %-14s %s\n", "target:", target)
	if cfg.Model != "" {
		fmt.Printf("  %-14s %s → %s\n", "model:", cfg.Model, resolveModel(cfg.Model))
	}
	if cfg.Effort != "" {
		fmt.Printf("  %-14s %s\n", "effort:", cfg.Effort)
	}
	fmt.Printf("  %-14s %d allow, %d deny\n", "permissions:",
		len(cfg.Permissions.Allow), len(cfg.Permissions.Deny))
	fmt.Printf("  %-14s %d\n", "hooks:", len(cfg.Hooks))
	fmt.Printf("  %-14s %d\n", "mcp_servers:", len(cfg.MCPServers))
	fmt.Printf("  %-14s %d\n", "commands:", len(cfg.Commands))
	fmt.Printf("  %-14s %d\n", "skills:", len(cfg.Skills))
	fmt.Printf("  %-14s %d global, %d local\n", "agents_md:",
		len(cfg.AgentsMD.Global.Sections), len(cfg.AgentsMD.Local.Sections))

	fmt.Println()
	fmt.Println("Applied:")

	type entry struct {
		label string
		check func() bool
	}

	settingsPath := filepath.Join(target, "settings.json")
	checks := []entry{
		{
			label: "settings.json [model]",
			check: func() bool { return hasSettingsKey(settingsPath, "model") },
		},
	}
	for _, s := range cfg.AgentsMD.Global.Sections {
		s := s
		gTarget := fsutil.ExpandHome(cfg.AgentsMD.Global.Target)
		checks = append(checks, entry{
			label: gTarget + " [" + s.Name + "]",
			check: func() bool { return markdown.ContainsSection(gTarget, s.Name) },
		})
	}
	for _, s := range cfg.AgentsMD.Local.Sections {
		s := s
		lTarget := cfg.AgentsMD.Local.Target
		checks = append(checks, entry{
			label: lTarget + " [" + s.Name + "]",
			check: func() bool { return markdown.ContainsSection(lTarget, s.Name) },
		})
	}
	for _, cmd := range cfg.Commands {
		cmd := cmd
		path := filepath.Join(target, "commands", cmd.Name+".md")
		checks = append(checks, entry{
			label: path,
			check: func() bool { _, err := os.Stat(path); return err == nil },
		})
	}
	if len(cfg.Skills) > 0 {
		targets := skillTargets(cfg)
		for _, skill := range cfg.Skills {
			skill := skill
			for _, skillsRoot := range targets {
				path := filepath.Join(skillsRoot, skill.Name, "SKILL.md")
				checks = append(checks, entry{
					label: path,
					check: func() bool { _, err := os.Stat(path); return err == nil },
				})
			}
		}
	}

	for _, e := range checks {
		state := "missing"
		if e.check() {
			state = "ok"
		}
		fmt.Printf("  %-48s %s\n", e.label, state)
	}

	if len(cfg.AgentsMD.Languages) > 0 {
		fmt.Println()
		fmt.Println("Docs:")
		for _, name := range cfg.Docs {
			lang, ok := cfg.AgentsMD.Languages[name]
			if !ok {
				continue
			}
			dst := fsutil.ExpandHome(lang.Target)
			state := langDocState(cfg.FS, lang.Source, dst)
			fmt.Printf("  %-14s %s [%s]\n", name+":", fsutil.ContractHome(dst), state)
		}
	}

	return nil
}

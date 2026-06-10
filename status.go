package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func hasSettingsKey(path, key string) bool {
	m := readJSONC(path)
	_, ok := m[key]
	return ok
}

func hasSectionMD(path, section string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "<!-- claudeconfig:begin "+section+" -->")
}

func runStatus(configPath string, cfg *Config, target string) error {
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
		gTarget := expandHome(cfg.AgentsMD.Global.Target)
		checks = append(checks, entry{
			label: gTarget + " [" + s.Name + "]",
			check: func() bool { return hasSectionMD(gTarget, s.Name) },
		})
	}
	for _, s := range cfg.AgentsMD.Local.Sections {
		s := s
		lTarget := cfg.AgentsMD.Local.Target
		checks = append(checks, entry{
			label: lTarget + " [" + s.Name + "]",
			check: func() bool { return hasSectionMD(lTarget, s.Name) },
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

	for _, e := range checks {
		state := "missing"
		if e.check() {
			state = "ok"
		}
		fmt.Printf("  %-48s %s\n", e.label, state)
	}

	if len(cfg.AgentsMD.Languages) > 0 {
		fmt.Println()
		fmt.Println("Language docs:")
		for _, name := range cfg.Langs {
			lang, ok := cfg.AgentsMD.Languages[name]
			if !ok {
				continue
			}
			dst := expandHome(lang.Target)
			state := langDocState(cfg.FS, lang.Source, dst)
			fmt.Printf("  %-14s %s [%s]\n", name+":", contractHome(dst), state)
		}
	}

	return nil
}

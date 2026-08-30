package claude

import (
	"fmt"
	"os"
	"path/filepath"

	"ubunatic.com/harnez/internal/fsutil"
	"ubunatic.com/harnez/internal/issues"
	"ubunatic.com/harnez/internal/jsonc"
	"ubunatic.com/harnez/internal/markdown"
	"ubunatic.com/harnez/internal/usage"
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
	if root := primeAgentRoot(cfg); root != "" {
		fmt.Printf("  %-14s %s\n", "prime_agent:", root)
	}
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
	fmt.Printf("  %-14s %d\n", "distill:", len(distillAdapters(cfg)))
	fmt.Printf("  %-14s %d global, %d local\n", "agents_md:",
		len(cfg.AgentsMD.Global.Sections), len(cfg.AgentsMD.Local.Sections))

	localCfg, localCfgPath, localCfgErr := usage.LoadLocalConfig("")
	localCfgState := "absent"
	switch {
	case localCfgErr != nil:
		localCfgState = fmt.Sprintf("error: %v", localCfgErr)
	case localCfg != nil:
		localCfgState = "present"
	}
	fmt.Printf("  %-14s %s [%s]\n", "local config:", fsutil.ContractHome(localCfgPath), localCfgState)

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
	ruleTargets := []string{fsutil.ExpandHome(cfg.AgentsMD.Global.Target)}
	if root := primeAgentRoot(cfg); root != "" {
		ruleTargets = appendUniquePath(ruleTargets, filepath.Join(root, "AGENTS.md"))
	}
	for _, ruleTarget := range ruleTargets {
		ruleTarget := ruleTarget
		for _, s := range cfg.AgentsMD.Global.Sections {
			s := s
			checks = append(checks, entry{
				label: ruleTarget + " [" + s.Name + "]",
				check: func() bool { return markdown.ContainsSection(ruleTarget, s.Name) },
			})
		}
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
		for _, cmdDir := range commandTargets(target, cfg) {
			path := filepath.Join(cmdDir, cmd.Name+".md")
			checks = append(checks, entry{
				label: path,
				check: func() bool { _, err := os.Stat(path); return err == nil },
			})
		}
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
	for _, adapter := range distillAdapters(cfg) {
		adapter := adapter
		checks = append(checks, entry{
			label: adapter.path,
			check: func() bool { _, err := os.Stat(adapter.path); return err == nil },
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
		fmt.Println("Docs:")
		for _, name := range cfg.Docs {
			lang, ok := cfg.AgentsMD.Languages[name]
			if !ok {
				continue
			}
			docTargets := []string{fsutil.ExpandHome(lang.Target)}
			if root := primeAgentRoot(cfg); root != "" {
				docTargets = appendUniquePath(docTargets, filepath.Join(root, "docs", filepath.Base(lang.Target)))
			}
			for _, dst := range docTargets {
				state := langDocState(cfg.FS, lang.Source, dst)
				fmt.Printf("  %-14s %s [%s]\n", name+":", fsutil.ContractHome(dst), state)
			}
		}
	}

	issuesTrackerPath := "issues/README.md"
	if _, err := os.Stat(issuesTrackerPath); err == nil {
		fmt.Println()
		fmt.Println("Issues:")
		report, err := issues.Lint("issues")
		if err != nil {
			fmt.Printf("  %-14s %s [error: %v]\n", "tracker:", issuesTrackerPath, err)
		} else {
			driftCount := len(report.Diagnostics)
			okCount := report.TotalFiles - driftCount
			if okCount < 0 {
				okCount = 0
			}
			if driftCount == 0 {
				fmt.Printf("  %-14s %s [%d tickets: all ok]\n", "tracker:", issuesTrackerPath, report.TotalFiles)
			} else {
				fmt.Printf("  %-14s %s [%d tickets: %d ok, %d drift]\n", "tracker:", issuesTrackerPath, report.TotalFiles, okCount, driftCount)
				for _, d := range report.Diagnostics {
					fmt.Printf("    ! %s\n", d.Message)
				}
			}
		}
	}

	return nil
}

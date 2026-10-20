package claude

import (
	"fmt"
	"os"
)

// initQuotaRules reconciles the opt-in quota section and test-q1 Make target.
func initQuotaRules(ir *initReport, cfg *Config, dir, agentsPath string, assumeYes bool) error {
	if cfg == nil || cfg.AgentsMD.Rules.QuotaSection == "" {
		return fmt.Errorf("quota-1 rules are not configured")
	}
	r, err := applySectionMD(agentsPath, quota1SectionName, cfg.AgentsMD.Rules.QuotaSection)
	if err != nil {
		return fmt.Errorf("agents_md [%s]: %w", quota1SectionName, err)
	}
	if r.changed {
		ir.changes++
	}
	printResult("wrote", agentsPath, r)

	makePath := localPath(dir, "Makefile")
	targetSnippet := "test-q1: 🤖  # run tests under Quota-1 enforcement\n\tharnez exec --quota-1 -- $(MAKE) test\n"
	if _, err := os.Stat(makePath); os.IsNotExist(err) {
		initialMake := fmt.Sprintf(".PHONY: ⚙️ 🤖\n⚙️:\n🤖:\n\n%s", targetSnippet)
		if err := os.WriteFile(makePath, []byte(initialMake), 0644); err != nil {
			return fmt.Errorf("scaffold %s: %w", makePath, err)
		}
		ir.changes++
		fmt.Printf("  scaffolded %s (test-q1)\n", makePath)
	} else if err == nil {
		var makeCfg MakeConfig
		if cfg != nil {
			makeCfg = cfg.Make
		}
		tr, err := ReconcileMakeTargets(makePath, targetSnippet, makeCfg, assumeYes, nil)
		if err != nil {
			return fmt.Errorf("reconcile test-q1 in %s: %w", makePath, err)
		}
		if tr {
			ir.changes++
			fmt.Printf("  reconciled %s (test-q1)\n", makePath)
		} else {
			fmt.Printf("  exists  %s (test-q1 unchanged)\n", makePath)
		}
	}
	return nil
}

// Package guard provides high-speed rule enforcement and tool safety guardrails
// backed by decision models (harnez decide / Jev). See issue 634.
package guard

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez/internal/decide"
)

// DefaultThreshold is the default confidence threshold for blocking rule violations.
const DefaultThreshold = 0.85

// Violation represents the outcome of a guard check.
type Violation struct {
	Blocked    bool    `json:"blocked"`
	Rule       string  `json:"rule,omitempty"`
	Risk       string  `json:"risk,omitempty"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason,omitempty"`
}

// CollectRules reads rules from .harnez/rules/*.md and AGENTS.md in repoRoot.
func CollectRules(repoRoot string) (map[string]string, error) {
	rules := make(map[string]string)

	if repoRoot == "" {
		repoRoot = "."
	}

	// 1. Read AGENTS.md if present
	agentsPath := filepath.Join(repoRoot, "AGENTS.md")
	if data, err := os.ReadFile(agentsPath); err == nil && len(data) > 0 {
		rules["AGENTS.md"] = string(data)
	}

	// 2. Read .harnez/rules/*.md
	rulesDir := filepath.Join(repoRoot, ".harnez", "rules")
	if entries, err := os.ReadDir(rulesDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			p := filepath.Join(rulesDir, entry.Name())
			if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
				rules[entry.Name()] = string(data)
			}
		}
	}

	return rules, nil
}

// CheckFileEdit verifies if a proposed file edit violates repository rules.
func CheckFileEdit(ctx context.Context, backend decide.Backend, repoRoot, filePath, proposedContent string, threshold float64) (*Violation, error) {
	if threshold <= 0 {
		threshold = DefaultThreshold
	}

	rules, err := CollectRules(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("collect rules: %w", err)
	}

	ruleChoices := make(map[string]string)
	ruleChoices["none"] = "No repository rule is violated; standard valid code change"
	for name, content := range rules {
		var summary string
		runes := []rune(content)
		if len(runes) > 200 {
			summary = string(runes[:200]) + "..."
		} else {
			summary = content
		}
		summary = strings.ReplaceAll(summary, "\n", " ")
		ruleChoices[name] = summary
	}

	req := &decide.Request{
		State: map[string]any{
			"target_file":      filePath,
			"proposed_content": proposedContent,
			"rules":            rules,
		},
		Questions: map[string]decide.Question{
			"rule_violation": {
				Type:         decide.TypeNoul,
				Instructions: fmt.Sprintf("Does this proposed modification to file %q violate any repository rules, bypass safeguards, or modify forbidden/generated files?", filePath),
				Criteria: map[string]string{
					"true":  "Modifies generated files directly, edits symlink instead of source canonical, breaks architectural layering, bypasses canary/test rules",
					"false": "Standard compliant code change",
				},
			},
			"violated_rule": {
				Type:         decide.TypeChoice,
				Instructions: "Which repository rule is violated by this change?",
				Criteria:     ruleChoices,
			},
		},
	}

	res, err := backend.Decide(ctx, req)
	if err != nil {
		return nil, err
	}

	v := &Violation{}
	if noulAns, ok := res.Answers["rule_violation"]; ok && noulAns.Noul != nil {
		v.Confidence = *noulAns.Noul
		if *noulAns.Noul >= threshold {
			v.Blocked = true
		}
	}

	if choiceAns, ok := res.Answers["violated_rule"]; ok && choiceAns.Choice != "" && choiceAns.Choice != "none" {
		v.Rule = choiceAns.Choice
	}

	if v.Blocked {
		v.Reason = fmt.Sprintf("Edit blocked: Jev rule enforcer detected violation of rule [%s] (confidence %.2f)", v.Rule, v.Confidence)
	}

	return v, nil
}

// CheckCommand evaluates the risk of running a terminal command.
func CheckCommand(ctx context.Context, backend decide.Backend, cwd, command string, threshold float64) (*Violation, error) {
	if threshold <= 0 {
		threshold = DefaultThreshold
	}

	req := &decide.Request{
		State: map[string]any{
			"command": command,
			"cwd":     cwd,
		},
		Questions: map[string]decide.Question{
			"action_risk": {
				Type:         decide.TypeChoice,
				Instructions: "Classify the operational risk of running this command.",
				Criteria: map[string]string{
					"read_only":   "Commands that only inspect state without mutating disk or git history (ls, git status, cat, grep)",
					"reversible":  "Standard developer commands that create/modify files or build/test in workspace (touch, go test, npm build, git add)",
					"destructive": "Commands that delete untracked data, destroy git history, force push, bypass quota guardrails, or alter system configurations (rm -rf, git push --force, git reset --hard, HARNEZ_QUOTA_BYPASS=1)",
				},
			},
			"destructive": {
				Type:         decide.TypeNoul,
				Instructions: "Is this command destructive, irreversible, or an unauthorized guardrail bypass?",
				Criteria: map[string]string{
					"true":  "Destructive, irreversible, or bypasses guardrails",
					"false": "Safe or normal standard developer command",
				},
			},
		},
	}

	res, err := backend.Decide(ctx, req)
	if err != nil {
		return nil, err
	}

	v := &Violation{}
	if choiceAns, ok := res.Answers["action_risk"]; ok && choiceAns.Choice != "" {
		v.Risk = choiceAns.Choice
	}

	if noulAns, ok := res.Answers["destructive"]; ok && noulAns.Noul != nil {
		v.Confidence = *noulAns.Noul
		if *noulAns.Noul >= threshold || (v.Risk == "destructive" && *noulAns.Noul >= 0.50) {
			v.Blocked = true
		}
	}

	if v.Blocked {
		v.Reason = fmt.Sprintf("Command blocked: Jev guardrail detected destructive action [%s] (confidence %.2f)", command, v.Confidence)
	}

	return v, nil
}

// Enabled reports whether decide-based guardrails are active.
func Enabled(explicit *bool) bool {
	if explicit != nil {
		return *explicit
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("HARNEZ_DECIDE_GUARD"))) {
	case "0", "false", "off", "no":
		return false
	case "1", "true", "on", "yes":
		return true
	}

	if home, err := os.UserHomeDir(); err == nil {
		data, err := os.ReadFile(filepath.Join(home, ".harnez", "config.yaml"))
		if err == nil {
			var cfg struct {
				Hooks struct {
					Decide struct {
						Guard *bool `yaml:"guard"`
					} `yaml:"decide"`
				} `yaml:"hooks"`
			}
			if yaml.Unmarshal(data, &cfg) == nil && cfg.Hooks.Decide.Guard != nil {
				return *cfg.Hooks.Decide.Guard
			}
		}
	}
	return false
}

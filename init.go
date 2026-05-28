package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const agentsMDTemplate = `Adhere to the following conventions.

<!-- claudeconfig:begin Project Summary -->
<!-- claudeconfig:end Project Summary -->

## Development Scripts

Run from project root.

`

const summarySection = "Project Summary"

const summaryPrompt = `Summarize this project for a coding agent in plain markdown.
Cover: what it does, the main components and their roles, key conventions, and anything
important to know before making changes. No preamble, no trailing commentary — just the summary.`

const reviewPrompt = `Review the following project summary for an AI coding agent (AGENTS.md).
Produce a refined version that is:
- Precise and actionable — an agent can act on it directly
- Grounded in the code — no assumptions beyond what is shown
- Concise — no redundancy, no padding
- Well-structured — key conventions and entry points front and centre

Output only the refined summary in plain markdown. No preamble, no trailing commentary.`

func claudeP(dir, prompt string) (string, error) {
	cmd := exec.Command("claude", "-p", prompt)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude -p: %w", err)
	}
	return string(out), nil
}

func fetchSummary(dir string) (string, error) {
	return claudeP(dir, summaryPrompt)
}

func reviewSummary(dir, agentsPath, draft string) (string, error) {
	diffCmd := exec.Command("git", "diff", "--", agentsPath)
	diffCmd.Dir = dir
	diffOut, _ := diffCmd.Output()

	var context string
	if strings.TrimSpace(string(diffOut)) != "" {
		context = "```diff\n" + string(diffOut) + "\n```"
	} else {
		context = "```markdown\n" + draft + "\n```"
	}
	return claudeP(dir, reviewPrompt+"\n\n"+context)
}

func runInit(dir string, withSummary, update, replace bool) error {
	agentsPath := filepath.Join(dir, "AGENTS.md")
	claudePath := filepath.Join(dir, "CLAUDE.md")

	if update {
		withSummary = true
	}

	changes := 0

	if replace {
		if err := os.Remove(agentsPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", agentsPath, err)
		}
	}

	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		if err := os.WriteFile(agentsPath, []byte(agentsMDTemplate), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", agentsPath, err)
		}
		fmt.Printf("  created %s\n", agentsPath)
		changes++
	} else {
		fmt.Printf("  exists  %s (unchanged)\n", agentsPath)
	}

	r, err := ensureSymlink(claudePath, agentsPath)
	if err != nil {
		return fmt.Errorf("symlink %s: %w", claudePath, err)
	}
	if r.changed {
		fmt.Printf("  symlink %s → %s\n", claudePath, agentsPath)
		changes++
	} else {
		fmt.Printf("  exists  %s (unchanged)\n", claudePath)
	}

	if withSummary {
		fmt.Println("  running claude -p to summarise project...")
		draft, err := fetchSummary(dir)
		if err != nil {
			return err
		}
		if _, err := applySectionMD(agentsPath, summarySection, draft); err != nil {
			return fmt.Errorf("summary section (draft): %w", err)
		}

		fmt.Println("  running claude -p to review and refine summary...")
		refined, err := reviewSummary(dir, agentsPath, draft)
		if err != nil {
			return err
		}
		r, err := applySectionMD(agentsPath, summarySection, refined)
		if err != nil {
			return fmt.Errorf("summary section (refined): %w", err)
		}
		if r.changed {
			fmt.Printf("  wrote   %s [%s]\n", agentsPath, summarySection)
			changes++
		} else {
			fmt.Printf("  exists  %s [%s] (unchanged)\n", agentsPath, summarySection)
		}
	}

	if changes == 0 {
		fmt.Println("No changes.")
	} else {
		fmt.Printf("%d change(s).\n", changes)
	}
	return nil
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const agentsMDTemplate = `Adhere to the following conventions.

## Development Scripts

Run from project root.

`

const summarySection = "Project Summary"

const summaryPrompt = `Summarize this project for a coding agent in plain markdown.
Cover: what it does, the main components and their roles, key conventions, and anything
important to know before making changes. No preamble, no trailing commentary — just the summary.`

func fetchSummary(dir string) (string, error) {
	cmd := exec.Command("claude", "-p", summaryPrompt)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude -p: %w", err)
	}
	return string(out), nil
}

func runInit(dir string, withSummary bool) error {
	agentsPath := filepath.Join(dir, "AGENTS.md")
	claudePath := filepath.Join(dir, "CLAUDE.md")

	changes := 0

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
		summary, err := fetchSummary(dir)
		if err != nil {
			return err
		}
		r, err := applySectionMD(agentsPath, summarySection, summary)
		if err != nil {
			return fmt.Errorf("summary section: %w", err)
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

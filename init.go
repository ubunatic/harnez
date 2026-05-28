package main

import (
	"encoding/json"
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
important to know before making changes.
Start your response with the first line of content. No preamble, no trailing commentary.`

const reviewPrompt = `Review the following project summary for an AI coding agent (AGENTS.md).
Produce a refined version that is:
- Precise and actionable — an agent can act on it directly
- Grounded in the code — no assumptions beyond what is shown
- Concise — no redundancy, no padding
- Well-structured — key conventions and entry points front and centre

Start your response with the first line of the summary. No preamble, no analysis, no trailing commentary.
Do not include HTML comment markers (<!-- ... -->) in your output.`

// sanitizeContent strips artifacts that LLMs sometimes inject: outer code fences
// wrapping the entire response, and claudeconfig section markers (which corrupt the
// file structure if embedded in content).
func sanitizeContent(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if nl := strings.Index(s, "\n"); nl > 0 {
			if end := strings.LastIndex(s, "\n```"); end > nl {
				s = strings.TrimSpace(s[nl+1 : end])
			}
		}
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "<!-- claudeconfig:") {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// stripMetaCommentary removes meta-commentary separated from the actual content by
// a "---" rule. Handles two patterns:
//   - preamble before "---", content after  → keep after
//   - content before "---", appendix after  → keep before
//
// Only acts when one side is substantially longer than the other (≥4 lines vs <4).
func stripMetaCommentary(s string) string {
	const sep = "\n---\n"
	idx := strings.LastIndex(s, sep)
	if idx < 0 {
		return s
	}
	before := strings.TrimSpace(s[:idx])
	after := strings.TrimSpace(s[idx+len(sep):])
	bl := strings.Count(before, "\n") + 1
	al := strings.Count(after, "\n") + 1
	switch {
	case al >= 4 && bl < 4:
		return after
	case bl >= 4 && al < 4:
		return before
	}
	return s
}

type claudeJSONResult struct {
	Result       string  `json:"result"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

func claudeP(dir, prompt string) (string, error) {
	cmd := exec.Command("claude", "-p", "--output-format", "json", prompt)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude -p: %w", err)
	}
	var res claudeJSONResult
	if err := json.Unmarshal(out, &res); err != nil {
		return string(out), nil
	}
	u := res.Usage
	parts := fmt.Sprintf("in=%d out=%d", u.InputTokens, u.OutputTokens)
	if u.CacheReadInputTokens > 0 {
		parts += fmt.Sprintf(" read=%d", u.CacheReadInputTokens)
	}
	if u.CacheCreationInputTokens > 0 {
		parts += fmt.Sprintf(" write=%d", u.CacheCreationInputTokens)
	}
	fmt.Printf("  tokens  %s  $%.4f\n", parts, res.TotalCostUSD)
	return res.Result, nil
}

func fetchSummary(dir string) (string, error) {
	return claudeP(dir, summaryPrompt)
}

func reviewSummary(dir, draft string) (string, error) {
	context := "```markdown\n" + draft + "\n```"
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
		draft = sanitizeContent(stripMetaCommentary(draft))
		if _, err := applySectionMD(agentsPath, summarySection, draft); err != nil {
			return fmt.Errorf("summary section (draft): %w", err)
		}

		fmt.Println("  running claude -p to review and refine summary...")
		refined, err := reviewSummary(dir, draft)
		if err != nil {
			return err
		}
		refined = sanitizeContent(stripMetaCommentary(refined))
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

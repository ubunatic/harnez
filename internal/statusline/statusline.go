// Package statusline implements Harnez status-line renderers for Claude Code
// and Antigravity CLI.
//
// MVP scope: current working directory and context usage. Claude Code renders a custom
// statusLine on its own row above the built-in footer badges — it cannot
// share a line with the "esc to interrupt" / "? for shortcuts" hint row;
// that is a fixed constraint of the current tool, not a limitation of this
// package. See docs/other/... (issue 095) for the decision record.
package statusline

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// input is the subset of Claude Code's and Antigravity CLI's statusLine JSON payload this
// package reads. Other fields (cost, git, ...) are deliberately not modeled.
type input struct {
	CWD   string `json:"cwd"`
	Model struct {
		ID string `json:"id"`
	} `json:"model"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
		ProjectDir string `json:"project_dir"`
	} `json:"workspace"`
	ContextWindow struct {
		CurrentUsage      contextUsage `json:"current_usage"`
		ContextWindowSize int          `json:"context_window_size"`
	} `json:"context_window"`
	RateLimits struct {
		FiveHour   *rateLimit `json:"five_hour"`
		SevenDay   *rateLimit `json:"seven_day"`
		SpendLimit *rateLimit `json:"spend_limit"`
	} `json:"rate_limits"`
	Quota map[string]quotaLimit `json:"quota"`
}

type rateLimit struct {
	UsedPercentage *float64 `json:"used_percentage"`
}

type quotaLimit struct {
	RemainingFraction *float64 `json:"remaining_fraction"`
}

// Render reads a Claude Code statusLine JSON payload from r and returns the
// line to print. workspace.current_dir is authoritative, with top-level cwd
// as a fallback. If the effective directory differs from workspace.project_dir,
// both are shown as project_dir → current_dir. Paths are tilde-collapsed
// relative to home when possible.
func Render(r io.Reader, home string) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("statusline: read stdin: %w", err)
	}

	var in input
	if err := json.Unmarshal(data, &in); err != nil {
		return "", fmt.Errorf("statusline: parse stdin JSON: %w", err)
	}

	dir := effectiveDirectory(in, home)
	return renderTemplate("claude", lineData{
		Directory:  dir,
		Context:    contextText(in.ContextWindow.CurrentUsage, in.ContextWindow.ContextWindowSize),
		RateLimits: rateLimitText(in),
	})
}

// RenderContextUsage reads a Claude Code or Antigravity CLI status-line payload
// and returns compact current context usage, or an empty string when unavailable.
func RenderContextUsage(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("statusline: read stdin: %w", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", nil
	}
	var in input
	if err := json.Unmarshal(data, &in); err != nil {
		return "", fmt.Errorf("statusline: parse stdin JSON: %w", err)
	}
	home, _ := os.UserHomeDir()
	line, err := renderTemplate("agy", lineData{
		Directory: effectiveDirectory(in, home),
		Context:   contextText(in.ContextWindow.CurrentUsage, in.ContextWindow.ContextWindowSize),
		Quotas:    quotaText(in),
	})
	if err != nil || line == "" {
		return line, err
	}
	return "\x1b[2m" + line + "\x1b[0m", nil
}

func effectiveDirectory(in input, home string) string {
	dir := in.Workspace.CurrentDir
	if dir == "" {
		dir = in.CWD
	}
	dir = collapseHome(dir, home)
	projectDir := collapseHome(in.Workspace.ProjectDir, home)
	if dir != "" && projectDir != "" && dir != projectDir {
		return projectDir + " → " + dir
	}
	return dir
}

type contextUsage struct {
	InputTokens              int `json:"input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

func contextText(usage contextUsage, windowSize int) contextData {
	total := usage.InputTokens + usage.CacheCreationInputTokens + usage.CacheReadInputTokens
	context := contextData{}
	if windowSize > 0 {
		context.WindowAvailable = true
		context.WindowSize = formatTokens(windowSize)
	}
	if total > 0 {
		context.Available = true
		context.TotalTokens = total
		context.Size = formatTokens(total)
		context.CacheRead = usage.CacheReadInputTokens
		context.CachePercent = usage.CacheReadInputTokens * 100 / total
		context.HasCache = usage.CacheReadInputTokens > 0
	}
	return context
}

func formatTokens(tokens int) string {
	return fmt.Sprintf("%dk", (tokens+500)/1000)
}

func rateLimitText(in input) []rateLimitData {
	var limits []rateLimitData
	appendRateLimit := func(label string, limit *rateLimit) {
		if limit != nil && limit.UsedPercentage != nil {
			remainingPercent := 100 - *limit.UsedPercentage
			remaining := remainingPercent / 100
			limits = append(limits, rateLimitData{
				Label:            label,
				RemainingPercent: formatDecimal(remainingPercent, 1),
				Remaining:        formatDecimal(remaining, 4),
			})
		}
	}
	appendRateLimit("5h", in.RateLimits.FiveHour)
	appendRateLimit("7d", in.RateLimits.SevenDay)
	appendRateLimit("spend", in.RateLimits.SpendLimit)
	return limits
}

func quotaText(in input) []quotaData {
	modelClass := activeModelClass(in.Model.ID)
	if modelClass == "" {
		return nil
	}
	quotaNames := make([]string, 0, len(in.Quota))
	for name := range in.Quota {
		if strings.HasPrefix(strings.ToLower(name), modelClass+"-") {
			quotaNames = append(quotaNames, name)
		}
	}
	sort.Strings(quotaNames)
	quotas := make([]quotaData, 0, len(quotaNames))
	for _, name := range quotaNames {
		remaining := in.Quota[name].RemainingFraction
		if remaining != nil {
			quotas = append(quotas, quotaData{
				Name:              name[len(modelClass)+1:],
				RemainingPercent:  formatDecimal(*remaining*100, 1),
				RemainingFraction: formatDecimal(*remaining, 4),
			})
		}
	}
	return quotas
}

func activeModelClass(modelID string) string {
	modelID = strings.ToLower(modelID)
	if strings.Contains(modelID, "gemini") {
		return "gemini"
	}
	if strings.Contains(modelID, "claude") || strings.Contains(modelID, "gpt") {
		return "3p"
	}
	return ""
}

func formatDecimal(value float64, precision int) string {
	formatted := strconv.FormatFloat(value, 'f', precision, 64)
	return strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
}

// collapseHome replaces a leading home-directory prefix with "~".
func collapseHome(dir, home string) string {
	if dir == "" {
		return ""
	}
	if home == "" {
		return dir
	}
	if dir == home {
		return "~"
	}
	if rest, ok := strings.CutPrefix(dir, home+string(os.PathSeparator)); ok {
		return "~" + string(os.PathSeparator) + rest
	}
	return dir
}

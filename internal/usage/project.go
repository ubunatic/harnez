package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ubunatic.com/harnez/internal/assess"
)

// AgentProjectAttribution holds the token totals and model breakdown attributed to a repository for a single agent.
type AgentProjectAttribution struct {
	AgentID         string           `json:"agent_id"` // "claude", "agy", "codex"
	Name            string           `json:"name"`     // "Claude Code", "Antigravity", "OpenAI Codex"
	Tokens          TokenBreakdown   `json:"tokens"`
	ModelTokens     map[string]int64 `json:"model_tokens,omitempty"`
	MatchedSessions int              `json:"matched_sessions"`
}

// CostOfChangeMetrics holds efficiency and generative ROI derivations.
type CostOfChangeMetrics struct {
	NetCodeLOC          int     `json:"net_code_loc"`
	TokensPerLOC        float64 `json:"tokens_per_loc"`
	ClosedTickets       int     `json:"closed_tickets"`
	TokensPerTicket     float64 `json:"tokens_per_ticket"`
	CacheReadEfficiency float64 `json:"cache_read_efficiency"` // Pct of total input that came from cache
}

// ProjectUsageResult is the complete telemetry summary attributed to a project/repository path.
type ProjectUsageResult struct {
	RepoDir        string                              `json:"repo_dir"`
	RepoName       string                              `json:"repo_name"`
	LifetimeTokens int64                               `json:"lifetime_tokens"`
	TotalTokens    TokenBreakdown                      `json:"total_tokens"`
	Agents         map[string]*AgentProjectAttribution `json:"agents"`
	CostOfChange   *CostOfChangeMetrics                `json:"cost_of_change,omitempty"`
}

// CollectProjectUsage scans Claude Code, AGY, and Codex sessions and computes token attribution and cost-of-change metrics for repoPath.
func CollectProjectUsage(repoPath string) (*ProjectUsageResult, error) {
	if repoPath == "" {
		repoPath = "."
	}
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		absPath = repoPath
	}
	absPath = filepath.Clean(absPath)
	repoName := filepath.Base(absPath)

	home, _ := os.UserHomeDir()
	claudeDir := filepath.Join(home, ".claude")
	geminiDir := filepath.Join(home, ".gemini", "antigravity-cli")
	codexDir := filepath.Join(home, ".codex")

	agents := make(map[string]*AgentProjectAttribution)

	// 1. Claude attribution
	claudeAttr := scanClaudeProjectUsage(claudeDir, absPath)
	if claudeAttr != nil && (claudeAttr.Tokens.TotalTokens > 0 || claudeAttr.MatchedSessions > 0) {
		agents["claude"] = claudeAttr
	}

	// 2. AGY attribution
	agyAttr := scanAGYProjectUsage(geminiDir, absPath)
	if agyAttr != nil && (agyAttr.Tokens.TotalTokens > 0 || agyAttr.MatchedSessions > 0) {
		agents["agy"] = agyAttr
	}

	// 3. Codex attribution
	codexAttr := scanCodexProjectUsage(codexDir, absPath)
	if codexAttr != nil && (codexAttr.Tokens.TotalTokens > 0 || codexAttr.MatchedSessions > 0) {
		agents["codex"] = codexAttr
	}

	// Aggregate lifetime totals
	var totBreakdown TokenBreakdown
	for _, a := range agents {
		totBreakdown.InputTokens += a.Tokens.InputTokens
		totBreakdown.OutputTokens += a.Tokens.OutputTokens
		totBreakdown.CacheReadTokens += a.Tokens.CacheReadTokens
		totBreakdown.CacheWriteTokens += a.Tokens.CacheWriteTokens
		totBreakdown.TotalTokens += a.Tokens.TotalTokens
	}

	// 4. Compute Cost-of-Change metrics if repo has Git / tickets
	costOfChange := computeCostOfChange(absPath, totBreakdown)

	return &ProjectUsageResult{
		RepoDir:        absPath,
		RepoName:       repoName,
		LifetimeTokens: totBreakdown.TotalTokens,
		TotalTokens:    totBreakdown,
		Agents:         agents,
		CostOfChange:   costOfChange,
	}, nil
}

func scanClaudeProjectUsage(claudeDir, targetRepo string) *AgentProjectAttribution {
	attr := &AgentProjectAttribution{
		AgentID:     "claude",
		Name:        "Claude Code",
		ModelTokens: make(map[string]int64),
	}

	projectsDir := filepath.Join(claudeDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return attr
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projPath := filepath.Join(projectsDir, entry.Name())
		files, err := os.ReadDir(projPath)
		if err != nil {
			continue
		}

		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}

			fullFile := filepath.Join(projPath, f.Name())
			matched, breakdown, modelTokens := parseClaudeProjectFile(fullFile, targetRepo)
			if matched {
				attr.MatchedSessions++
				attr.Tokens.InputTokens += breakdown.InputTokens
				attr.Tokens.OutputTokens += breakdown.OutputTokens
				attr.Tokens.CacheReadTokens += breakdown.CacheReadTokens
				attr.Tokens.CacheWriteTokens += breakdown.CacheWriteTokens
				attr.Tokens.TotalTokens += breakdown.TotalTokens
				for m, toks := range modelTokens {
					attr.ModelTokens[m] += toks
				}
			}
		}
	}

	return attr
}

func parseClaudeProjectFile(filePath, targetRepo string) (bool, TokenBreakdown, map[string]int64) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, TokenBreakdown{}, nil
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	matched := false
	var breakdown TokenBreakdown
	modelTokens := make(map[string]int64)

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			var entry struct {
				Cwd     string `json:"cwd"`
				Message *struct {
					Model string `json:"model"`
					Usage *struct {
						InputTokens              int64 `json:"input_tokens"`
						OutputTokens             int64 `json:"output_tokens"`
						CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
						CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if json.Unmarshal(line, &entry) == nil {
				if entry.Cwd != "" && pathMatches(entry.Cwd, targetRepo) {
					matched = true
				}
				if entry.Message != nil && entry.Message.Usage != nil {
					u := entry.Message.Usage
					totalTurn := u.InputTokens + u.OutputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
					breakdown.InputTokens += u.InputTokens
					breakdown.OutputTokens += u.OutputTokens
					breakdown.CacheReadTokens += u.CacheReadInputTokens
					breakdown.CacheWriteTokens += u.CacheCreationInputTokens
					breakdown.TotalTokens += totalTurn

					mName := simplifyModelName(entry.Message.Model)
					if mName != "" {
						modelTokens[mName] += totalTurn
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}
	}

	return matched, breakdown, modelTokens
}

func scanAGYProjectUsage(geminiDir, targetRepo string) *AgentProjectAttribution {
	attr := &AgentProjectAttribution{
		AgentID:     "agy",
		Name:        "Antigravity",
		ModelTokens: make(map[string]int64),
	}

	// 1. Scan history.jsonl to find conversations matching targetRepo
	matchedConvIDs := make(map[string]bool)
	histPath := filepath.Join(geminiDir, "history.jsonl")
	if hf, err := os.Open(histPath); err == nil {
		scanner := bufio.NewScanner(hf)
		for scanner.Scan() {
			var h struct {
				Workspace      string `json:"workspace"`
				ConversationID string `json:"conversationId"`
			}
			if json.Unmarshal(scanner.Bytes(), &h) == nil {
				if h.Workspace != "" && pathMatches(h.Workspace, targetRepo) && h.ConversationID != "" {
					matchedConvIDs[h.ConversationID] = true
				}
			}
		}
		hf.Close()
	}

	// 2. Scan brain conversations transcripts
	brainDir := filepath.Join(geminiDir, "brain")
	entries, err := os.ReadDir(brainDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			convID := entry.Name()
			transcriptPath := filepath.Join(brainDir, convID, ".system_generated", "logs", "transcript.jsonl")
			if _, err := os.Stat(transcriptPath); err != nil {
				continue
			}

			// If already matched from history or matches content
			isMatched := matchedConvIDs[convID]
			matched, breakdown, modelTokens := parseAGYTranscript(transcriptPath, targetRepo, isMatched)
			if matched {
				attr.MatchedSessions++
				attr.Tokens.InputTokens += breakdown.InputTokens
				attr.Tokens.OutputTokens += breakdown.OutputTokens
				attr.Tokens.CacheReadTokens += breakdown.CacheReadTokens
				attr.Tokens.CacheWriteTokens += breakdown.CacheWriteTokens
				attr.Tokens.TotalTokens += breakdown.TotalTokens
				for m, toks := range modelTokens {
					attr.ModelTokens[m] += toks
				}
			}
		}
	}

	return attr
}

func parseAGYTranscript(filePath, targetRepo string, alreadyMatched bool) (bool, TokenBreakdown, map[string]int64) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, TokenBreakdown{}, nil
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	matched := alreadyMatched
	var breakdown TokenBreakdown
	modelTokens := make(map[string]int64)

	defaultModel := "Gemini"

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			var step struct {
				Content   string `json:"content"`
				StepIndex int    `json:"step_index"`
				ToolCalls []struct {
					Name string         `json:"name"`
					Args map[string]any `json:"args"`
				} `json:"tool_calls"`
			}
			if json.Unmarshal(line, &step) == nil {
				if !matched {
					if strings.Contains(step.Content, targetRepo) {
						matched = true
					}
					for _, tc := range step.ToolCalls {
						for _, v := range tc.Args {
							if s, ok := v.(string); ok && pathMatches(s, targetRepo) {
								matched = true
							}
						}
					}
				}

				// Estimate tokens from content length if direct token fields absent
				tokEst := int64(assess.EstimateTokens(line))
				if tokEst > 0 {
					breakdown.InputTokens += tokEst * 3 / 4
					breakdown.OutputTokens += tokEst / 4
					breakdown.TotalTokens += tokEst
					modelTokens[defaultModel] += tokEst
				}
			}
		}
		if err != nil {
			break
		}
	}

	return matched, breakdown, modelTokens
}

func scanCodexProjectUsage(codexDir, targetRepo string) *AgentProjectAttribution {
	attr := &AgentProjectAttribution{
		AgentID:     "codex",
		Name:        "OpenAI Codex",
		ModelTokens: make(map[string]int64),
	}

	root := filepath.Join(codexDir, "sessions")
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !strings.HasPrefix(info.Name(), "rollout-") || !strings.HasSuffix(info.Name(), ".jsonl") {
			return nil
		}

		matched, breakdown, modelTokens := parseCodexRollout(path, targetRepo)
		if matched {
			attr.MatchedSessions++
			attr.Tokens.InputTokens += breakdown.InputTokens
			attr.Tokens.OutputTokens += breakdown.OutputTokens
			attr.Tokens.CacheReadTokens += breakdown.CacheReadTokens
			attr.Tokens.CacheWriteTokens += breakdown.CacheWriteTokens
			attr.Tokens.TotalTokens += breakdown.TotalTokens
			for m, toks := range modelTokens {
				attr.ModelTokens[m] += toks
			}
		}
		return nil
	})

	return attr
}

func parseCodexRollout(filePath, targetRepo string) (bool, TokenBreakdown, map[string]int64) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, TokenBreakdown{}, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	matched := false
	var latestBreakdown TokenBreakdown
	modelTokens := make(map[string]int64)
	activeModel := "gpt-5"

	for scanner.Scan() {
		var record struct {
			Type    string `json:"type"`
			Payload struct {
				Cwd   string `json:"cwd"`
				Model string `json:"model"`
				Type  string `json:"type"`
				Info  struct {
					Total struct {
						Input      int64 `json:"input_tokens"`
						Cached     int64 `json:"cached_input_tokens"`
						CacheWrite int64 `json:"cache_write_input_tokens"`
						Output     int64 `json:"output_tokens"`
						Total      int64 `json:"total_tokens"`
					} `json:"total_token_usage"`
				} `json:"info"`
			} `json:"payload"`
		}
		if json.Unmarshal(scanner.Bytes(), &record) != nil {
			continue
		}

		if record.Payload.Cwd != "" && pathMatches(record.Payload.Cwd, targetRepo) {
			matched = true
		}
		if record.Payload.Model != "" {
			activeModel = simplifyModelName(record.Payload.Model)
		}

		if record.Type == "event_msg" && record.Payload.Type == "token_count" {
			t := record.Payload.Info.Total
			if t.Total > 0 {
				latestBreakdown = TokenBreakdown{
					InputTokens:      t.Input,
					CacheReadTokens:  t.Cached,
					CacheWriteTokens: t.CacheWrite,
					OutputTokens:     t.Output,
					TotalTokens:      t.Total,
				}
			}
		}
	}

	if latestBreakdown.TotalTokens > 0 {
		modelTokens[activeModel] = latestBreakdown.TotalTokens
	}

	return matched, latestBreakdown, modelTokens
}

func pathMatches(candidate, target string) bool {
	c := filepath.Clean(candidate)
	t := filepath.Clean(target)
	if c == t || strings.HasPrefix(c, t+"/") {
		return true
	}
	return false
}

func simplifyModelName(model string) string {
	lower := strings.ToLower(model)
	if strings.Contains(lower, "claude-sonnet-5") || strings.Contains(lower, "sonnet-5") {
		return "Sonnet 5"
	}
	if strings.Contains(lower, "claude-sonnet-4-6") || strings.Contains(lower, "sonnet-4.6") {
		return "Sonnet 4.6"
	}
	if strings.Contains(lower, "claude-fable-5") || strings.Contains(lower, "fable-5") {
		return "Fable 5"
	}
	if strings.Contains(lower, "claude-opus") {
		return "Opus"
	}
	if strings.Contains(lower, "claude-haiku") {
		return "Haiku"
	}
	if strings.Contains(lower, "gemini-3.7-flash") || strings.Contains(lower, "gemini-flash") {
		return "Gemini 3.7 Flash"
	}
	if strings.Contains(lower, "gemini-3.7-pro") || strings.Contains(lower, "gemini-pro") {
		return "Gemini 3.7 Pro"
	}
	if strings.Contains(lower, "gpt-5.6") || strings.Contains(lower, "gpt-5") {
		return "gpt-5.6"
	}
	if strings.Contains(lower, "gpt-4") {
		return "gpt-4"
	}
	if model != "" {
		return model
	}
	return "Standard Model"
}

func computeCostOfChange(repoDir string, totalTokens TokenBreakdown) *CostOfChangeMetrics {
	trackRes, err := assess.ExtractMultiTrackHistory(repoDir)
	if err != nil || trackRes == nil {
		return nil
	}

	netCodeLOC := 0
	if codeTrack, ok := trackRes.Tracks[assess.TrackCode]; ok && codeTrack != nil {
		netCodeLOC = codeTrack.CurrentLines
	}

	closedTickets := 0
	if issueTrack, ok := trackRes.Tracks[assess.TrackIssues]; ok && issueTrack != nil {
		closedTickets = issueTrack.ClosedTickets
	}

	toksPerLOC := 0.0
	if netCodeLOC > 0 && totalTokens.TotalTokens > 0 {
		toksPerLOC = float64(totalTokens.TotalTokens) / float64(netCodeLOC)
	}

	toksPerTicket := 0.0
	if closedTickets > 0 && totalTokens.TotalTokens > 0 {
		toksPerTicket = float64(totalTokens.TotalTokens) / float64(closedTickets)
	}

	cacheEfficiency := 0.0
	totalInput := totalTokens.InputTokens + totalTokens.CacheReadTokens + totalTokens.CacheWriteTokens
	if totalInput > 0 {
		cacheEfficiency = float64(totalTokens.CacheReadTokens) / float64(totalInput) * 100.0
	}

	return &CostOfChangeMetrics{
		NetCodeLOC:          netCodeLOC,
		TokensPerLOC:        toksPerLOC,
		ClosedTickets:       closedTickets,
		TokensPerTicket:     toksPerTicket,
		CacheReadEfficiency: cacheEfficiency,
	}
}

// RenderProjectUsageCard formats the ProjectUsageResult into a clean terminal report.
func RenderProjectUsageCard(res *ProjectUsageResult) string {
	if res == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project AI Telemetry: %s (%s)\n", res.RepoName, res.RepoDir))
	sb.WriteString(fmt.Sprintf("Lifetime Attributed Tokens: %s tokens\n", formatTokenMagnitude(res.LifetimeTokens)))

	// Sort agent keys
	agentKeys := make([]string, 0, len(res.Agents))
	for k := range res.Agents {
		agentKeys = append(agentKeys, k)
	}
	sort.Strings(agentKeys)

	for _, k := range agentKeys {
		a := res.Agents[k]
		if a == nil || a.Tokens.TotalTokens == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("  - %-14s %s tokens", a.Name+":", formatTokenMagnitude(a.Tokens.TotalTokens)))

		// Model breakdown if available
		if len(a.ModelTokens) > 0 {
			var modelParts []string
			type mStat struct {
				name   string
				tokens int64
			}
			var mList []mStat
			for m, toks := range a.ModelTokens {
				mList = append(mList, mStat{m, toks})
			}
			sort.Slice(mList, func(i, j int) bool {
				return mList[i].tokens > mList[j].tokens
			})

			for _, m := range mList {
				pct := float64(m.tokens) / float64(a.Tokens.TotalTokens) * 100.0
				if pct >= 1.0 {
					modelParts = append(modelParts, fmt.Sprintf("%s: %.0f%%", m.name, pct))
				} else {
					modelParts = append(modelParts, m.name)
				}
			}
			if len(modelParts) > 0 {
				sb.WriteString(fmt.Sprintf(" (%s)", strings.Join(modelParts, ", ")))
			}
		}
		sb.WriteString("\n")
	}

	if res.CostOfChange != nil {
		coc := res.CostOfChange
		sb.WriteString("\nCost of Change:\n")
		if coc.NetCodeLOC > 0 {
			sb.WriteString(fmt.Sprintf("  - Net Code Lines: +%s LOC (~%s tokens / LOC)\n",
				formatLOC(coc.NetCodeLOC), formatNumberWithCommas(int64(math.Round(coc.TokensPerLOC)))))
		}
		if coc.ClosedTickets > 0 {
			sb.WriteString(fmt.Sprintf("  - Shipped Tickets: %d closed tickets (~%s tokens / ticket)\n",
				coc.ClosedTickets, formatTokenMagnitude(int64(math.Round(coc.TokensPerTicket)))))
		}
		if coc.CacheReadEfficiency > 0 {
			sb.WriteString(fmt.Sprintf("  - Prompt Cache Leverage: %.1f%% cache read efficiency\n", coc.CacheReadEfficiency))
		}
	}

	return sb.String()
}

func formatTokenMagnitude(n int64) string {
	if n < 0 {
		return "0"
	}
	if n >= 1_000_000_000 {
		return fmt.Sprintf("%.2fB", float64(n)/1_000_000_000.0)
	}
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000.0)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1_000.0)
	}
	return fmt.Sprintf("%d", n)
}

func formatLOC(n int) string {
	if n >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000.0)
	}
	return fmt.Sprintf("%d", n)
}

func formatNumberWithCommas(n int64) string {
	if n < 0 {
		return "-" + formatNumberWithCommas(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var res []string
	for len(s) > 3 {
		res = append([]string{s[len(s)-3:]}, res...)
		s = s[:len(s)-3]
	}
	if len(s) > 0 {
		res = append([]string{s}, res...)
	}
	return strings.Join(res, ",")
}

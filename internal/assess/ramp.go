package assess

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// RAMPRuleSetVersion identifies the small versioned rule set used for classification.
const RAMPRuleSetVersion = "ramp-v1"

// RAMPLevel represents the 4-tier repository AI maturity level.
type RAMPLevel string

const (
	RAMPLevelL1 RAMPLevel = "L1" // Unconfigured
	RAMPLevelL2 RAMPLevel = "L2" // Grounded prompting
	RAMPLevelL3 RAMPLevel = "L3" // Agent augmented
	RAMPLevelL4 RAMPLevel = "L4" // Orchestration
)

// RAMPCategory represents one of the nine RAMP artifact categories.
type RAMPCategory string

const (
	// Level 1 (1 category)
	RAMPCatUnconfigured RAMPCategory = "unconfigured"

	// Level 2 (3 categories)
	RAMPCatAIRules       RAMPCategory = "ai_rules"
	RAMPCatToolConfig    RAMPCategory = "tool_config"
	RAMPCatAgentStandards RAMPCategory = "agent_standards"

	// Level 3 (3 categories)
	RAMPCatNamedAgents      RAMPCategory = "named_agents"
	RAMPCatReusableCommands RAMPCategory = "reusable_commands"
	RAMPCatDomainSkills     RAMPCategory = "domain_skills"

	// Level 4 (2 categories)
	RAMPCatMultiAgentFlows RAMPCategory = "multi_agent_flows"
	RAMPCatSessionRecords  RAMPCategory = "session_records"
)

// RAMPConfidence describes the classification confidence.
type RAMPConfidence string

const (
	ConfidenceHigh    RAMPConfidence = "high"
	ConfidenceMedium  RAMPConfidence = "medium"
	ConfidenceLow     RAMPConfidence = "low"
	ConfidenceUnknown RAMPConfidence = "unknown"
)

// ArtifactStatus describes whether an artifact is committed, uncommitted, or projected.
type ArtifactStatus string

const (
	StatusCommitted   ArtifactStatus = "committed"
	StatusUncommitted ArtifactStatus = "uncommitted"
	StatusProjected   ArtifactStatus = "projected"
)

// RAMPArtifact captures inspectable evidence for an AI maturity artifact.
type RAMPArtifact struct {
	Path       string         `json:"path"`
	Category   RAMPCategory   `json:"category"`
	Level      RAMPLevel      `json:"level"`
	RuleID     string         `json:"rule_id"`
	Reason     string         `json:"reason"`
	Confidence RAMPConfidence `json:"confidence"`
	Status     ArtifactStatus `json:"status"`
	IsBaseline bool           `json:"is_baseline"`
}

// RAMPProfile holds the full offline, read-only RAMP evaluation result.
type RAMPProfile struct {
	RuleSetVersion      string         `json:"rule_set_version"`
	BaselineLevel       RAMPLevel      `json:"baseline_level"`
	ProjectedLevel      RAMPLevel      `json:"projected_level"`
	BaselineCategories  []RAMPCategory `json:"baseline_categories,omitempty"`
	ProjectedCategories []RAMPCategory `json:"projected_categories,omitempty"`
	Artifacts           []RAMPArtifact `json:"artifacts,omitempty"`
	CoherenceAlerts     []string       `json:"coherence_alerts,omitempty"`
	GitAvailable        bool           `json:"git_available"`
	GitNotice           string         `json:"git_notice,omitempty"`
	Summary             string         `json:"summary"`
}

// LevelDescription returns a human-readable title for a RAMP level.
func LevelDescription(level RAMPLevel) string {
	switch level {
	case RAMPLevelL1:
		return "Unconfigured"
	case RAMPLevelL2:
		return "Grounded Prompting"
	case RAMPLevelL3:
		return "Agent Augmented"
	case RAMPLevelL4:
		return "Orchestration"
	default:
		return string(level)
	}
}

// CategoryDescription returns a human-readable description for a RAMP category.
func CategoryDescription(cat RAMPCategory) string {
	switch cat {
	case RAMPCatUnconfigured:
		return "Unconfigured (no validated AI artifacts)"
	case RAMPCatAIRules:
		return "AI Behavior Rules (AGENTS.md, instructions, prompt rules)"
	case RAMPCatToolConfig:
		return "AI Tool Configuration (.claude, .cursor, aider configs)"
	case RAMPCatAgentStandards:
		return "Agent Architecture & Standards (invariants, workflow docs)"
	case RAMPCatNamedAgents:
		return "Named Agents (subagent personas, roles)"
	case RAMPCatReusableCommands:
		return "Reusable Commands (slash commands, prompt procedures)"
	case RAMPCatDomainSkills:
		return "Domain Skills (SKILL.md, agent tools)"
	case RAMPCatMultiAgentFlows:
		return "Multi-Agent Flows (runnable multi-agent orchestration)"
	case RAMPCatSessionRecords:
		return "Agent Session Records (committed transcripts/trajectories)"
	default:
		return string(cat)
	}
}

func levelRank(level RAMPLevel) int {
	switch level {
	case RAMPLevelL4:
		return 4
	case RAMPLevelL3:
		return 3
	case RAMPLevelL2:
		return 2
	case RAMPLevelL1:
		return 1
	default:
		return 0
	}
}

// maxBoundedReadBytes is the upper bound for content inspection (64KB).
const maxBoundedReadBytes = 64 * 1024

// readBoundedFile safely reads up to maxBoundedReadBytes from path.
func readBoundedFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var buf bytes.Buffer
	_, err = io.CopyN(&buf, f, maxBoundedReadBytes)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return buf.Bytes(), nil
}

// inspectGitMetadata checks if dir is inside a git repo and retrieves tracked and dirty files.
func inspectGitMetadata(dir string) (bool, map[string]bool, map[string]bool) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	if err := cmd.Run(); err != nil {
		return false, nil, nil
	}

	trackedMap := make(map[string]bool)
	lsCmd := exec.Command("git", "-C", dir, "ls-files")
	if out, err := lsCmd.Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				trackedMap[filepath.ToSlash(line)] = true
			}
		}
	}

	dirtyMap := make(map[string]bool)
	statusCmd := exec.Command("git", "-C", dir, "status", "--porcelain")
	if out, err := statusCmd.Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if len(line) >= 3 {
				// Porcelain v1 format: XY PATH (or PATH -> NEWPATH)
				p := strings.TrimSpace(line[2:])
				if arrowIdx := strings.Index(p, " -> "); arrowIdx != -1 {
					p = strings.TrimSpace(p[arrowIdx+4:])
				}
				dirtyMap[filepath.ToSlash(p)] = true
			}
		}
	}

	return true, trackedMap, dirtyMap
}

// classifyRAMPPath inspects a file path and bounded content to classify RAMP artifact.
func classifyRAMPPath(relPath string, data []byte) *RAMPArtifact {
	cleanRel := filepath.ToSlash(relPath)
	base := filepath.Base(cleanRel)
	lowerBase := strings.ToLower(base)
	lowerRel := strings.ToLower(cleanRel)

	// Explicit false positives to filter out early
	if base == "Makefile" || base == "makefile" || base == "GNUmakefile" {
		return nil // Build targets (e.g. test-q1) are enforcement signals, not RAMP artifacts
	}
	if strings.HasSuffix(lowerBase, ".service") {
		return nil // Systemd service units are not AI artifacts
	}
	if strings.HasPrefix(lowerRel, ".github/workflows/") || strings.HasPrefix(lowerRel, ".gitlab-ci") {
		return nil // CI workflows are not multi-agent flows
	}
	if lowerBase == "readme.md" || lowerBase == "license" || lowerBase == "contributing.md" {
		// Generic project docs are not AI rules unless they contain explicit AI agent instruction headers
		if !containsAIInstructionHeader(data) {
			return nil
		}
	}

	// 1. Level 4: Orchestration
	// 1a. Session transcripts / logs
	if strings.HasSuffix(lowerRel, ".jsonl") && (strings.Contains(lowerRel, "transcript") || strings.Contains(lowerRel, "agent-sessions") || strings.Contains(lowerRel, ".system_generated/logs/")) {
		return &RAMPArtifact{
			Path:       cleanRel,
			Category:   RAMPCatSessionRecords,
			Level:      RAMPLevelL4,
			RuleID:     "RULE_SESSION_TRANSCRIPTS",
			Reason:     "Committed agent session execution log or trajectory record",
			Confidence: ConfidenceHigh,
		}
	}

	// 1b. Multi-agent flows
	if (strings.HasPrefix(lowerRel, "flows/") || strings.Contains(lowerRel, "/flows/") || strings.Contains(lowerRel, "multi-agent")) &&
		(strings.HasSuffix(lowerRel, ".yaml") || strings.HasSuffix(lowerRel, ".yml") || strings.HasSuffix(lowerRel, ".json")) {
		if containsMultiAgentMarkers(data) {
			return &RAMPArtifact{
				Path:       cleanRel,
				Category:   RAMPCatMultiAgentFlows,
				Level:      RAMPLevelL4,
				RuleID:     "RULE_MULTI_AGENT_FLOW",
				Reason:     "Runnable multi-agent orchestration flow configuration",
				Confidence: ConfidenceHigh,
			}
		}
	}

	// 2. Level 3: Agent Augmented
	// 2a. Domain Skills
	if lowerBase == "skill.md" || strings.Contains(lowerRel, "/skills/") && strings.HasSuffix(lowerRel, ".md") {
		if isSkillDefinition(data) {
			return &RAMPArtifact{
				Path:       cleanRel,
				Category:   RAMPCatDomainSkills,
				Level:      RAMPLevelL3,
				RuleID:     "RULE_DOMAIN_SKILLS",
				Reason:     "Agent skill definition with structured instructions",
				Confidence: ConfidenceHigh,
			}
		}
	}

	// 2b. Reusable Commands
	if (strings.Contains(lowerRel, ".claude/commands/") || strings.Contains(lowerRel, ".codex/commands/") || strings.Contains(lowerRel, ".prime/agent/commands/")) &&
		strings.HasSuffix(lowerRel, ".md") {
		return &RAMPArtifact{
			Path:       cleanRel,
			Category:   RAMPCatReusableCommands,
			Level:      RAMPLevelL3,
			RuleID:     "RULE_REUSABLE_COMMANDS",
			Reason:     "Reusable agent slash command definition",
			Confidence: ConfidenceHigh,
		}
	}

	// 2c. Named Agents
	if (strings.Contains(lowerRel, ".claude/agents/") || strings.Contains(lowerRel, ".codex/agents/") || strings.Contains(lowerRel, "agents/subagents/")) &&
		(strings.HasSuffix(lowerRel, ".md") || strings.HasSuffix(lowerRel, ".yaml") || strings.HasSuffix(lowerRel, ".json")) {
		return &RAMPArtifact{
			Path:       cleanRel,
			Category:   RAMPCatNamedAgents,
			Level:      RAMPLevelL3,
			RuleID:     "RULE_NAMED_AGENTS",
			Reason:     "Named subagent definition or persona specification",
			Confidence: ConfidenceHigh,
		}
	}

	// 3. Level 2: Grounded Prompting
	// 3a. AI Behavior Rules (AGENTS.md, CLAUDE.md, .cursorrules, .windsurfrules, copilot-instructions)
	if base == "AGENTS.md" || base == "CLAUDE.md" || cleanRel == ".claude/CLAUDE.md" || cleanRel == ".prime/agent/AGENTS.md" || cleanRel == ".codex/AGENTS.md" {
		return &RAMPArtifact{
			Path:       cleanRel,
			Category:   RAMPCatAIRules,
			Level:      RAMPLevelL2,
			RuleID:     "RULE_AGENTS_MD",
			Reason:     "Canonical agent instructions and behavior rules",
			Confidence: ConfidenceHigh,
		}
	}
	if base == ".cursorrules" || (strings.HasPrefix(lowerRel, ".cursor/rules/") && (strings.HasSuffix(lowerRel, ".mdc") || strings.HasSuffix(lowerRel, ".md"))) {
		return &RAMPArtifact{
			Path:       cleanRel,
			Category:   RAMPCatAIRules,
			Level:      RAMPLevelL2,
			RuleID:     "RULE_CURSOR_RULES",
			Reason:     "Cursor IDE AI behavior rules",
			Confidence: ConfidenceHigh,
		}
	}
	if base == ".windsurfrules" {
		return &RAMPArtifact{
			Path:       cleanRel,
			Category:   RAMPCatAIRules,
			Level:      RAMPLevelL2,
			RuleID:     "RULE_WINDSURF_RULES",
			Reason:     "Windsurf AI behavior rules",
			Confidence: ConfidenceHigh,
		}
	}
	if cleanRel == ".github/copilot-instructions.md" || base == ".copilot-instructions.md" {
		return &RAMPArtifact{
			Path:       cleanRel,
			Category:   RAMPCatAIRules,
			Level:      RAMPLevelL2,
			RuleID:     "RULE_COPILOT_INSTRUCTIONS",
			Reason:     "GitHub Copilot custom instructions",
			Confidence: ConfidenceHigh,
		}
	}

	// 3b. AI Tool Configuration (.claude/settings.json, aider, continue, cursor)
	if cleanRel == ".claude/settings.json" || cleanRel == ".claude/config.json" {
		return &RAMPArtifact{
			Path:       cleanRel,
			Category:   RAMPCatToolConfig,
			Level:      RAMPLevelL2,
			RuleID:     "RULE_CLAUDE_CONFIG",
			Reason:     "Claude Code tool and permission configuration",
			Confidence: ConfidenceHigh,
		}
	}
	if cleanRel == ".aider.conf.yml" || cleanRel == ".aider.yaml" || strings.HasPrefix(lowerRel, ".continue/config") || strings.HasPrefix(lowerRel, ".cursor/settings") {
		return &RAMPArtifact{
			Path:       cleanRel,
			Category:   RAMPCatToolConfig,
			Level:      RAMPLevelL2,
			RuleID:     "RULE_AI_TOOL_CONFIG",
			Reason:     "AI coding tool configuration file",
			Confidence: ConfidenceHigh,
		}
	}

	// 3c. Agent Architecture & Standards
	if cleanRel == "docs/practices/AgenticLoop.md" || cleanRel == "docs/AgenticLoop.md" || cleanRel == "docs/practices/IssueTracking.md" || cleanRel == "docs/IssueTracking.md" {
		return &RAMPArtifact{
			Path:       cleanRel,
			Category:   RAMPCatAgentStandards,
			Level:      RAMPLevelL2,
			RuleID:     "RULE_AGENTIC_STANDARDS",
			Reason:     "Normative agentic loop practices and coding standards",
			Confidence: ConfidenceHigh,
		}
	}
	if (lowerBase == "prompt.md" || lowerBase == "system_prompt.md" || lowerBase == "agent_guide.md") && len(data) > 0 {
		return &RAMPArtifact{
			Path:       cleanRel,
			Category:   RAMPCatAgentStandards,
			Level:      RAMPLevelL2,
			RuleID:     "RULE_PROMPT_DOC",
			Reason:     "Agent prompt or architecture specification doc",
			Confidence: ConfidenceHigh,
		}
	}

	// Ambiguous path check (e.g. scripts/agent_helper.sh, misc/prompt_template.txt) -> marked unknown, do not promote level
	if strings.Contains(lowerRel, "agent") || strings.Contains(lowerRel, "prompt") || strings.Contains(lowerRel, "skill") {
		if strings.HasSuffix(lowerRel, ".sh") || strings.HasSuffix(lowerRel, ".py") || strings.HasSuffix(lowerRel, ".txt") || strings.HasSuffix(lowerRel, ".json") {
			return &RAMPArtifact{
				Path:       cleanRel,
				Category:   RAMPCatUnconfigured,
				Level:      RAMPLevelL1,
				RuleID:     "RULE_AMBIGUOUS_AI_ARTIFACT",
				Reason:     "Ambiguous AI-related path without verified RAMP structure; marked unknown to prevent false promotion",
				Confidence: ConfidenceUnknown,
			}
		}
	}

	return nil
}

func containsAIInstructionHeader(data []byte) bool {
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "<!-- harnez:begin") ||
		strings.Contains(lower, "ai instructions") ||
		strings.Contains(lower, "instructions for ai") ||
		strings.Contains(lower, "agent guidelines")
}

func containsMultiAgentMarkers(data []byte) bool {
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "agents:") ||
		strings.Contains(lower, "subagents:") ||
		strings.Contains(lower, "orchestrator") ||
		strings.Contains(lower, "workflow:") ||
		strings.Contains(lower, "crew:") ||
		strings.Contains(lower, "pipeline:")
}

func isSkillDefinition(data []byte) bool {
	if len(data) == 0 {
		return true // SKILL.md path itself qualifies if created
	}
	s := string(data)
	return strings.Contains(s, "name:") || strings.Contains(s, "# ") || strings.Contains(s, "description:")
}

// AssessRAMP evaluates the RAMP profile of a repository (offline, read-only).
func AssessRAMP(repoDir string) (*RAMPProfile, error) {
	return AssessRAMPWithProjected(repoDir, nil)
}

// AssessRAMPWithProjected evaluates RAMP with optional projected files (e.g. from init).
func AssessRAMPWithProjected(repoDir string, projectedFiles []string) (*RAMPProfile, error) {
	absDir, err := filepath.Abs(repoDir)
	if err != nil {
		absDir = repoDir
	}

	gitAvail, trackedMap, dirtyMap := inspectGitMetadata(absDir)

	var artifacts []RAMPArtifact
	foundPaths := make(map[string]bool)

	// Scan files on disk
	err = filepath.WalkDir(absDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == ".hg" || name == ".svn" || name == "node_modules" || name == "vendor" || name == ".cache" || name == "target" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(absDir, p)
		if err != nil {
			rel = p
		}
		cleanRel := filepath.ToSlash(rel)

		data, err := readBoundedFile(p)
		if err != nil {
			return nil
		}

		art := classifyRAMPPath(cleanRel, data)
		if art != nil {
			foundPaths[cleanRel] = true
			if gitAvail {
				if trackedMap[cleanRel] && !dirtyMap[cleanRel] {
					art.Status = StatusCommitted
					art.IsBaseline = true
				} else {
					art.Status = StatusUncommitted
					art.IsBaseline = false
				}
			} else {
				art.Status = StatusUncommitted
				art.IsBaseline = false
			}
			artifacts = append(artifacts, *art)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory for RAMP: %w", err)
	}

	// Add projected files (if not already committed on disk)
	for _, proj := range projectedFiles {
		cleanProj := filepath.ToSlash(proj)
		alreadyCommitted := false
		for _, a := range artifacts {
			if a.Path == cleanProj && a.Status == StatusCommitted {
				alreadyCommitted = true
				break
			}
		}
		if !alreadyCommitted {
			// Classify projected file without on-disk content
			art := classifyRAMPPath(cleanProj, nil)
			if art == nil {
				// Default L2 for AGENTS.md / docs
				if strings.HasSuffix(cleanProj, "AGENTS.md") || strings.HasSuffix(cleanProj, "CLAUDE.md") {
					art = &RAMPArtifact{
						Path:       cleanProj,
						Category:   RAMPCatAIRules,
						Level:      RAMPLevelL2,
						RuleID:     "RULE_AGENTS_MD",
						Reason:     "Canonical agent instructions (projected write)",
						Confidence: ConfidenceHigh,
					}
				} else if strings.HasPrefix(cleanProj, "docs/") {
					art = &RAMPArtifact{
						Path:       cleanProj,
						Category:   RAMPCatAgentStandards,
						Level:      RAMPLevelL2,
						RuleID:     "RULE_AGENTIC_STANDARDS",
						Reason:     "Projected agent guidance doc",
						Confidence: ConfidenceHigh,
					}
				}
			}
			if art != nil {
				art.Status = StatusProjected
				art.IsBaseline = false
				// If previously found as uncommitted, replace or keep projected label
				replaced := false
				for i, a := range artifacts {
					if a.Path == cleanProj {
						artifacts[i] = *art
						replaced = true
						break
					}
				}
				if !replaced {
					artifacts = append(artifacts, *art)
				}
			}
		}
	}

	// Sort artifacts deterministically by path
	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].Path < artifacts[j].Path
	})

	// Calculate baseline and projected levels
	baselineLevel := RAMPLevelL1
	projectedLevel := RAMPLevelL1

	baselineCatMap := make(map[RAMPCategory]bool)
	projectedCatMap := make(map[RAMPCategory]bool)

	for _, art := range artifacts {
		if art.Confidence == ConfidenceUnknown {
			continue // Do not promote level on unknown/ambiguous artifacts
		}

		if art.Status == StatusCommitted && art.IsBaseline {
			if levelRank(art.Level) > levelRank(baselineLevel) {
				baselineLevel = art.Level
			}
			baselineCatMap[art.Category] = true
		}

		// Projected level considers all qualifying artifacts
		if levelRank(art.Level) > levelRank(projectedLevel) {
			projectedLevel = art.Level
		}
		projectedCatMap[art.Category] = true
	}

	// If no git metadata is available, add explicit notice
	var gitNotice string
	if !gitAvail {
		gitNotice = "Git metadata absent: existing working-tree files treated as uncommitted/projected"
	}

	// Compute coherence alerts on highest achieved level
	evalLevel := baselineLevel
	catMap := baselineCatMap
	if levelRank(projectedLevel) > levelRank(baselineLevel) {
		evalLevel = projectedLevel
		catMap = projectedCatMap
	}

	var alerts []string
	if evalLevel == RAMPLevelL4 {
		hasL3 := catMap[RAMPCatNamedAgents] || catMap[RAMPCatReusableCommands] || catMap[RAMPCatDomainSkills]
		hasL2 := catMap[RAMPCatAIRules] || catMap[RAMPCatToolConfig] || catMap[RAMPCatAgentStandards]
		if !hasL3 {
			alerts = append(alerts, "Coherence warning: Level L4 (Orchestration) evidenced without L3 (Agent Augmented) capabilities")
		}
		if !hasL2 {
			alerts = append(alerts, "Coherence warning: Level L4 (Orchestration) evidenced without L2 (Grounded Prompting) baseline rules")
		}
	} else if evalLevel == RAMPLevelL3 {
		hasL2 := catMap[RAMPCatAIRules] || catMap[RAMPCatToolConfig] || catMap[RAMPCatAgentStandards]
		if !hasL2 {
			alerts = append(alerts, "Coherence warning: Level L3 (Agent Augmented) evidenced without L2 (Grounded Prompting) baseline rules")
		}
	}

	var baselineCats []RAMPCategory
	for c := range baselineCatMap {
		baselineCats = append(baselineCats, c)
	}
	sort.Slice(baselineCats, func(i, j int) bool { return baselineCats[i] < baselineCats[j] })

	var projectedCats []RAMPCategory
	for c := range projectedCatMap {
		projectedCats = append(projectedCats, c)
	}
	sort.Slice(projectedCats, func(i, j int) bool { return projectedCats[i] < projectedCats[j] })

	profile := &RAMPProfile{
		RuleSetVersion:      RAMPRuleSetVersion,
		BaselineLevel:       baselineLevel,
		ProjectedLevel:      projectedLevel,
		BaselineCategories:  baselineCats,
		ProjectedCategories: projectedCats,
		Artifacts:           artifacts,
		CoherenceAlerts:     alerts,
		GitAvailable:        gitAvail,
		GitNotice:           gitNotice,
		Summary:             fmt.Sprintf("RAMP-informed estimate: Baseline %s (%s), Projected %s (%s)", baselineLevel, LevelDescription(baselineLevel), projectedLevel, LevelDescription(projectedLevel)),
	}

	return profile, nil
}

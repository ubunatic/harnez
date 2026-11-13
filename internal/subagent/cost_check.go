package subagent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// CostCheckInput holds one finished turn's numbers for CheckCost (issue 985 M4).
// A value <= 0 means the provider did not report it; unknown values never trigger a check.
type CostCheckInput struct {
	Role         string
	Model        string
	Turn         int // 1-based turn number in the session
	CachedTokens int // cached input tokens re-read this turn
	NewTokens    int // uncached input plus output tokens this turn
	ContextUsage int // context tokens at the end of the turn
	CompactLimit int // the model's compaction limit (compact_thresholds), not its context window
	FreshStart   int // estimated context tokens of a fresh session, e.g. turn 1's context usage
}

// CheckCost reports whether a session has become more expensive than a fresh one. It returns the
// "[check: ...]" line and true when any spec threshold is crossed, or "" and false otherwise.
// A zero threshold disables that trigger.
func CheckCost(spec CostCheckSpec, in CostCheckInput) (string, bool) {
	if in.Turn < spec.MinTurns {
		return "", false
	}
	ratioKnown := in.NewTokens > 0 && in.CachedTokens > 0
	overRatio := spec.CachedToNewRatio > 0 && ratioKnown &&
		float64(in.CachedTokens)/float64(in.NewTokens) >= spec.CachedToNewRatio
	overContext := spec.ContextUsageFraction > 0 && in.ContextUsage > 0 && in.CompactLimit > 0 &&
		float64(in.ContextUsage) >= spec.ContextUsageFraction*float64(in.CompactLimit)
	if !overRatio && !overContext {
		return "", false
	}
	var reread string
	if ratioKnown {
		// Round down, as in the ticket's example (7.1M for 175k is 40x): the line means "at least N times".
		reread = fmt.Sprintf("turn re-read %s cached for %s new (%dx)", HumanCount(in.CachedTokens),
			HumanCount(in.NewTokens), in.CachedTokens/in.NewTokens)
	} else {
		reread = "turn re-read unknown"
	}
	parts := []string{
		reread,
		"context usage " + knownCount(in.ContextUsage),
		"fresh start ~" + knownCount(in.FreshStart),
		fmt.Sprintf("consider: harnez agent start --role %s --model %s -f <ticket>", in.Role, in.Model),
	}
	return "[check: " + strings.Join(parts, "; ") + "]", true
}

func knownCount(n int) string {
	if n <= 0 {
		return "unknown"
	}
	return HumanCount(n)
}

// HumanCount renders 1230742 as 1.2M and 32700 as 32.7k.
func HumanCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 10_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

type costCheckConfig struct {
	Agent struct {
		CostCheck struct {
			CachedToNewRatio     *float64 `yaml:"cached_to_new_ratio"`
			ContextUsageFraction *float64 `yaml:"context_usage_fraction"`
			MinTurns             *int     `yaml:"min_turns"`
		} `yaml:"cost_check"`
	} `yaml:"agent"`
}

// EffectiveCostCheckSpec returns the spec/agent.yaml cost_check settings,
// overlaid with any agent.cost_check overrides in ~/.harnez/config.yaml.
func EffectiveCostCheckSpec() (CostCheckSpec, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return CostCheckSpec{}, err
	}
	out := spec.CostCheck
	home, err := os.UserHomeDir()
	if err != nil {
		return out, nil
	}
	b, readErr := os.ReadFile(filepath.Join(home, ".harnez", "config.yaml"))
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return out, nil
		}
		return CostCheckSpec{}, fmt.Errorf("read agent cost_check config: %w", readErr)
	}
	var cfg costCheckConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return CostCheckSpec{}, fmt.Errorf("parse agent cost_check config: %w", err)
	}
	cc := cfg.Agent.CostCheck
	if cc.CachedToNewRatio != nil {
		if *cc.CachedToNewRatio < 0 {
			return CostCheckSpec{}, fmt.Errorf("agent.cost_check.cached_to_new_ratio must be >= 0")
		}
		out.CachedToNewRatio = *cc.CachedToNewRatio
	}
	if cc.ContextUsageFraction != nil {
		if *cc.ContextUsageFraction < 0 {
			return CostCheckSpec{}, fmt.Errorf("agent.cost_check.context_usage_fraction must be >= 0")
		}
		out.ContextUsageFraction = *cc.ContextUsageFraction
	}
	if cc.MinTurns != nil {
		if *cc.MinTurns < 0 {
			return CostCheckSpec{}, fmt.Errorf("agent.cost_check.min_turns must be >= 0")
		}
		out.MinTurns = *cc.MinTurns
	}
	return out, nil
}

package subagent

import (
	"fmt"
	"strings"
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

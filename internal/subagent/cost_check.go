package subagent

import "fmt"

// CheckCost evaluates whether a session is becoming expensive based on the
// defined CostCheckSpec thresholds, returning a formatted message and true if a
// threshold was crossed, or false otherwise.
func CheckCost(spec CostCheckSpec, model Model, turn int, cachedTokens, newTokens, contextUsage, compactLimit int) (string, bool) {
	if spec.MinTurns > 0 && turn < spec.MinTurns {
		return "", false
	}
	if newTokens <= 0 {
		newTokens = 1 // Prevent division by zero, though newTokens should generally be > 0.
	}

	ratio := float64(cachedTokens) / float64(newTokens)
	overRatio := spec.CachedToNewRatio > 0 && ratio >= spec.CachedToNewRatio

	overContext := false
	if spec.ContextUsageFraction > 0 && compactLimit > 0 {
		overContext = float64(contextUsage) >= spec.ContextUsageFraction*float64(compactLimit)
	}

	if overRatio || overContext {
		freshCost := newTokens // Rough estimate of what a fresh session would start at
		return fmt.Sprintf("[check: turn re-read %s cached for %s new (%.0fx); context usage %s; fresh start ~%s; consider: harnez agent start --role <role> --model %s -f <ticket>]",
			HumanCount(cachedTokens), HumanCount(newTokens), ratio,
			HumanCount(contextUsage), HumanCount(freshCost), model.Name), true
	}

	return "", false
}

// HumanCount renders 1230742 as 1.2M and 32700 as 32.7k.
func HumanCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 10_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprintf("%d", n)
}

// Package compactcheck implements a cheap, no-LLM-call heuristic for
// assessing whether a /compact (context summarization) likely dropped
// information that mattered — see issue 180.
//
// This is deliberately a manual/opt-in comparator, not automatic watching:
// Claude Code exposes a PreCompact hook (fires before compaction, with
// access to the pre-compact transcript) but no PostCompact hook, so there
// is no host-native signal harnez can hook to observe the post-compact
// summary automatically. See issue 180's Research Findings section for the
// full writeup. `harnez compact-check <before> <after>` lets a user or
// agent run the same comparison manually against two saved snapshots
// whenever they suspect a bad compaction.
package compactcheck

import (
	"fmt"
	"regexp"
	"strings"
)

// identifierRE matches path-like or identifier-like tokens worth tracking
// across a compaction: file paths, dotted/snake_case/camelCase identifiers,
// ticket references, etc. Deliberately permissive and cheap (single regex
// pass) rather than a real tokenizer/AST — this is a v1 heuristic, not a
// precise analysis.
var identifierRE = regexp.MustCompile(`[A-Za-z0-9_][A-Za-z0-9_./-]{5,}[A-Za-z0-9_/]`)

// minMentions is the minimum occurrence count in the "before" text for an
// identifier to be considered load-bearing enough to check for survival.
// Identifiers mentioned only once are too noisy a signal (could be a typo,
// a one-off aside) to flag confidently at this heuristic's cost/precision
// tradeoff.
const minMentions = 2

// Report summarizes a before/after compaction comparison.
type Report struct {
	BeforeBytes        int
	AfterBytes         int
	ReductionRatio     float64 // 1 - AfterBytes/BeforeBytes; 0 if BeforeBytes is 0
	DroppedIdentifiers []string
}

// Compare extracts repeated identifiers from before, and flags any that
// vanish entirely from after. A large ReductionRatio is expected and fine on
// its own (that's the point of compaction) — DroppedIdentifiers is the
// actionable signal: specific things that were mentioned repeatedly
// pre-compact and then disappeared entirely.
func Compare(before, after string) Report {
	r := Report{
		BeforeBytes: len(before),
		AfterBytes:  len(after),
	}
	if r.BeforeBytes > 0 {
		r.ReductionRatio = 1 - float64(r.AfterBytes)/float64(r.BeforeBytes)
	}

	beforeCounts := countIdentifiers(before)
	afterCounts := countIdentifiers(after)

	var dropped []string
	for id, n := range beforeCounts {
		if n < minMentions {
			continue
		}
		if afterCounts[id] == 0 {
			dropped = append(dropped, id)
		}
	}
	// Deterministic output order.
	sortStrings(dropped)
	r.DroppedIdentifiers = dropped
	return r
}

func countIdentifiers(s string) map[string]int {
	counts := make(map[string]int)
	for _, m := range identifierRE.FindAllString(s, -1) {
		counts[m]++
	}
	return counts
}

func sortStrings(s []string) {
	// Small helper to avoid importing sort in a way that shadows anything;
	// insertion sort is plenty for the handful of dropped identifiers a v1
	// heuristic is expected to surface.
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// FormatReport renders a Report as a short, human-readable summary suitable
// for stdout or a telemetry note.
func FormatReport(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "compact-check: %d -> %d bytes (%.0f%% reduction)\n", r.BeforeBytes, r.AfterBytes, r.ReductionRatio*100)
	if len(r.DroppedIdentifiers) == 0 {
		b.WriteString("no repeated identifiers vanished across the compaction\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%d identifier(s) mentioned %d+ times before, absent after:\n", len(r.DroppedIdentifiers), minMentions)
	for _, id := range r.DroppedIdentifiers {
		fmt.Fprintf(&b, "  - %s\n", id)
	}
	return b.String()
}

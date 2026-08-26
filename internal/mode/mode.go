package mode

import (
	"fmt"
	"os"
	"strings"

	"ubunatic.com/harnez/internal/markdown"
)

// Tier represents the ConciseMode terseness level.
type Tier string

const (
	TierOff      Tier = "off"
	TierLite     Tier = "lite"
	TierStandard Tier = "std"
	TierUltra    Tier = "ultra"
)

const ConciseModeSection = "Concise Mode"

// TierInfo holds metadata and formatting for a ConciseMode tier.
type TierInfo struct {
	Tier       Tier
	Level      int
	Name       string
	ShortDesc  string
	Directive  string
	AgentsLine string
}

var tiers = map[Tier]TierInfo{
	TierOff: {
		Tier:      TierOff,
		Level:     0,
		Name:      "Off",
		ShortDesc: "ConciseMode disabled / default narrative behavior",
		Directive: "[HARNEZ DIRECTIVE: Operational mode reset to default (ConciseMode OFF).\n- Normal conversational narrative and detailed explanations restored.]",
	},
	TierLite: {
		Tier:       TierLite,
		Level:      1,
		Name:       "Concise Lite",
		ShortDesc:  "Professional Terse, no fluff, complete sentences",
		Directive:  "[HARNEZ DIRECTIVE: Operational mode switched to Concise Lite (Level 1).\n- Drop opening pleasantries and speculative closing remarks.\n- Keep full grammar and complete-sentence explanations.\n- Core invariant: preserve all code, diffs, tool parameters, and command syntax 100% verbatim.]",
		AgentsLine: "## Operational Mode: Concise Lite (Level 1)\n- Drop opening pleasantries and speculative closing remarks.\n- Keep full grammar and complete-sentence explanations.\n- Core invariant: preserve all code, diffs, tool parameters, and command syntax 100% verbatim.\n- Reference: @docs/practices/ConciseMode.md",
	},
	TierStandard: {
		Tier:       TierStandard,
		Level:      2,
		Name:       "Concise Standard",
		ShortDesc:  "Telegraphic fragments, zero filler",
		Directive:  "[HARNEZ DIRECTIVE: Operational mode switched to Concise Standard (Level 2).\n- Strip conversational filler, pleasantries, and connective narrative.\n- Use dense telegraphic fragments: Action -> Finding -> Patch.\n- Core invariant: preserve all code, diffs, tool parameters, and command syntax 100% verbatim.]",
		AgentsLine: "## Operational Mode: Concise Standard (Level 2)\n- Strip conversational filler, pleasantries, and connective narrative.\n- Use dense telegraphic fragments: Action -> Finding -> Patch.\n- Core invariant: preserve all code, diffs, tool parameters, and command syntax 100% verbatim.\n- Reference: @docs/practices/ConciseMode.md",
	},
	TierUltra: {
		Tier:       TierUltra,
		Level:      3,
		Name:       "Concise Ultra",
		ShortDesc:  "Diffs/status only, zero narrative",
		Directive:  "[HARNEZ DIRECTIVE: Operational mode switched to Concise Ultra (Level 3).\n- Output confined to essential diffs, command invocations, and single-line status confirmations.\n- Zero narrative text.\n- Core invariant: preserve all code, diffs, tool parameters, and command syntax 100% verbatim.]",
		AgentsLine: "## Operational Mode: Concise Ultra (Level 3)\n- STRICT: Output confined to essential diffs, command invocations, and single-line status confirmations.\n- ZERO narrative prose, conversational text, explanations, or filler.\n- Core invariant: preserve all code, diffs, tool parameters, and command syntax 100% verbatim.\n- Reference: @docs/practices/ConciseMode.md",
	},
}

// ParseTier resolves user input strings/numbers into a canonical Tier.
func ParseTier(input string) (Tier, error) {
	norm := strings.ToLower(strings.TrimSpace(input))
	switch norm {
	case "lite", "1", "level1", "l1":
		return TierLite, nil
	case "std", "standard", "2", "level2", "l2":
		return TierStandard, nil
	case "ultra", "3", "level3", "l3":
		return TierUltra, nil
	case "off", "reset", "default", "none", "0":
		return TierOff, nil
	default:
		return "", fmt.Errorf("unknown mode %q: expected lite (1), std (2), ultra (3), or off", input)
	}
}

// GetTierInfo returns the TierInfo descriptor for a given tier.
func GetTierInfo(t Tier) TierInfo {
	if info, ok := tiers[t]; ok {
		return info
	}
	return tiers[TierOff]
}

// Options configure the SetMode execution.
type Options struct {
	FilePath string
	Quiet    bool
	DryRun   bool
}

// Result describes the outcome of SetMode.
type Result struct {
	Tier       Tier
	Directive  string
	FileUpdate string
	Changed    bool
}

// SetMode applies the chosen tier: generating the runtime directive and synchronizing AGENTS.md.
func SetMode(tier Tier, opts Options) (Result, error) {
	info := GetTierInfo(tier)
	res := Result{
		Tier:      tier,
		Directive: info.Directive,
	}

	targetFile := opts.FilePath
	if targetFile == "" {
		targetFile = "./AGENTS.md"
	}

	if opts.DryRun {
		if tier == TierOff {
			res.FileUpdate = fmt.Sprintf("would remove %q section from %s", ConciseModeSection, targetFile)
		} else {
			res.FileUpdate = fmt.Sprintf("would write %s to %s [%s]", info.Name, targetFile, ConciseModeSection)
		}
		return res, nil
	}

	if tier == TierOff {
		removed, cleaned, err := markdown.Clean(targetFile, ConciseModeSection)
		if err != nil && !os.IsNotExist(err) {
			return res, fmt.Errorf("clean %s in %s: %w", ConciseModeSection, targetFile, err)
		}
		if removed {
			res.FileUpdate = fmt.Sprintf("removed %s (file was empty)", targetFile)
			res.Changed = true
		} else if cleaned {
			res.FileUpdate = fmt.Sprintf("removed %q section from %s", ConciseModeSection, targetFile)
			res.Changed = true
		} else {
			res.FileUpdate = fmt.Sprintf("no %q section in %s", ConciseModeSection, targetFile)
		}
		return res, nil
	}

	changed, existed, err := markdown.Apply(targetFile, ConciseModeSection, info.AgentsLine+"\n")
	if err != nil {
		return res, fmt.Errorf("apply %s in %s: %w", ConciseModeSection, targetFile, err)
	}
	res.Changed = changed
	if changed {
		if existed {
			res.FileUpdate = fmt.Sprintf("updated %q section in %s to %s", ConciseModeSection, targetFile, info.Name)
		} else {
			res.FileUpdate = fmt.Sprintf("added %q section to %s (%s)", ConciseModeSection, targetFile, info.Name)
		}
	} else {
		res.FileUpdate = fmt.Sprintf("%s already configured with %s", targetFile, info.Name)
	}

	return res, nil
}

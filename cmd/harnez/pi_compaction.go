package main

import (
	"fmt"
	"io"

	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/picompact"
	"ubunatic.com/harnez/internal/subagent"
)

// piCompactionParams loads the issue 1023 small-window values from spec/agent.yaml.
func piCompactionParams() (picompact.Params, error) {
	spec, err := subagent.PiCompaction()
	if err != nil {
		return picompact.Params{}, err
	}
	return picompact.Params{
		Threshold:       spec.SmallWindowThresholdTokens,
		ReserveFraction: spec.ReserveFraction,
		KeepFraction:    spec.KeepRecentFraction,
	}, nil
}

// piAgentDir is Pi's agent dir as `harnez apply` sees it, or "" when the config is unavailable.
func piAgentDir() string {
	cfg, err := claude.LoadConfigEmbedded()
	if err != nil {
		return ""
	}
	return claude.PiAgentRoot(cfg)
}

// warnSmallPiWindow prints one warning when a Pi-driven model has a small
// window and no compaction override; the fix is `harnez apply` (issue 1023).
// Any lookup failure stays silent: the warning must never block a start.
func warnSmallPiWindow(warn io.Writer, m subagent.Model, dir string) {
	if m.Provider != "pi" && m.Provider != "lmcoder" {
		return
	}
	params, err := piCompactionParams()
	if err != nil {
		return
	}
	if key, window, missing := picompact.MissingOverride(dir, m.Provider, m.Name, params); missing {
		fmt.Fprintf(warn, "Warning: Pi model %s has a %d-token context window and no compaction override; run 'harnez apply' to add one.\n", key, window)
	}
}

// applyPiCompaction writes Pi compaction overrides for small-window models during `harnez apply`.
func applyPiCompaction(dir string, warn io.Writer) error {
	params, err := piCompactionParams()
	if err != nil {
		return fmt.Errorf("Pi compaction: %w", err)
	}
	return picompact.Sync(dir, params, warn)
}

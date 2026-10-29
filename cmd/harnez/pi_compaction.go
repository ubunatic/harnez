package main

import (
	"fmt"
	"io"

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

// applyPiCompaction writes Pi compaction overrides for small-window models during `harnez apply`.
func applyPiCompaction(dir string, warn io.Writer) error {
	params, err := piCompactionParams()
	if err != nil {
		return fmt.Errorf("Pi compaction: %w", err)
	}
	return picompact.Sync(dir, params, warn)
}

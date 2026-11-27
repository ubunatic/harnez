package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/subagent"
)

const autocompactFlagUsage = "compaction window for this run: auto (configured threshold), a token count, or a count with k (120k); claude and codex only"

// autocompactFlag reads --autocompact from cmd. set is false when the flag was
// not passed, so a resume keeps the persisted value; tokens is 0 for auto.
func autocompactFlag(cmd *cobra.Command, value string) (set bool, tokens int, err error) {
	if !cmd.Flags().Changed("autocompact") {
		return false, 0, nil
	}
	tokens, err = subagent.ParseAutocompact(value)
	if err != nil {
		return false, 0, err
	}
	return true, tokens, nil
}

// checkAutocompactProvider rejects --autocompact for providers whose CLI has
// no per-run compaction window harnez can set. A zero tokens value (auto)
// is accepted for the same providers because it also clears a stored value.
func checkAutocompactProvider(provider string) error {
	switch provider {
	case "claude", "codex":
		return nil
	}
	return fmt.Errorf("--autocompact is supported for claude and codex only, not %q", provider)
}

// sessionModel is the stored session's model, carrying its persisted
// compaction override so the watchdog and compaction use the same threshold.
func sessionModel(sess *subagent.Session) subagent.Model {
	return subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier, CompactTokens: sess.CompactTokens}
}

package main

import (
	"strings"

	"github.com/spf13/cobra"
)

// rewritePluralCommands maps a plural command word ("channels") to its singular
// command ("channel") before Cobra parses argv, so plurals work without showing
// up as aliases in help. Only the leading command path is rewritten, an existing
// command or alias always wins (skills stays skills), and the walk stops at the
// first flag or unknown word.
func rewritePluralCommands(root *cobra.Command, argv []string) []string {
	out := append([]string(nil), argv...)
	cmd := root
	for i, word := range out {
		if strings.HasPrefix(word, "-") {
			break
		}
		next := findChild(cmd, word)
		if next == nil {
			singular, ok := strings.CutSuffix(word, "s")
			if !ok || singular == "" {
				break
			}
			if next = findChild(cmd, singular); next == nil {
				break
			}
			out[i] = next.Name()
		}
		cmd = next
	}
	return out
}

// findChild prefers a command name over an alias, as Cobra does
// (dochistory aliases repo-history, which is also a command).
func findChild(cmd *cobra.Command, word string) *cobra.Command {
	var aliased *cobra.Command
	for _, child := range cmd.Commands() {
		if child.Name() == word {
			return child
		}
		if aliased == nil && child.HasAlias(word) {
			aliased = child
		}
	}
	return aliased
}

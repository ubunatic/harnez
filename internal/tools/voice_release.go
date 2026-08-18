//go:build !debug

package tools

import "github.com/spf13/cobra"

func addDebugVoiceCommands(cmd *cobra.Command, d Dependencies) {
	// Debug canary and VAD probe commands are excluded in release builds.
}

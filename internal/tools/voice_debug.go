//go:build debug

package tools

import "github.com/spf13/cobra"

func addDebugVoiceCommands(cmd *cobra.Command, d Dependencies) {
	cmd.AddCommand(NewVoiceInputCanaryCommand(d), NewVoiceInputVADProbeCommand(d))
}

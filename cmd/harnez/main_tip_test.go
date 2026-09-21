package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestUnderAgentCommand(t *testing.T) {
	root := &cobra.Command{Use: "harnez"}
	agent := &cobra.Command{Use: "agent"}
	start := &cobra.Command{Use: "start"}
	other := &cobra.Command{Use: "issues"}
	nested := &cobra.Command{Use: "agent"} // an unrelated nested "agent" must not match
	root.AddCommand(agent, other)
	agent.AddCommand(start)
	other.AddCommand(nested)
	for cmd, want := range map[*cobra.Command]bool{agent: true, start: true, other: false, nested: false, root: false} {
		if got := underAgentCommand(cmd); got != want {
			t.Fatalf("underAgentCommand(%s) = %v, want %v", cmd.CommandPath(), got, want)
		}
	}
}

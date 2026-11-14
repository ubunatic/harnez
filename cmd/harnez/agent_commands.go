package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/transcript"
)

func (ac *agentCLI) commandsCmd() *cobra.Command {
	return &cobra.Command{Use: "commands [<id-or-path>]", Short: "Print shell commands from a session transcript", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if ac.name != "" && len(args) != 0 {
			return fmt.Errorf("commands: use either an ID or --name")
		}
		id := ac.name
		if len(args) != 0 {
			id = args[0]
		}
		if id == "" {
			return fmt.Errorf("commands: specify a session ID, path, or --name")
		}
		provider := ""
		_, statErr := os.Stat(id)
		isPath := ac.name == "" && (statErr == nil || strings.ContainsAny(id, `/\`) || strings.HasSuffix(id, ".jsonl"))
		if !isPath {
			store, err := subagent.OpenSessionStore(ac.storeDir)
			if err != nil {
				return err
			}
			dir := ""
			if cmd.Flags().Changed("dir") {
				dir = ac.workDir
				if dir == "." {
					dir, err = os.Getwd()
					if err != nil {
						return err
					}
				}
			}
			s, err := resolveSession(store, id, dir)
			if err != nil {
				if ac.name != "" || dir != "" || !strings.Contains(err.Error(), "not found") {
					return err
				}
			} else {
				id, provider = s.ProviderID(), s.Provider
				if provider != "claude" && provider != "codex" {
					return fmt.Errorf("commands: unsupported provider %q", provider)
				}
			}
		} else if cmd.Flags().Changed("dir") {
			return fmt.Errorf("commands: --dir requires a managed session")
		}
		path, err := transcript.ResolveCommands(id, provider)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return transcript.Commands(f, func(s string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), strings.ReplaceAll(strings.ReplaceAll(s, "\r", `\r`), "\n", `\n`))
			return err
		})
	}}
}

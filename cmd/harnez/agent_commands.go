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
		if _, err := os.Stat(id); err != nil && !strings.ContainsAny(id, `/\`) && !strings.HasSuffix(id, ".jsonl") {
			store, err := subagent.OpenSessionStore(ac.storeDir)
			if err != nil {
				return err
			}
			sessions, err := store.List("", true)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			var matches []*subagent.Session
			for _, s := range sessions {
				if s.ID == id {
					matches = []*subagent.Session{s}
					break
				}
			}
			if len(matches) == 0 {
				for _, s := range sessions {
					if s.Name == id {
						matches = append(matches, s)
					}
				}
			}
			if len(matches) == 0 && ac.name == "" {
				for _, s := range sessions {
					if !strings.HasPrefix(s.ID, id) {
						continue
					}
					matches = append(matches, s)
				}
			}
			if len(matches) > 1 {
				return fmt.Errorf("commands: ambiguous session %q", id)
			}
			if len(matches) == 1 {
				s := matches[0]
				if cmd.Flags().Changed("dir") && ac.workDir != "." {
					if _, err := resolveSession(store, s.ID, ac.workDir); err != nil {
						return err
					}
				}
				id, provider = s.ProviderID(), s.Provider
				if provider != "claude" && provider != "codex" {
					return fmt.Errorf("commands: unsupported provider %q", provider)
				}
			} else if ac.name != "" {
				return fmt.Errorf("commands: session %q not found", id)
			}
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

package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/modelblock"
	"ubunatic.com/harnez/internal/subagent"
)

// resolveBlockTarget maps <spec|provider:model> to the provider and model
// name that quota availability is keyed by.
func resolveBlockTarget(arg string, entries []subagent.ModelEntry) (provider, model string, err error) {
	for _, e := range entries {
		if e.Spec == arg || e.Model.Provider+":"+e.Model.Name == arg || strings.HasPrefix(e.Spec, arg+":") {
			return e.Model.Provider, e.Model.Name, nil
		}
	}
	return "", "", fmt.Errorf("unknown model %q; see `harnez agent models --names`", arg)
}

func runModelsBlock(out io.Writer, path, target, until, reason string, now time.Time) error {
	entries, err := subagent.KnownModelEntriesE()
	if err != nil {
		return err
	}
	provider, model, err := resolveBlockTarget(target, entries)
	if err != nil {
		return err
	}
	end, err := modelblock.ParseUntil(until, now)
	if err != nil {
		return err
	}
	if !end.After(now) {
		return fmt.Errorf("--until %s is not in the future", end.Format(time.RFC3339))
	}
	if err := modelblock.Add(path, modelblock.Block{Provider: provider, Model: model, Until: end, Reason: reason, CreatedAt: now}, now); err != nil {
		return err
	}
	fmt.Fprintf(out, "blocked %s:%s until %s\n", provider, model, end.Format(time.RFC3339))
	return nil
}

func runModelsUnblock(out io.Writer, path, target string) error {
	entries, err := subagent.KnownModelEntriesE()
	if err != nil {
		return err
	}
	provider, model, err := resolveBlockTarget(target, entries)
	if err != nil {
		return err
	}
	removed, err := modelblock.Remove(path, provider, model)
	if err != nil {
		return err
	}
	if !removed {
		fmt.Fprintf(out, "%s:%s was not blocked\n", provider, model)
		return nil
	}
	fmt.Fprintf(out, "unblocked %s:%s\n", provider, model)
	return nil
}

func modelsBlockCmd() *cobra.Command {
	var until, reason string
	cmd := &cobra.Command{
		Use:   "block <spec|provider:model> --until <RFC3339|duration> [--reason text]",
		Short: "Mark a model out of quota until a time; start and resume refuse it until then",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if until == "" {
				return fmt.Errorf("--until is required (RFC3339 time or a duration like 3h30m)")
			}
			return runModelsBlock(cmd.OutOrStdout(), modelblock.Path(), args[0], until, reason, time.Now())
		},
	}
	cmd.Flags().StringVar(&until, "until", "", "block end: RFC3339 time or duration such as 3h30m")
	cmd.Flags().StringVar(&reason, "reason", "", "why the model is blocked")
	return cmd
}

func modelsUnblockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unblock <spec|provider:model>",
		Short: "Remove a model block",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelsUnblock(cmd.OutOrStdout(), modelblock.Path(), args[0])
		},
	}
}

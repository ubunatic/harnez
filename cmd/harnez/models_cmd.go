package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/subagent"
)

type modelsSetOptions struct {
	until string
	hours float64
	cheap string
	lean  string
	smart string
	hello bool
}

func (ac *agentCLI) modelsSetCmd() *cobra.Command {
	var opts modelsSetOptions
	cmd := &cobra.Command{
		Use:   "set [--until=HH:MM|RFC3339|--hours=<float>] [<cheap>/<lean>/<smart>] [--cheap=<model>] [--lean=<model>] [--smart=<model>] [--hello]",
		Short: "Set temporary time-limited default models for agent effort tiers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelsSet(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), ac.workDir, args, opts)
		},
	}
	cmd.Flags().StringVar(&opts.until, "until", "", "expiry time as HH:MM or RFC3339")
	cmd.Flags().Float64Var(&opts.hours, "hours", 0, "expiry duration in hours")
	cmd.Flags().StringVar(&opts.cheap, "cheap", "", "model for cheap tier")
	cmd.Flags().StringVar(&opts.lean, "lean", "", "model for lean tier")
	cmd.Flags().StringVar(&opts.smart, "smart", "", "model for smart tier")
	cmd.Flags().BoolVar(&opts.hello, "hello", false, "run minimal hello probe in plain mode for each set model")
	return cmd
}

func newModelsCmd() *cobra.Command {
	cli := &agentCLI{}
	cmd := cli.modelsCmd()
	cmd.Use = "models"
	return cmd
}

func parseExpiry(until string, hours float64, now time.Time) (time.Time, error) {
	if until != "" && hours > 0 {
		return time.Time{}, fmt.Errorf("cannot specify both --until and --hours; choose one expiration boundary")
	}
	if until == "" && hours <= 0 {
		return time.Time{}, fmt.Errorf("expiration boundary required: specify --until=HH:MM|RFC3339 or --hours=<float>")
	}
	if hours > 0 {
		d := time.Duration(hours * float64(time.Hour))
		return now.Add(d), nil
	}

	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, until); err == nil {
		if !t.After(now) {
			return time.Time{}, fmt.Errorf("expiry time %s must be in the future", until)
		}
		return t, nil
	}

	// Try HH:MM
	parts := strings.Split(until, ":")
	if len(parts) == 2 {
		h, err1 := strconv.Atoi(parts[0])
		m, err2 := strconv.Atoi(parts[1])
		if err1 == nil && err2 == nil && h >= 0 && h < 24 && m >= 0 && m < 60 {
			target := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
			if !target.After(now) {
				target = target.Add(24 * time.Hour)
			}
			return target, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid --until format %q: expected HH:MM or RFC3339 timestamp", until)
}

func parseTierAssignments(args []string, opts modelsSetOptions) (map[string]string, error) {
	tiers := make(map[string]string)
	if len(args) > 0 {
		pos := args[0]
		parts := strings.Split(pos, "/")
		if len(parts) == 3 {
			if strings.TrimSpace(parts[0]) != "" {
				tiers["cheap"] = strings.TrimSpace(parts[0])
			}
			if strings.TrimSpace(parts[1]) != "" {
				tiers["lean"] = strings.TrimSpace(parts[1])
			}
			if strings.TrimSpace(parts[2]) != "" {
				tiers["smart"] = strings.TrimSpace(parts[2])
			}
		} else if len(parts) == 1 {
			return nil, fmt.Errorf("invalid positional syntax %q: expected <cheap>/<lean>/<smart> or use --cheap, --lean, --smart flags", pos)
		} else {
			return nil, fmt.Errorf("invalid positional syntax %q: expected 3 slash-separated tiers <cheap>/<lean>/<smart>", pos)
		}
	}

	if opts.cheap != "" {
		tiers["cheap"] = strings.TrimSpace(opts.cheap)
	}
	if opts.lean != "" {
		tiers["lean"] = strings.TrimSpace(opts.lean)
	}
	if opts.smart != "" {
		tiers["smart"] = strings.TrimSpace(opts.smart)
	}

	if len(tiers) == 0 {
		return nil, fmt.Errorf("no tier models specified: provide <cheap>/<lean>/<smart> or at least one of --cheap, --lean, --smart")
	}

	// Validate each model spec
	for tier, spec := range tiers {
		if _, err := subagent.ResolveModel(spec); err != nil {
			return nil, fmt.Errorf("invalid model spec for tier %s (%q): %w", tier, spec, err)
		}
	}

	return tiers, nil
}

func runModelsSet(ctx context.Context, out, errOut io.Writer, workDir string, args []string, opts modelsSetOptions) error {
	now := time.Now()
	expiresAt, err := parseExpiry(opts.until, opts.hours, now)
	if err != nil {
		return err
	}

	newTiers, err := parseTierAssignments(args, opts)
	if err != nil {
		return err
	}

	// Merge with existing active overrides if unexpired
	existing, _ := subagent.LoadTierOverrides()
	mergedTiers := make(map[string]string)
	if existing != nil && existing.Tiers != nil {
		for k, v := range existing.Tiers {
			mergedTiers[k] = v
		}
	}
	for k, v := range newTiers {
		mergedTiers[k] = v
	}

	overrides := subagent.TierOverrides{
		ExpiresAt: expiresAt,
		Tiers:     mergedTiers,
	}

	if err := subagent.SaveTierOverrides(overrides); err != nil {
		return fmt.Errorf("save tier overrides: %w", err)
	}

	// Print confirmation
	fmt.Fprintf(out, "Set tier model defaults (expires in %s at %s):\n",
		time.Until(expiresAt).Round(time.Second), expiresAt.Format(time.RFC3339))
	tierKeys := []string{"cheap", "lean", "smart"}
	for _, k := range tierKeys {
		if m, ok := mergedTiers[k]; ok {
			fmt.Fprintf(out, "  %s: %s\n", k, m)
		}
	}

	if opts.hello {
		runHelloProbes(ctx, out, errOut, workDir, newTiers)
	}

	return nil
}

func runHelloProbes(ctx context.Context, out, errOut io.Writer, workDir string, tiers map[string]string) {
	fmt.Fprintln(out, "\nRunning hello probes (plain mode, lowest effort):")
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "TIER\tMODEL\tSTATUS\tDETAILS")

	keys := make([]string, 0, len(tiers))
	for k := range tiers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, tier := range keys {
		spec := tiers[tier]
		resolved, err := subagent.ResolveModel(spec)
		if err != nil {
			fmt.Fprintf(tw, "%s\t%s\tfail\t%v\n", tier, spec, err)
			continue
		}

		// Ensure lowest effort
		probeModel := resolved
		if probeModel.SupportsEffort() {
			probeModel.Tier = "low"
		}

		contextMode := ""
		if subagent.SupportsContextMode(probeModel.Provider) {
			contextMode = "plain"
		}
		driver := agentDriver(probeModel, workDir)
		res, err := driver.Run(ctx, subagent.RunOptions{
			Prompt:      "hello",
			Model:       probeModel,
			Dir:         workDir,
			ContextMode: contextMode,
		})
		if err != nil {
			fmt.Fprintf(tw, "%s\t%s\tfail\t%v\n", tier, probeModel.Spec(), err)
		} else {
			_ = res
			fmt.Fprintf(tw, "%s\t%s\tok\tready\n", tier, probeModel.Spec())
		}
	}
	_ = tw.Flush()
}

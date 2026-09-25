package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/bench"
)

// benchRunner is swapped in tests so no real agent CLI is invoked.
var benchRunner bench.CommandRunner

func benchDir() (string, error) {
	if d := os.Getenv("HARNEZ_BENCH_DIR"); d != "" {
		return d, nil
	}
	return bench.DefaultDir()
}

func newBenchCmd() *cobra.Command {
	var setup bool
	cmd := &cobra.Command{
		Use:    "bench",
		Short:  "Optional agent benchmark harness (developer tool; needs --setup)",
		Hidden: true,
		Long: `Send small specced tasks to agent CLIs under a documentation condition
(lite/full docs, with or without PNG cards) and record scored results in a bench
database (~/.harnez/bench/bench.sqlite) that is separate from telemetry.

The feature is opt-in: run 'harnez bench --setup' once, then use 'bench run',
'bench tasks' and 'bench results'. Real runs spend agent tokens; defaults use the
cheap models haiku (claude) and gpt-6-luna (codex).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !setup {
				return cmd.Help()
			}
			dir, err := benchDir()
			if err != nil {
				return err
			}
			found, err := bench.Setup(dir, nil)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "bench ready: %s\n", bench.DBPath(dir))
			for _, a := range []string{bench.AgentClaude, bench.AgentCodex, bench.AgentAgy} {
				if p, ok := found[a]; ok {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-6s %s\n", a, p)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-6s not found on PATH\n", a)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&setup, "setup", false, "create the bench database and enable the bench commands")
	cmd.AddCommand(newBenchTasksCmd(), newBenchRunCmd(), newBenchResultsCmd())
	return cmd
}

// openBench gates a subcommand on setup and opens the bench store.
func openBench() (*bench.Store, error) {
	dir, err := benchDir()
	if err != nil {
		return nil, err
	}
	if err := bench.Ready(dir); err != nil {
		return nil, err
	}
	return bench.OpenStore(bench.DBPath(dir))
}

func newBenchTasksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tasks",
		Short: "List the specced bench tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			spec, err := bench.LoadSpec()
			if err != nil {
				return err
			}
			for _, t := range spec.Tasks {
				fmt.Fprintf(cmd.OutOrStdout(), "%-28s %s\n", t.ID, t.Rule)
			}
			return nil
		},
	}
}

func newBenchRunCmd() *cobra.Command {
	var agent, model, docs, repo, readModeName, cardFlags string
	var docCards, fixtureYAML bool
	var multi int
	var repeat int
	var tasks []string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run bench tasks against one agent under one doc condition",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := openBench()
			if err != nil {
				return err
			}
			defer store.Close()
			mode, err := bench.ParseDocs(docs)
			if err != nil {
				return err
			}
			spec, err := bench.LoadSpec()
			if err != nil {
				return err
			}
			readMode, err := bench.ParseRead(readModeName)
			if err != nil {
				return err
			}
			if (fixtureYAML || multi != 0) && readMode == "" {
				return fmt.Errorf("bench: --yaml and --multi need --read")
			}
			if multi < 0 || multi > bench.MaxMulti {
				return fmt.Errorf("bench: --multi must be 1-%d", bench.MaxMulti)
			}
			if cardFlags != "" && readMode != "auto" {
				return fmt.Errorf("bench: --card needs --read auto")
			}
			cond := bench.Condition{Docs: mode, Cards: docCards, Read: readMode, Yaml: fixtureYAML, Multi: multi, Card: strings.TrimSpace(cardFlags)}
			selected, err := spec.SelectFor(tasks, cond)
			if err != nil {
				return err
			}
			opts := bench.Options{Agent: agent, Model: model, Cond: cond, RepoRoot: repo, Repeat: repeat, Run: benchRunner}
			out := cmd.OutOrStdout()
			return bench.RunTasks(cmd.Context(), store, spec, selected, opts, func(r bench.Run) {
				status := "PASS"
				switch {
				case r.Error != "":
					status = "ERROR " + truncateBench(r.Error, 80)
				case !r.Pass:
					status = "FAIL " + r.Detail
				}
				fmt.Fprintf(out, "%-28s %-6s %-14s %-10s in=%d out=%d total=%d turns=%d %s\n", r.Task, r.Agent, r.Model, opts.Cond.Label(), r.InputTokens, r.OutputTokens, r.TotalTokens, r.Turns, status)
			})
		},
	}
	cmd.Flags().StringVar(&agent, "agent", bench.AgentClaude, "agent CLI: claude, codex or agy")
	cmd.Flags().StringVar(&model, "model", "", "model (default: haiku for claude, gpt-6-luna for codex, gemini-3.7-flash at low effort for agy; 'luna', 'flash' and 'flash37' are aliases)")
	cmd.Flags().StringVar(&docs, "docs", "full", "doc variant: full or lite")
	cmd.Flags().BoolVar(&docCards, "cards", false, "deliver docs as PNG context cards instead of Markdown")
	cmd.Flags().StringVar(&readModeName, "read", "", "run fixture read tasks: native, text, auto, or card (forced PNG)")
	cmd.Flags().BoolVar(&fixtureYAML, "yaml", false, "with --read: deliver the fixture as one YAML file instead of Markdown")
	cmd.Flags().IntVar(&multi, "multi", 0, "with --read: split the fixture into N files by first letter (26/N letters each); bare --multi means 5, use --multi=N otherwise")
	cmd.Flags().Lookup("multi").NoOptDefVal = "5"
	cmd.Flags().StringVar(&cardFlags, "card", "", "with --read auto: card flags the agent is told to add to harnez read, e.g. --card=--style=compact")
	cmd.Flags().StringSliceVar(&tasks, "task", nil, "task IDs to run (default: all)")
	cmd.Flags().IntVar(&repeat, "repeat", 1, "runs per task")
	cmd.Flags().StringVar(&repo, "repo", ".", "repository root holding the docs")
	return cmd
}

func newBenchResultsCmd() *cobra.Command {
	var recent int
	cmd := &cobra.Command{
		Use:   "results",
		Short: "Summarize recorded bench runs per condition",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := openBench()
			if err != nil {
				return err
			}
			defer store.Close()
			sums, err := store.Summaries()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%-7s %-14s %-5s %-6s %-30s %5s %5s %9s %8s %9s %6s %9s %4s\n", "agent", "model", "docs", "cards", "read", "runs", "pass", "avg_in", "avg_out", "avg_total", "turns", "avg_usd", "err")
			for _, s := range sums {
				fmt.Fprintf(out, "%-7s %-14s %-5s %-6v %-30s %5d %5d %9.0f %8.0f %9.0f %6.1f %9.4f %4d\n", s.Agent, s.Model, s.Docs, s.Cards, s.Read, s.Runs, s.Passes, s.AvgInput, s.AvgOut, s.AvgTotal, s.AvgTurns, s.AvgCostUSD, s.Errors)
			}
			if recent > 0 {
				runs, err := store.Recent(recent)
				if err != nil {
					return err
				}
				for _, r := range runs {
					fmt.Fprintf(out, "#%d %s %s/%s %s cards=%v pass=%v session=%s %s\n", r.ID, r.Task, r.Agent, r.Model, r.Docs, r.Cards, r.Pass, r.SessionID, strings.TrimSpace(r.Detail+" "+r.Error))
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&recent, "recent", 0, "also list the newest N runs")
	return cmd
}

func truncateBench(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

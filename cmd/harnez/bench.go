package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/bench"
	"ubunatic.com/harnez/internal/readcard"
	"ubunatic.com/harnez/internal/subagent"
)

// benchRunner is swapped in tests so no real agent CLI is invoked.
var benchRunner bench.CommandRunner

var benchCardRenderer = renderBenchCard

func renderBenchCard(source, output string) (string, error) {
	res, err := readcard.ReadFile(source, readcard.TextOptions{})
	if err != nil {
		return "", err
	}
	rendered, err := readcard.RenderFileToCards(res.Lines, source, readcard.RenderOptions{
		ShowLineNumbers: true, OutputPath: output, Title: filepath.Base(source),
		SourceLines: res.SourceLines, StartLine: res.StartLine, SourceTokens: res.TokenStats.TextTokens,
	})
	if err != nil {
		return "", err
	}
	var cards strings.Builder
	for i, path := range rendered.Files {
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		config, decodeErr := png.DecodeConfig(f)
		closeErr := f.Close()
		if decodeErr != nil {
			return "", decodeErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if i > 0 {
			cards.WriteString("\n")
		}
		fmt.Fprintf(&cards, "card: %s (%d bytes, %dx%d px)", path, info.Size(), config.Width, config.Height)
	}
	return cards.String(), nil
}

func benchPreamble(spec *bench.Spec, task bench.Task, cond bench.Condition, root string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s) | read: %s\n", task.ID, task.Rule, cond.ReadVariant())
	if cond.Read == "" {
		fmt.Fprintf(&b, "question: %s\n", task.Prompt)
	} else {
		dir, err := os.MkdirTemp("", "harnez-bench-preamble.*")
		if err != nil {
			return "", err
		}
		fixtures, err := bench.StageWorkspace(dir, root, spec, task, cond)
		if err != nil {
			_ = os.RemoveAll(dir)
			return "", err
		}
		for _, fixture := range fixtures {
			path := filepath.Join(dir, filepath.FromSlash(fixture))
			data, err := os.ReadFile(path)
			if err != nil {
				_ = os.RemoveAll(dir)
				return "", err
			}
			lines := strings.Count(string(data), "\n")
			if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
				lines++
			}
			fmt.Fprintf(&b, "fixture: %s (%d lines)\n", fixture, lines)
			if cond.Read == "card" {
				cardsDir, err := os.MkdirTemp("", "harnez-bench-cards.*")
				if err != nil {
					_ = os.RemoveAll(dir)
					return "", err
				}
				cardPath := filepath.Join(cardsDir, strings.TrimSuffix(filepath.Base(fixture), filepath.Ext(fixture))+".png")
				cardInfo, err := benchCardRenderer(path, cardPath)
				if err != nil {
					_ = os.RemoveAll(dir)
					return "", fmt.Errorf("bench: preamble card %s: %w", fixture, err)
				}
				fmt.Fprintf(&b, "%s\n", cardInfo)
			}
		}
		_ = os.RemoveAll(dir)
		fmt.Fprintf(&b, "question: %s\n", bench.TaskPrompt(task, cond.Read))
	}
	if task.Info != "" {
		fmt.Fprintf(&b, "info: %s\n", task.Info)
	}
	return b.String(), nil
}

func compactBenchTokens(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%.1fk", float64(n)/1000)
}

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
	var docCards, fixtureYAML, quiet bool
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
			readModes, err := bench.ParseReadList(readModeName)
			if err != nil {
				return err
			}
			if (fixtureYAML || multi != 0) && len(readModes) == 0 {
				return fmt.Errorf("bench: --yaml and --multi need --read")
			}
			if multi < 0 || multi > bench.MaxMulti {
				return fmt.Errorf("bench: --multi must be 1-%d", bench.MaxMulti)
			}
			if cardFlags != "" && (len(readModes) != 1 || readModes[0] != "auto") {
				return fmt.Errorf("bench: --card needs --read auto")
			}
			models, err := bench.ParseModels(model)
			if err != nil {
				return err
			}
			if len(models) == 0 {
				defaultModel, err := bench.DefaultModelForProvider(agent)
				if err != nil {
					return err
				}
				models, err = bench.ParseModels(defaultModel.Spec())
				if err != nil {
					return err
				}
			}
			if len(readModes) == 0 {
				readModes = []string{""}
			}
			out := cmd.OutOrStdout()
			errOut := cmd.ErrOrStderr()
			var matrix []bench.Run
			type benchPlan struct {
				model    subagent.Model
				cond     bench.Condition
				selected []bench.Task
			}
			var plans []benchPlan
			total := 0
			preambled := map[string]bool{}
			for _, m := range models {
				for _, readMode := range readModes {
					cond := bench.Condition{Docs: mode, Cards: docCards, Read: readMode, Yaml: fixtureYAML, Multi: multi, Card: strings.TrimSpace(cardFlags)}
					selected, err := spec.SelectFor(tasks, cond)
					if err != nil {
						return err
					}
					for _, task := range selected {
						key := task.ID + "\x00" + cond.ReadVariant() + "\x00" + cond.Docs + fmt.Sprint(cond.Cards)
						if !quiet && !preambled[key] {
							preamble, err := benchPreamble(spec, task, cond, repo)
							if err != nil {
								return err
							}
							fmt.Fprint(errOut, preamble)
							preambled[key] = true
						}
					}
					plans = append(plans, benchPlan{model: m, cond: cond, selected: selected})
					total += len(selected) * repeat
				}
			}
			current := 0
			for _, plan := range plans {
				opts := bench.Options{Agent: plan.model.Provider, Model: plan.model.Spec(), Cond: plan.cond, RepoRoot: repo, Repeat: repeat, Run: benchRunner}
				if !quiet {
					opts.OnStart = func(r bench.Run) {
						current++
						read := r.ReadMode
						if read == "" {
							read = "docs"
						}
						fmt.Fprintf(errOut, "[%d/%d] %s:%s %s %s ...\n", current, total, r.Agent, strings.TrimPrefix(r.Model, r.Agent+":"), read, r.Task)
					}
				}
				if err := bench.RunTasks(cmd.Context(), store, spec, plan.selected, opts, func(r bench.Run) {
					matrix = append(matrix, r)
					if !quiet {
						result := "pass"
						if r.Error != "" {
							result = r.Error
						} else if !r.Pass {
							result = "fail: " + r.Detail
						}
						fmt.Fprintf(errOut, "%s, %s in, %d turns, %s\n", result, compactBenchTokens(r.InputTokens), r.Turns, time.Duration(r.DurationMS)*time.Millisecond)
					}
				}); err != nil {
					return err
				}
			}
			fmt.Fprintf(out, "%-24s %-8s %-24s %-6s %10s %7s %13s %9s  %s\n", "model", "read", "task", "pass", "input", "turns", "helper", "duration", "reason")
			for _, r := range matrix {
				helpers := int64(0)
				for _, h := range r.HelperUsage {
					helpers += h.InputTokens
				}
				read := r.ReadMode
				if read == "" {
					read = "docs"
				}
				status := "FAIL"
				if r.Pass {
					status = "PASS"
				}
				if r.Error != "" {
					status = "ERROR"
				}
				reason := r.Error
				if reason == "" {
					reason = r.Detail
				}
				fmt.Fprintf(out, "%-24s %-8s %-24s %-6s %10d %7d %13d %8.2fs  %s\n", r.Model, read, r.Task, status, r.InputTokens, r.Turns, helpers, float64(r.DurationMS)/1000, reason)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agent, "agent", bench.AgentClaude, "deprecated provider selector (use --model)")
	cmd.Flags().StringVar(&model, "model", "", "comma-separated provider:model:tier specs from 'harnez agent models'")
	cmd.Flags().StringVar(&docs, "docs", "lite", "doc variant: full or lite")
	cmd.Flags().BoolVar(&docCards, "cards", false, "deliver docs as PNG context cards instead of Markdown")
	cmd.Flags().StringVar(&readModeName, "read", "", "comma-separated fixture read modes: native, text, auto, card")
	cmd.Flags().BoolVar(&fixtureYAML, "yaml", false, "with --read: deliver the fixture as one YAML file instead of Markdown")
	cmd.Flags().IntVar(&multi, "multi", 0, "with --read: split the fixture into N files by first letter (26/N letters each); bare --multi means 5, use --multi=N otherwise")
	cmd.Flags().Lookup("multi").NoOptDefVal = "5"
	cmd.Flags().StringVar(&cardFlags, "card", "", "with --read auto: card flags the agent is told to add to harnez read, e.g. --card=--style=compact")
	cmd.Flags().StringSliceVar(&tasks, "task", nil, "task IDs to run (default: all)")
	cmd.Flags().IntVar(&repeat, "repeat", 1, "runs per task")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "suppress preamble and progress on stderr")
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
			fmt.Fprintf(out, "%-7s %-14s %-5s %-6s %-30s %5s %5s %9s %8s %9s %6s %9s %4s %s\n", "agent", "model", "docs", "cards", "read", "runs", "pass", "avg_in", "avg_out", "avg_total", "turns", "avg_usd", "err", "helpers(calls/input/total)")
			for _, s := range sums {
				fmt.Fprintf(out, "%-7s %-14s %-5s %-6v %-30s %5d %5d %9.0f %8.0f %9.0f %6.1f %9.4f %4d %s\n", s.Agent, s.Model, s.Docs, s.Cards, s.Read, s.Runs, s.Passes, s.AvgInput, s.AvgOut, s.AvgTotal, s.AvgTurns, s.AvgCostUSD, s.Errors, s.Helpers)
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

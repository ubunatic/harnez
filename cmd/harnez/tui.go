package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/tui"
	"ubunatic.com/harnez/internal/tui/router"
	"ubunatic.com/loom"
)

type tuiOpts struct {
	model     string
	sessionID string
	render    bool
	cols      int
	rows      int
}

func newTUICmd() *cobra.Command {
	var o tuiOpts
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Interactive Loom-based terminal chat interface for harnez",
		Long: `Interactive multi-model terminal chat interface for harnez.

Built on ubunatic.com/loom, providing a header status line, scrollable chat thread,
and multiline prompt box (Enter to send; Shift+Enter, Ctrl+J or Alt+Enter for a
newline). Shift+Enter and Ctrl+J need a terminal with the kitty keyboard
protocol; elsewhere they send, so use Alt+Enter or end the line with "\".
Ctrl+G edits the prompt in $VISUAL, $EDITOR or the first editor.fallbacks
entry of spec/tui.yaml found on PATH.

Prompts run through the active model's harness CLI (claude, codex, agy, pi), so
turns draw on the matching subscription; the next prompt resumes the same
backend session. Esc or Ctrl+C cancels a running turn. "/model <spec>" switches
the active model (see harnez agent models).

A prompt starting with "!" runs the rest as a background shell command (sh -c,
in the working directory) instead of sending it to the model. Ctrl+B toggles
the job panel: up/down select, Enter shows the live log, x cancels the job
(SIGTERM to its process group, SIGKILL after jobs.cancel_grace_ms), Esc closes.
Quitting cancels all running jobs.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if o.model == "" {
				m, err := subagent.DefaultModelSpec()
				if err != nil {
					return fmt.Errorf("resolve default model: %w", err)
				}
				o.model = m
			}
			resolved, err := router.ResolveSpec(o.model)
			if err != nil {
				return fmt.Errorf("--model: %w", err)
			}
			o.model = resolved
			if o.sessionID == "" {
				o.sessionID = fmt.Sprintf("tui-%s", uuid.New().String()[:8])
			}

			spec, err := tui.LoadSpec()
			if err != nil {
				return fmt.Errorf("load tui spec: %w", err)
			}

			return runTUI(cmd.Context(), cmd.OutOrStdout(), spec, &o)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&o.model, "model", "m", "", "active model for tui chat session (e.g. codex:luna, claude:sonnet)")
	f.StringVarP(&o.sessionID, "session", "s", "", "session identifier for tui chat thread")
	f.BoolVar(&o.render, "render", false, "render single TUI frame to stdout and exit (for non-interactive scripts/tests)")
	f.IntVar(&o.cols, "cols", 80, "column width for --render mode")
	f.IntVar(&o.rows, "rows", 24, "row height for --render mode")
	return cmd
}

func runTUI(_ context.Context, out io.Writer, spec *tui.Spec, o *tuiOpts) error {
	app := tui.NewAppWidget(spec, o.model, o.sessionID, nil)

	f, isFile := out.(*os.File)
	isTTY := isFile && term.IsTerminal(int(f.Fd()))
	if o.render || !isTTY {
		for _, l := range tui.RenderCanvas(app, o.cols, o.rows) {
			fmt.Fprintln(out, l)
		}
		return nil
	}

	rows := o.rows
	if _, r, err := term.GetSize(int(f.Fd())); err == nil && r > 0 {
		rows = r
	}
	pane, err := loom.New(rows)
	if err != nil {
		return fmt.Errorf("open terminal: %w", err)
	}
	defer pane.Close()
	pane.MaxCols = 0 // chat uses the full terminal width, not loom's widget cap
	pane.KittyKeyboard = spec.Terminal.KittyKeyboard
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("working directory: %w", err)
	}
	rt := router.New(dir)
	app.AttachRunner(rt.Turn, pane.Post, router.ResolveSpec)
	app.AttachEditor(pane.Suspend, tui.NewEditor(spec.Editor))
	app.AttachJobs(tui.NewJobManager(spec.Jobs, dir, pane.Post))
	// A running provider turn must not outlive the TUI.
	defer app.Shutdown()
	return pane.Run(app)
}

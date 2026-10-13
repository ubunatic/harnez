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
and multiline prompt box (Enter to send, Ctrl+J for newline).`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if o.model == "" {
				m, err := subagent.DefaultModel()
				if err == nil && m.Name != "" {
					o.model = m.Provider + ":" + m.Name
				} else {
					o.model = "codex:luna"
				}
			}
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
	var app *tui.AppWidget
	app = tui.NewAppWidget(spec, o.model, o.sessionID, func(prompt string) {
		// Mock assistant response for initial interactive harness
		tui.AddTurn(app.Chat, tui.RoleAssistant, o.model, fmt.Sprintf("Received prompt: %s", prompt))
	})

	f, isFile := out.(*os.File)
	isTTY := isFile && term.IsTerminal(int(f.Fd()))

	if o.render || !isTTY {
		cols, rows := o.cols, o.rows
		if isFile && isTTY {
			if c, r, err := term.GetSize(int(f.Fd())); err == nil && c > 0 && r > 0 {
				cols, rows = c, r
			}
		}
		lines := tui.RenderCanvas(app, cols, rows)
		for _, l := range lines {
			fmt.Fprintln(out, l)
		}
		return nil
	}

	pane, err := loom.New(24)
	if err != nil {
		lines := tui.RenderCanvas(app, 80, 24)
		for _, l := range lines {
			fmt.Fprintln(out, l)
		}
		return nil
	}
	defer pane.Close()

	return pane.Run(app)
}

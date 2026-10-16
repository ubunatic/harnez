package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/fsutil"
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
entry of spec/tui.yaml found on PATH. Ctrl+O opens the active model's backend
session in the provider's own CLI (claude --resume, codex resume, agy
--conversation); the tui returns when it exits.

Prompts run through the active model's harness CLI (claude, codex, agy, pi), so
turns draw on the matching subscription; the next prompt resumes the same
backend session natively. Backend session ids are saved per --session id
(sessions.dir of spec/tui.yaml), so "harnez tui --session X" resumes them after
a restart; a session the provider no longer knows starts fresh with a note. The
header shows [resumed] or [new session] for the last turn. Esc or Ctrl+C cancels a running turn. "/model <spec>" switches
the active model (see harnez agent models). Ctrl+T (or Alt+M) or a bare "/model"
opens the model picker: models of spec/agent.yaml by provider, one row per tier,
● marks the active model and ↻ one with a backend session in this tui session.
Type to filter, up/down select, Enter switches, Esc closes; a switch during a
running turn applies to the next turn.

A prompt starting with "!" runs the rest as a background shell command (sh -c,
in the working directory) instead of sending it to the model. Ctrl+B toggles
the job panel: up/down select, Enter shows the live log, x cancels the job
(SIGTERM to its process group, SIGKILL after jobs.cancel_grace_ms), Esc closes.
Quitting cancels all running jobs.

"@" at the start of a word opens a file picker over the repository (git
ls-files, honoring .gitignore); "/" as the first character opens a command
picker with the TUI built-ins and installed skills. Type to filter, up/down
select, Tab or Enter inserts, Esc closes.`,
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
	// Persist backend session ids so --session X resumes them after a restart;
	// without the store the session still works, in memory only.
	storeDir := fsutil.ExpandHome(spec.Sessions.Dir)
	if _, err := router.Prune(storeDir, time.Duration(spec.Sessions.MaxAgeDays)*24*time.Hour, o.sessionID); err != nil {
		tui.AddTurn(app.Chat, tui.RoleNote, "", err.Error())
	}
	if st, err := router.NewStore(storeDir, o.sessionID); err != nil {
		tui.AddTurn(app.Chat, tui.RoleError, "", err.Error()+"; backend sessions are not saved")
	} else {
		rt.Store = st
		if warning, err := rt.Load(); err != nil {
			tui.AddTurn(app.Chat, tui.RoleError, "", err.Error()+"; starting new backend sessions")
		} else if warning != "" {
			tui.AddTurn(app.Chat, tui.RoleNote, "", warning)
		}
	}
	app.AttachRunner(rt.Turn, pane.Post, router.ResolveSpec)
	app.AttachEditor(pane.Suspend, tui.NewEditor(spec.Editor))
	app.AttachNative(pane.Suspend, rt.Native, nil)
	app.AttachSessions(rt.Session)
	app.AttachJobs(tui.NewJobManager(spec.Jobs, dir, pane.Post))
	app.AttachPickers(dir, tuiSkillDirs(dir), pane.Post)
	// A running provider turn must not outlive the TUI.
	defer app.Shutdown()
	return pane.Run(app)
}

// tuiSkillDirs lists the skill dirs the / picker reads: the project's
// .claude/skills, then the agent skill dirs harnez apply installs into.
func tuiSkillDirs(dir string) []string {
	dirs := []string{filepath.Join(dir, ".claude", "skills")}
	cfg, _, err := claude.OpenConfig("")
	if err != nil {
		return dirs
	}
	for _, t := range claude.SkillTargetsByAgent(cfg) {
		dirs = append(dirs, t.Dir)
	}
	return dirs
}

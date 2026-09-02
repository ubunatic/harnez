// feedback implements `harnez feedback`, a low-friction mechanism for an
// agent to durably record something it noticed mid-session — a harnez bug,
// a broken/contradictory instruction, a gap in its own guidance — without
// either losing the observation when the session ends or having to
// hand-author a full issues/NNN-*.md ticket on the spot. See
// issues/184-harnez-feedback-command-for-agent-filed-tickets.md.
package main

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/feedback"
	"ubunatic.com/harnez/internal/resolve"
)

func newFeedbackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feedback",
		Short: "Record and review agent-observed harnez bugs and instruction gaps",
		Long: `feedback gives an agent a single low-friction command to call the moment it
notices something worth tracking — a genuine harnez bug, a broken or
contradictory instruction, a clear gap in its own guidance — without either
losing the observation when the session ends or stopping to hand-author a
full issues/NNN-*.md ticket on the spot.

Entries are appended to a per-project JSONL log under ~/.harnez/feedback/
(never lost, never blocking); ` + "`harnez feedback list`" + ` surfaces unreviewed
entries so the log doesn't silently pile up forgotten. Pass --file-ticket at
log time, or run ` + "`harnez feedback promote <id>`" + ` later, to turn an entry
into a proper tracked issues/NNN-*.md ticket.`,
	}
	cmd.AddCommand(newFeedbackIssueCmd(), newFeedbackListCmd(), newFeedbackPromoteCmd())
	return cmd
}

func newFeedbackIssueCmd() *cobra.Command {
	var severity string
	var dir string
	var fileTicket bool

	cmd := &cobra.Command{
		Use:   `issue "<description>"`,
		Short: "Append one feedback entry describing an observed bug or bad instruction",
		Args:  cobra.ExactArgs(1),
		Long: `issue appends a single free-text feedback entry to this project's
~/.harnez/feedback/ log — a single command, no multi-field form, so it stays
genuinely low-effort to call mid-task.

  harnez feedback issue "<description>" [--severity bug|instruction] [--file-ticket]

Pass --file-ticket to immediately promote the entry into a tracked
issues/NNN-*.md ticket (docs/IssueTracking.md's metadata schema) instead of
leaving it only in the log.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFeedbackIssue(cmd.OutOrStdout(), args[0], severity, dir, fileTicket)
		},
	}
	cmd.Flags().StringVar(&severity, "severity", "", "free-form category, e.g. \"bug\" or \"instruction\" (default: unset)")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "repo root this feedback concerns (also the issues/ root for --file-ticket)")
	cmd.Flags().BoolVar(&fileTicket, "file-ticket", false, "immediately promote this entry to a full issues/NNN-*.md ticket")
	return cmd
}

func runFeedbackIssue(w io.Writer, description, severity, dir string, fileTicket bool) error {
	sessionID, err := resolve.Session(resolve.SessionOptions{})
	if err != nil {
		sessionID = "" // best-effort: an unresolvable session must never block filing feedback
	}

	now := time.Now()
	e := feedback.Entry{
		ID:          feedback.NewID(dir, description, now),
		Time:        now,
		Project:     projectBase(dir),
		SessionID:   sessionID,
		Description: description,
		Severity:    severity,
		Status:      feedback.StatusNew,
	}

	feedbackDir := feedback.DefaultDir()
	if fileTicket {
		issuesDir := filepath.Join(dir, "issues")
		path, err := feedback.Promote(issuesDir, e)
		if err != nil {
			return fmt.Errorf("feedback: %w", err)
		}
		e.Status = feedback.StatusPromoted
		e.TicketPath = path
		if err := feedback.Append(feedbackDir, dir, e); err != nil {
			return fmt.Errorf("feedback: %w", err)
		}
		fmt.Fprintf(w, "filed %s (id %s); run `harnez index -d %s` to add it to issues/README.md\n", path, e.ID, dir)
		return nil
	}

	if err := feedback.Append(feedbackDir, dir, e); err != nil {
		return fmt.Errorf("feedback: %w", err)
	}
	fmt.Fprintf(w, "logged feedback entry %s\n", e.ID)
	return nil
}

func newFeedbackListCmd() *cobra.Command {
	var dir string
	var all bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List this project's unreviewed feedback entries",
		Long: `list prints this project's feedback log so it doesn't silently pile up
unreviewed the way an ignored mechanism does (see issue 181's motivation).
By default only "new" (not yet promoted) entries are shown; pass --all to
include already-promoted entries too.

Output is deterministic, tab-separated, one entry per line, no header:

  ID<TAB>TIME<TAB>STATUS<TAB>SEVERITY<TAB>DESCRIPTION`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFeedbackList(cmd.OutOrStdout(), dir, all)
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "repo root whose feedback log to list")
	cmd.Flags().BoolVar(&all, "all", false, "also include already-promoted entries")
	return cmd
}

func runFeedbackList(w io.Writer, dir string, all bool) error {
	entries, err := feedback.Load(feedback.DefaultDir(), dir)
	if err != nil {
		return fmt.Errorf("feedback: %w", err)
	}
	for _, e := range entries {
		if !all && e.Status != feedback.StatusNew {
			continue
		}
		severity := e.Severity
		if severity == "" {
			severity = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", e.ID, e.Time.Format(time.RFC3339), e.Status, severity, e.Description)
	}
	return nil
}

func newFeedbackPromoteCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "promote <id>",
		Short: "Turn a logged feedback entry into a full issues/NNN-*.md ticket",
		Args:  cobra.ExactArgs(1),
		Long: `promote looks up a previously logged feedback entry by its id (see
` + "`harnez feedback list`" + `) and writes it out as a proper issues/NNN-*.md ticket
using docs/IssueTracking.md's metadata schema, reusing the same ticket
numbering ` + "`harnez find`/`harnez index`" + ` already use. It does not run
` + "`harnez index`" + ` itself — run that afterward to add the new ticket to
issues/README.md.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFeedbackPromote(cmd.OutOrStdout(), dir, args[0])
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "repo root containing issues/ and the feedback log to promote from")
	return cmd
}

func runFeedbackPromote(w io.Writer, dir, id string) error {
	feedbackDir := feedback.DefaultDir()
	entries, err := feedback.Load(feedbackDir, dir)
	if err != nil {
		return fmt.Errorf("feedback: %w", err)
	}

	var found *feedback.Entry
	for i := range entries {
		if entries[i].ID == id {
			found = &entries[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("feedback: no entry with id %q (see `harnez feedback list -d %s --all`)", id, dir)
	}
	if found.Status == feedback.StatusPromoted {
		return fmt.Errorf("feedback: entry %s was already promoted to %s", id, found.TicketPath)
	}

	issuesDir := filepath.Join(dir, "issues")
	path, err := feedback.Promote(issuesDir, *found)
	if err != nil {
		return fmt.Errorf("feedback: %w", err)
	}
	found.Status = feedback.StatusPromoted
	found.TicketPath = path
	if err := feedback.Append(feedbackDir, dir, *found); err != nil {
		return fmt.Errorf("feedback: %w", err)
	}
	fmt.Fprintf(w, "filed %s; run `harnez index -d %s` to add it to issues/README.md\n", path, dir)
	return nil
}

// projectBase renders dir as the display-friendly project label stored on
// each entry (basename only — the log file itself already lives at a
// path-hash-derived location scoped to the full absolute path, so this is
// just for human-readable list output, matching the same coarse
// filepath.Base(wd) convention `harnez rate` uses for ToolCall.ProjectName).
func projectBase(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return filepath.Base(abs)
}

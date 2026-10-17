package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/agentpolicy"
	"ubunatic.com/harnez/internal/subagent"
)

func (ac *agentCLI) listCmd() *cobra.Command {
	list := &cobra.Command{Use: "list", Short: "List agent sessions", RunE: func(cmd *cobra.Command, _ []string) error {
		if ac.children && ac.all {
			return fmt.Errorf("--children cannot be combined with --all-sessions")
		}
		s, err := ac.store()
		if err != nil {
			return err
		}
		p := ac.parent()
		if ac.all {
			p = ""
		}
		xs, err := s.List(p, ac.all)
		if err != nil {
			return err
		}
		if ac.failedOnly {
			filtered := xs[:0]
			for _, x := range xs {
				if x.Status == "delete-failed" {
					filtered = append(filtered, x)
				}
			}
			xs = filtered
		} else {
			filtered := xs[:0]
			for _, x := range xs {
				if x.Status != "delete-failed" {
					filtered = append(filtered, x)
				}
			}
			xs = filtered
		}
		visibleManagedCount := countVisibleAgentSessions(xs, ac.includeDead)
		managed := xs
		if p != "" {
			if every, err := s.List("", true); err == nil {
				managed = every
			}
		}
		var external []*subagent.Session
		if !ac.failedOnly && (!ac.includeDead || ac.all || (p == "" && !ac.children)) {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home directory for external sessions: %w", err)
			}
			if ac.includeDead {
				external, err = subagent.DiscoverExternalSessions(home)
			} else {
				external, err = subagent.DiscoverActiveCodexSessions(home)
			}
			if err != nil {
				return err
			}
			enrichExternalAgentTokenCounts(external)
		}
		var deleted []*subagent.Session
		archivedIDs := make(map[string]struct{})
		if ac.includeArchived {
			deleted, err = s.ListDeleted()
			if err != nil {
				return err
			}
			for _, x := range deleted {
				archivedIDs[x.ID] = struct{}{}
			}
			xs = append(xs, deleted...)
		}
		xs = appendExternalAgentSessions(xs, managed, external, deleted)
		if !ac.includeDead && !ac.failedOnly {
			filtered := xs[:0]
			for _, x := range xs {
				_, archived := archivedIDs[x.ID]
				if x.Status == "running" || x.Status == "active" || archived {
					filtered = append(filtered, x)
				}
			}
			xs = filtered
		}
		if !ac.all && p != "" {
			if every, err := s.List("", true); err == nil {
				totalVisible := countVisibleAgentSessions(every, ac.includeDead)
				if totalVisible > visibleManagedCount {
					fmt.Fprintf(cmd.ErrOrStderr(), "harnez: %d more agents in other sessions; use --all-sessions\n", totalVisible-visibleManagedCount)
				}
			}
		}
		if ac.children && !ac.all { /* List already scopes to direct children. */
		}
		if ac.jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(xs)
		}
		colorEnabled, err := agentListColorEnabled(ac.listColorMode, cmd.OutOrStdout())
		if err != nil {
			return err
		}
		if colorEnabled {
			fmt.Fprintln(cmd.OutOrStdout(), agentListColor("\x1b[1m", "NAME\tPROVIDER\tSTATUS\tRESUME\tCONTEXT_USED\tCONTEXT_SIZE\tSESSION_USED\tWORKING_DIR", true))
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "ID\tNAME\tPROVIDER\tSTATUS\tRESUME\tCONTEXT_USED\tCONTEXT_SIZE\tSESSION_USED\tSOURCE\tWORKING_DIR")
		}
		for _, x := range xs {
			resume := resumeState(x)
			if x.HarnessType == "external" {
				resume = "external"
			}
			if colorEnabled {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", x.Name, x.Provider, agentListStatusColor(x.Status, true), resume, formatAgentContextTokens(x.LastContextTokens()), formatAgentContextTokens(x.ContextWindowSize), formatAgentSessionUsedTokens(x.SessionUsedTokens, x.SessionUsedKnown), x.WorkingDir)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", x.ID, x.Name, x.Provider, x.Status, resume, formatAgentContextTokens(x.LastContextTokens()), formatAgentContextTokens(x.ContextWindowSize), formatAgentSessionUsedTokens(x.SessionUsedTokens, x.SessionUsedKnown), x.Source, x.WorkingDir)
			}
		}
		return nil
	}}
	list.Flags().BoolVar(&ac.children, "children", false, "list child sessions")
	list.Flags().BoolVar(&ac.all, "all-sessions", false, "list all sessions")
	list.Flags().BoolVar(&ac.failedOnly, "failed", false, "list unresolved provider deletions")
	list.Flags().BoolVar(&ac.includeDead, "dead", false, "include inactive and completed sessions")
	list.Flags().BoolVar(&ac.includeArchived, "archived", false, "include deleted archived sessions")
	list.Flags().BoolVar(&ac.jsonOut, "json", false, "JSON output")
	list.Flags().StringVar(&ac.listColorMode, "color", "auto", "color output: auto, always, or never")
	return list
}

func (ac *agentCLI) statusCmd() *cobra.Command {
	status := &cobra.Command{Use: "status", Short: "Show agent session status", Args: noArgs("session is now --name <session>"), RunE: func(cmd *cobra.Command, a []string) error {
		if cmd.Flags().Changed("turn") && ac.statusTurn <= 0 {
			return fmt.Errorf("--turn must be a positive turn number")
		}
		if ac.statusAllReplies && ac.statusTurn != 0 {
			return fmt.Errorf("--all and --turn cannot be used together")
		}
		if ac.statusTurn > 0 || ac.statusAllReplies {
			ac.statusReply = true
		}
		if ac.name == "" && ac.statusReply {
			return fmt.Errorf("--reply, --turn, and --all require --name <session>")
		}
		s, e := ac.store()
		if e != nil {
			return e
		}
		if ac.name == "" {
			return runAgentRepoStatus(cmd, s, ac.workDir, ac.jsonOut)
		}
		x, e := ac.find(cmd, s, ac.name)
		if e != nil {
			return e
		}
		return statusSession(cmd, x, ac.jsonOut, ac.statusReply, ac.statusTurn, ac.statusAllReplies)
	}}
	status.ValidArgsFunction = agentSessionCompletion(ac.storeDir, ac.parent)
	status.Flags().BoolVar(&ac.jsonOut, "json", false, "JSON output")
	status.Flags().BoolVar(&ac.statusReply, "reply", false, "show the latest saved reply")
	status.Flags().IntVar(&ac.statusTurn, "turn", 0, "show the saved reply for turn N")
	status.Flags().BoolVar(&ac.statusAllReplies, "all", false, "show all saved turn replies")
	return status
}

func (ac *agentCLI) waitCmd() *cobra.Command {
	var waitTimeout time.Duration
	wait := &cobra.Command{Use: "wait <session>", Short: "Wait for an agent session to finish", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		s, err := ac.store()
		if err != nil {
			return err
		}
		sess, err := waitForAgent(cmd.Context(), s, args[0], waitTimeout)
		if err != nil {
			return err
		}
		if sess.Status == "running" {
			if ac.jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(sess)
			}
			return fmt.Errorf("timed out waiting for session %q", sess.ID)
		}
		return writeAgentOutput(cmd, ac.jsonOut, agentOutput{Session: sess, Response: sess.Response, Messages: sess.Messages})
	}}
	wait.Flags().DurationVar(&waitTimeout, "timeout", 0, "maximum wait duration (0 waits indefinitely)")
	wait.Flags().BoolVar(&ac.jsonOut, "json", false, "JSON output")
	wait.ValidArgsFunction = agentSessionCompletion(ac.storeDir, ac.parent)
	return wait
}

func (ac *agentCLI) addPolicyCmds(root *cobra.Command) {
	var persist bool
	for _, spec := range []struct {
		name string
		mode string
	}{
		{"enable", "harnez"},
		{"disable", "native"},
	} {
		mode := spec.mode
		policyCmd := &cobra.Command{Use: spec.name, Short: "Set subagent dispatch policy to " + mode, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			path, changed, err := agentpolicy.Configure(ac.workDir, mode, persist)
			if err != nil {
				return err
			}
			if changed {
				fmt.Fprintf(cmd.OutOrStdout(), "  updated %s\n", path)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  unchanged %s\n", path)
			}
			return nil
		}}
		policyCmd.Flags().BoolVar(&persist, "persist", false, "accepted for compatibility; policy stays in .harnez/rules/Local.md")
		root.AddCommand(policyCmd)
	}
}

func (ac *agentCLI) compactCmd() *cobra.Command {
	compact := &cobra.Command{Use: "compact", Short: "Compact an agent session", Args: noArgs("session is now --name <session>"), RunE: func(cmd *cobra.Command, a []string) error {
		if ac.name == "" {
			return fmt.Errorf("compact: --name <session> is required")
		}
		s, e := ac.store()
		if e != nil {
			return e
		}
		x, e := ac.find(cmd, s, ac.name)
		if e != nil {
			return e
		}
		return compactSession(cmd, s, x)
	}}
	compact.ValidArgsFunction = agentSessionCompletion(ac.storeDir, ac.parent)
	return compact
}

func (ac *agentCLI) stopCmd() *cobra.Command {
	stop := &cobra.Command{Use: "stop", Short: "Stop an agent session", Args: noArgs("session is now --name <session>"), RunE: func(cmd *cobra.Command, a []string) error {
		s, e := ac.store()
		if e != nil {
			return e
		}
		if ac.all {
			xs, err := s.List("", true)
			if err != nil {
				return err
			}
			for _, x := range xs {
				if !subagent.CanManage(ac.parent(), x) {
					continue
				}
				if x.HarnessType == "interactive" && x.Status != "active" {
					continue
				}
				if e = stopSession(cmd, s, x); e != nil {
					return e
				}
			}
			return nil
		}
		if ac.children && len(a) == 0 {
			xs, _ := s.List(ac.parent(), false)
			for _, x := range xs {
				if x.HarnessType == "interactive" && x.Status != "active" {
					continue
				}
				if e = stopSession(cmd, s, x); e != nil {
					return e
				}
			}
			return nil
		}
		if ac.name == "" {
			return fmt.Errorf("stop: --name <session> is required")
		}
		x, e := ac.find(cmd, s, ac.name)
		if e != nil {
			return e
		}
		if !canManageTarget(ac.parent(), x) {
			return fmt.Errorf("session %q is outside caller lineage", x.ID)
		}
		return stopSession(cmd, s, x)
	}}
	stop.Flags().BoolVar(&ac.children, "children", false, "stop child sessions")
	stop.Flags().BoolVar(&ac.all, "all", false, "stop all manageable sessions")
	stop.ValidArgsFunction = agentSessionCompletion(ac.storeDir, ac.parent)
	return stop
}

func (ac *agentCLI) deleteCmd() *cobra.Command {
	var allCompleted, force, retryFailed bool
	warnUnrated := func(cmd *cobra.Command, x *subagent.Session) bool {
		return !sessionLatestTurnRated(x)
	}
	remove := &cobra.Command{Use: "delete", Short: "Delete an agent session", Aliases: []string{"rm"}, Args: noArgs("session is now --name <session>"), RunE: func(cmd *cobra.Command, a []string) error {
		s, e := ac.store()
		if e != nil {
			return e
		}
		if retryFailed {
			if ac.all || allCompleted || ac.name != "" {
				return fmt.Errorf("delete --retry-failed cannot be combined with --all, --all-completed, or --name")
			}
			xs, err := s.List("", true)
			if err != nil {
				return err
			}
			var failures []error
			for _, x := range xs {
				if x.Status != "delete-failed" || !subagent.CanManage(ac.parent(), x) {
					continue
				}
				if e = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}, x.WorkingDir).Delete(cmd.Context(), x.ProviderID()); e != nil {
					x.LastError = e.Error()
					if saveErr := s.Save(x); saveErr != nil {
						failures = append(failures, fmt.Errorf("%s: persist updated unresolved deletion: %w", x.Name, saveErr))
					}
					failures = append(failures, fmt.Errorf("%s: %w", x.Name, e))
					continue
				}
				if e = s.Delete(x.ID); e != nil {
					failures = append(failures, fmt.Errorf("%s: %w", x.Name, e))
					continue
				}
				fmt.Fprintln(cmd.OutOrStdout(), x.Name)
			}
			for _, err := range failures {
				fmt.Fprintln(cmd.ErrOrStderr(), "delete failed:", err)
			}
			if len(failures) > 0 {
				return errors.New("some sessions failed to delete")
			}
			return nil
		}
		if allCompleted {
			if ac.all || ac.name != "" {
				return fmt.Errorf("delete --all-completed cannot be combined with --all or --name")
			}
			xs, err := s.List("", true)
			if err != nil {
				return err
			}
			var deleted []string
			var failures []error
			for _, x := range xs {
				if !subagent.CanManage(ac.parent(), x) {
					continue
				}
				if x.Status != "completed" {
					continue
				}
				if warnUnrated(cmd, x) && !force {
					printUnratedDeleteWarning(cmd, []*subagent.Session{x})
					failures = append(failures, fmt.Errorf("%s: refusing to delete unrated session without --force", x.Name))
					continue
				}
				if x.HarnessType == "interactive" {
					if e = s.Delete(x.ID); e != nil {
						failures = append(failures, fmt.Errorf("%s: %w", x.Name, e))
						continue
					}
					deleted = append(deleted, x.Name)
					continue
				}
				if e = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}, x.WorkingDir).Delete(cmd.Context(), x.ProviderID()); e != nil {
					x.Status, x.LastError = "delete-failed", e.Error()
					if saveErr := s.Save(x); saveErr != nil {
						failures = append(failures, fmt.Errorf("%s: persist unresolved deletion: %w", x.Name, saveErr))
					}
					failures = append(failures, fmt.Errorf("%s: %w", x.Name, e))
					continue
				}
				if e = s.Delete(x.ID); e != nil {
					failures = append(failures, fmt.Errorf("%s: %w", x.Name, e))
					continue
				}
				deleted = append(deleted, x.Name)
			}
			for _, name := range deleted {
				fmt.Fprintln(cmd.OutOrStdout(), name)
			}
			for _, err := range failures {
				fmt.Fprintln(cmd.ErrOrStderr(), "delete failed:", err)
			}
			if len(failures) > 0 {
				return errors.New("some sessions failed to delete")
			}
			return nil
		}
		if ac.all {
			xs, err := s.List("", true)
			if err != nil {
				return err
			}
			var failures []error
			var unrated []*subagent.Session
			skippedFailed := 0
			for _, x := range xs {
				if !subagent.CanManage(ac.parent(), x) {
					continue
				}
				if x.Status == "delete-failed" {
					if x.Provider != "codex" {
						skippedFailed++
						continue
					}
				}
				if warnUnrated(cmd, x) {
					if !force {
						unrated = append(unrated, x)
						continue
					}
				}
				if x.HarnessType == "interactive" {
					if x.Status == "active" {
						failures = append(failures, fmt.Errorf("%s: session is active; stop it before deletion", x.Name))
						continue
					}
					if e = s.Delete(x.ID); e != nil {
						failures = append(failures, fmt.Errorf("%s: %w", x.Name, e))
						continue
					}
					fmt.Fprintln(cmd.OutOrStdout(), x.Name)
					continue
				}
				if e = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}, x.WorkingDir).Delete(cmd.Context(), x.ProviderID()); e != nil {
					x.Status, x.LastError = "delete-failed", e.Error()
					if saveErr := s.Save(x); saveErr != nil {
						failures = append(failures, fmt.Errorf("%s: persist unresolved deletion: %w", x.Name, saveErr))
					}
					failures = append(failures, fmt.Errorf("%s: %w", x.Name, e))
					continue
				}
				if e = s.Delete(x.ID); e != nil {
					failures = append(failures, fmt.Errorf("%s: %w", x.Name, e))
					continue
				}
				fmt.Fprintln(cmd.OutOrStdout(), x.Name)
			}
			if len(unrated) > 0 && !force {
				printUnratedDeleteWarning(cmd, unrated)
			}
			if skippedFailed > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "%d unresolved provider deletion(s) skipped; retry with --retry-failed\n", skippedFailed)
			}
			for _, err := range failures {
				fmt.Fprintln(cmd.ErrOrStderr(), "delete failed:", err)
			}
			if len(failures) > 0 {
				return errors.New("some sessions failed to delete")
			}
			return nil
		}
		if ac.name == "" {
			return fmt.Errorf("delete: --name <session> is required")
		}
		x, e := ac.find(cmd, s, ac.name)
		if e != nil {
			return e
		}
		if !canManageTarget(ac.parent(), x) {
			return fmt.Errorf("session %q is outside caller lineage", x.ID)
		}
		if warnUnrated(cmd, x) && !force {
			printUnratedDeleteWarning(cmd, []*subagent.Session{x})
			return fmt.Errorf("delete: refusing to delete unrated session %q without --force", x.Name)
		}
		if x.HarnessType == "interactive" {
			if x.Status == "active" {
				return fmt.Errorf("session %q is active; stop it before deletion", x.Name)
			}
			return s.Delete(x.ID)
		}
		if e = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}, x.WorkingDir).Delete(cmd.Context(), x.ProviderID()); e != nil {
			x.Status, x.LastError = "delete-failed", e.Error()
			if saveErr := s.Save(x); saveErr != nil {
				return fmt.Errorf("%w; failed to persist unresolved deletion: %v", e, saveErr)
			}
			return e
		}
		return s.Delete(x.ID)
	}}
	remove.Flags().BoolVar(&ac.all, "all", false, "delete all manageable sessions")
	remove.Flags().BoolVar(&allCompleted, "all-completed", false, "delete all completed sessions")
	remove.Flags().BoolVar(&force, "force", false, "delete unrated sessions without warning")
	remove.Flags().BoolVar(&retryFailed, "retry-failed", false, "retry unresolved provider deletions")
	remove.ValidArgsFunction = agentSessionCompletion(ac.storeDir, ac.parent)
	return remove
}

func (ac *agentCLI) rateCmd() *cobra.Command {
	rate := &cobra.Command{Use: "rate <1-5> <reason>", Short: "Rate the latest turn of an agent session", Args: cobra.MinimumNArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if ac.name == "" {
			return fmt.Errorf("rate: --name <session> is required")
		}
		s, err := ac.store()
		if err != nil {
			return err
		}
		sess, err := ac.find(cmd, s, ac.name)
		if err != nil {
			return err
		}
		score, err := strconv.Atoi(args[0])
		if err != nil || score < 1 || score > 5 {
			return fmt.Errorf("rate: score must be an integer from 1 to 5")
		}
		if err := rateLatestSessionTurn(s, sess, score, strings.Join(args[1:], " ")); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Rated %s turn %d: %d/5\n", sess.Name, sess.TurnRecords[len(sess.TurnRecords)-1].Turn, score)
		return nil
	}}
	rate.ValidArgsFunction = agentSessionCompletion(ac.storeDir, ac.parent)
	return rate
}

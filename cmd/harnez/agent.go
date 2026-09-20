package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/subagent"
)

var agentDriver = func(m subagent.Model) subagent.Driver {
	if m.Provider == "claude" {
		return subagent.ClaudeDriver{}
	}
	return subagent.CodexDriver{}
}

type agentOutput struct {
	*subagent.Session
	Response     string `json:"response,omitempty"`
	ReconnectCmd string `json:"reconnect_cmd,omitempty"`
}

func newAgentCmd() *cobra.Command {
	var jsonOut, children, all bool
	var storeDir, workDir, name string
	root := &cobra.Command{Use: "agent", Short: "Manage subagent sessions"}
	root.PersistentFlags().StringVar(&storeDir, "store-dir", subagent.DefaultStoreDir(), "session store directory")
	store := func() (*subagent.FileSessionStore, error) { return subagent.NewSessionStore(storeDir) }
	parent := func() string {
		if v := os.Getenv("HARNEZ_SESSION_ID"); v != "" {
			return v
		}
		return os.Getenv("AGY_CONVERSATION_ID")
	}
	find := func(s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
		if x, err := s.Get(id); err == nil {
			return x, nil
		}
		all, err := s.List("", true)
		if err != nil {
			return nil, err
		}
		for _, x := range all {
			if x.Name == id {
				return x, nil
			}
		}
		return nil, fmt.Errorf("session %q not found", id)
	}
	write := func(cmd *cobra.Command, v any) error {
		if jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(v)
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), v)
		return err
	}

	start := &cobra.Command{Use: "start <provider:model[:tier]> <prompt>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		m, err := subagent.ResolveModel(args[0])
		if err != nil {
			return err
		}
		s, err := store()
		if err != nil {
			return err
		}
		id := uuid.NewString()
		sessName := name
		if sessName == "" {
			sessName = "agent-" + m.Provider + "-" + id[:8]
		}
		r, err := agentDriver(m).Run(cmd.Context(), subagent.RunOptions{Prompt: args[1], Model: m, Dir: workDir})
		if err != nil {
			return err
		}
		if r.SessionID != "" {
			id = r.SessionID
		}
		now := time.Now()
		sess := &subagent.Session{ID: id, Name: sessName, Provider: m.Provider, Model: m.Name, Tier: m.Tier, WorkingDir: workDir, ParentSessionID: parent(), CallerPID: os.Getpid(), HarnessType: "harnez", Status: "completed", TokensCumulative: r.TokensCumulative, TokensTurn: r.TokensTurn, CachedTokens: r.CachedTokens, CreatedAt: now, LastActiveAt: now}
		if err := s.Save(sess); err != nil {
			return err
		}
		out := agentOutput{Session: sess, Response: r.Response, ReconnectCmd: "harnez agent resume " + id + " \"<prompt>\""}
		if jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Harnez Agent Started: %s\nReconnect / Resume: %s\n\n%s\n", id, out.ReconnectCmd, r.Response)
		return nil
	}}
	start.Flags().StringVarP(&workDir, "dir", "d", ".", "working directory")
	start.Flags().StringVar(&name, "name", "", "session name")
	start.Flags().BoolVar(&jsonOut, "json", false, "JSON output")

	resume := &cobra.Command{Use: "resume <session> <prompt>", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store()
		if err != nil {
			return err
		}
		sess, err := find(s, args[0])
		if err != nil {
			return err
		}
		if !subagent.CanManage(parent(), sess) {
			return fmt.Errorf("session %q is outside caller lineage", sess.ID)
		}
		d := agentDriver(subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier})
		if subagent.ShouldCompact(sess.TokensCumulative) {
			if _, err = d.Compact(cmd.Context(), sess.ID); err != nil {
				return err
			}
		}
		r, err := d.Resume(cmd.Context(), sess.ID, args[1])
		if err != nil {
			return err
		}
		sess.TokensTurn = r.TokensTurn
		sess.TokensCumulative += r.TokensTurn
		sess.CachedTokens = r.CachedTokens
		sess.LastActiveAt = time.Now()
		if err = s.Save(sess); err != nil {
			return err
		}
		return write(cmd, agentOutput{Session: sess, Response: r.Response})
	}}
	resume.Flags().BoolVar(&jsonOut, "json", false, "JSON output")

	list := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := store()
		if err != nil {
			return err
		}
		p := parent()
		if all {
			p = ""
		}
		xs, err := s.List(p, all)
		if err != nil {
			return err
		}
		if children && !all { /* List already scopes to direct children. */
		}
		if jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(xs)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "ID\tNAME\tPROVIDER\tSTATUS\tTOKENS\tCACHED")
		for _, x := range xs {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%d\t%d\n", x.ID, x.Name, x.Provider, x.Status, x.TokensCumulative, x.CachedTokens)
		}
		return nil
	}}
	list.Flags().BoolVar(&children, "children", false, "list child sessions")
	list.Flags().BoolVar(&all, "all-sessions", false, "list all sessions")
	list.Flags().BoolVar(&jsonOut, "json", false, "JSON output")

	status := &cobra.Command{Use: "status <session>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, a []string) error {
		s, e := store()
		if e != nil {
			return e
		}
		x, e := find(s, a[0])
		if e != nil {
			return e
		}
		if jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(x)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s\nStatus: %s\nProvider: %s:%s\nTokens: %d (turn %d)\nCached: %d\n", x.Name, x.Status, x.Provider, x.Model, x.TokensCumulative, x.TokensTurn, x.CachedTokens)
		return nil
	}}
	status.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	compact := &cobra.Command{Use: "compact <session>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, a []string) error {
		s, e := store()
		if e != nil {
			return e
		}
		x, e := find(s, a[0])
		if e != nil {
			return e
		}
		_, e = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model, Tier: x.Tier}).Compact(cmd.Context(), x.ID)
		return e
	}}
	stop := &cobra.Command{Use: "stop [session]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, a []string) error {
		s, e := store()
		if e != nil {
			return e
		}
		if children && len(a) == 0 {
			xs, _ := s.List(parent(), false)
			for _, x := range xs {
				if e = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}).Stop(cmd.Context(), x.ID); e != nil {
					return e
				}
				x.Status = "stopped"
				_ = s.Save(x)
			}
			return nil
		}
		if len(a) == 0 {
			return fmt.Errorf("session is required")
		}
		x, e := find(s, a[0])
		if e != nil {
			return e
		}
		if !subagent.CanManage(parent(), x) {
			return fmt.Errorf("session %q is outside caller lineage", x.ID)
		}
		e = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}).Stop(cmd.Context(), x.ID)
		x.Status = "stopped"
		if e == nil {
			e = s.Save(x)
		}
		return e
	}}
	stop.Flags().BoolVar(&children, "children", false, "stop child sessions")
	remove := &cobra.Command{Use: "delete <session>", Aliases: []string{"rm"}, Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, a []string) error {
		s, e := store()
		if e != nil {
			return e
		}
		x, e := find(s, a[0])
		if e != nil {
			return e
		}
		if !subagent.CanManage(parent(), x) {
			return fmt.Errorf("session %q is outside caller lineage", x.ID)
		}
		if e = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}).Delete(cmd.Context(), x.ID); e != nil {
			return e
		}
		return s.Delete(x.ID)
	}}
	root.AddCommand(start, resume, list, status, compact, stop, remove)
	return root
}

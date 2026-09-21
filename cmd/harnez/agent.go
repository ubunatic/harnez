package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/agentpolicy"
	"ubunatic.com/harnez/internal/privacy"
	"ubunatic.com/harnez/internal/subagent"
)

var agentDriver = func(m subagent.Model) subagent.Driver {
	switch m.Provider {
	case "claude":
		return subagent.ClaudeDriver{}
	case "codex":
		return subagent.CodexDriver{}
	default:
		return subagent.UnsupportedDriver{Provider: m.Provider}
	}
}

var agentInteractiveRunner subagent.InteractiveRunner = subagent.CLIInteractiveRunner{}

type agentOutput struct {
	*subagent.Session
	Response     string   `json:"response,omitempty"`
	Messages     []string `json:"messages,omitempty"`
	ReconnectCmd string   `json:"reconnect_cmd,omitempty"`
}

func newAgentCmd() *cobra.Command {
	var jsonOut, children, all bool
	var storeDir, workDir, name, modelSpec, streamMode string
	var planFirst bool
	root := &cobra.Command{Use: "agent", Short: "Manage subagent sessions"}
	root.PersistentFlags().StringVar(&storeDir, "store-dir", subagent.DefaultStoreDir(), "session store directory")
	root.PersistentFlags().StringVarP(&workDir, "dir", "d", ".", "working directory or session scope")
	root.PersistentFlags().StringVar(&name, "name", "", "session name")
	root.PersistentFlags().StringVar(&modelSpec, "model", "", "provider:model[:tier]")
	store := func() (*subagent.FileSessionStore, error) { return subagent.NewSessionStore(storeDir) }
	parent := func() string {
		if v := os.Getenv("HARNEZ_SESSION_ID"); v != "" {
			return v
		}
		return os.Getenv("AGY_CONVERSATION_ID")
	}
	_ = root.RegisterFlagCompletionFunc("name", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return agentSessionCompletion(storeDir, parent)(cmd, args, toComplete)
	})
	_ = root.RegisterFlagCompletionFunc("model", agentModelCompletion)
	find := func(cmd *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
		dir := ""
		if cmd.Flags().Changed("dir") {
			dir = workDir
		}
		return resolveSession(s, id, dir)
	}
	write := func(cmd *cobra.Command, v any) error {
		if jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(v)
		}
		if o, ok := v.(agentOutput); ok && o.Session != nil {
			return printAgentMessages(cmd, o.Messages, o.Response)
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), v)
		return err
	}

	var startFiles []string
	start := &cobra.Command{Use: "start [prompt...]", Args: func(*cobra.Command, []string) error { return nil }, RunE: func(cmd *cobra.Command, args []string) error {
		words, tail := promptArgs(args, cmd.Flags().ArgsLenAtDash())
		spec := modelSpec
		if spec == "" && len(words) >= 2 && oldStyleModelWord(words[0]) {
			return fmt.Errorf("model is now --model <spec>; to send this text literally put it after --")
		}
		if spec == "" {
			var defaultErr error
			spec, defaultErr = subagent.DefaultModelSpec()
			if defaultErr != nil {
				return defaultErr
			}
		}
		prompt, err := assemblePrompt(startFiles, words, tail, cmd.InOrStdin())
		if err != nil {
			return err
		}
		if err := checkStreamMode(streamMode); err != nil {
			return err
		}
		m, err := subagent.ResolveModel(spec)
		if err != nil {
			return fmt.Errorf("agent start %q rejected: %w; ask for guidance rather than using a different model", spec, err)
		}
		s, err := store()
		if err != nil {
			return err
		}
		id := uuid.NewString()
		sessName := name
		if sessName == "" {
			sessions, listErr := s.List("", true)
			if listErr != nil {
				return listErr
			}
			taken := make(map[string]bool, len(sessions)*2)
			for _, existing := range sessions {
				taken[existing.Name] = true
				taken[existing.ID] = true
			}
			sessName, err = subagent.GenerateSessionName(func(candidate string) bool { return taken[candidate] })
			if err != nil {
				return err
			}
		}
		if name != "" {
			if existing, findErr := resolveSession(s, name, ""); findErr == nil {
				return fmt.Errorf("session name %q is already in use by %s in %s", name, existing.ID, existing.WorkingDir)
			}
		}
		canonicalWorkDir, err := filepath.Abs(workDir)
		if err != nil {
			return fmt.Errorf("resolve working directory: %w", err)
		}
		tl := newTimeline(cmd)
		opts := subagent.RunOptions{Prompt: prompt, Model: m, Dir: canonicalWorkDir}
		d := agentDriver(m)
		sd, streaming := d.(subagent.StreamingDriver)
		streaming = streaming && !jsonOut
		var ts *turnStream
		var r *subagent.TurnResult
		if streaming {
			ts = newTurnStream(cmd, streamMode, false)
			ts.stopCmd, ts.name, ts.planFirst = "harnez agent stop "+sessName, sessName, planFirst
			opts.Prompt = withProtocol(prompt, planFirst)
			ts.watch()
			r, err = sd.RunStream(cmd.Context(), opts, func(ev subagent.Event) {
				if ev.Kind == "session" {
					extra := []string{fmt.Sprintf("name=%s dir=%s", sessName, canonicalWorkDir)}
					if modelSpec == "" {
						extra = append(extra, "model: "+spec+" (default)")
					}
					ts.info(ev.Text, m.Provider+":"+m.Name, "start", "new", append(extra, "reconnect: harnez agent resume "+ev.Text+" \"<prompt>\"")...)
				}
				ts.onEvent(ev)
			})
			if err != nil {
				ts.abort()
			}
		} else {
			tl.announceTurn("start", m.Provider+":"+m.Name, sessName)
			r, err = d.Run(cmd.Context(), opts)
		}
		if err != nil {
			return fmt.Errorf("agent start %q failed: %w; verify the provider/model configuration or ask for guidance", spec, err)
		}
		if r.SessionID != "" {
			id = r.SessionID
		}
		now := time.Now()
		sess := &subagent.Session{ID: id, Name: sessName, StartPrompt: promptStorage(startFiles, words, tail, prompt), Provider: m.Provider, Model: m.Name, Tier: m.Tier, WorkingDir: canonicalWorkDir, ParentSessionID: parent(), CallerPID: os.Getpid(), HarnessType: "harnez", Status: "completed", TokensCumulative: r.TokensCumulative, TokensSinceCompact: subagent.CompactionTokens(r), TokensTurn: r.TokensTurn, CachedTokens: r.CachedTokens, CreatedAt: now, LastActiveAt: now}
		if err := s.Save(sess); err != nil {
			return err
		}
		out := agentOutput{Session: sess, Response: r.Response, Messages: r.Messages, ReconnectCmd: "harnez agent resume " + id + " \"<prompt>\""}
		if streaming {
			ts.finish(r)
			return nil
		}
		tl.finishTurn(r, id, out.ReconnectCmd)
		if jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
		}
		return printAgentMessages(cmd, r.Messages, r.Response)
	}}
	start.Flags().StringSliceVarP(&startFiles, "file", "f", nil, "prompt file (repeatable; - reads stdin)")
	start.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	start.Flags().BoolVar(&planFirst, "plan-first", false, "agent confirms and plans, then ends its turn without executing; resume to give the go-ahead")
	start.Flags().StringVar(&streamMode, "stream", streamFull, "live output: full (all messages) or stats (heartbeats and final reply only)")

	models := &cobra.Command{Use: "models", Short: "List known agent models and tiers", RunE: func(cmd *cobra.Command, _ []string) error {
		defaultSpec, err := subagent.DefaultModelSpec()
		if err != nil {
			return err
		}
		for _, m := range subagent.KnownModels() {
			line := m.Spec()
			if line == defaultSpec {
				line += "  (default)"
			}
			fmt.Fprintln(cmd.OutOrStdout(), line)
		}
		return nil
	}}

	var chatName string
	chat := &cobra.Command{Use: "chat", Short: "Launch an interactive agent session", Args: noArgs("model is now --model <spec>"), RunE: func(cmd *cobra.Command, args []string) error {
		spec := modelSpec
		if spec == "" {
			var defaultErr error
			spec, defaultErr = subagent.DefaultModelSpec()
			if defaultErr != nil {
				return defaultErr
			}
		}
		m, err := subagent.ResolveModel(spec)
		if err != nil {
			return fmt.Errorf("agent chat %q rejected: %w; ask for guidance rather than using a different model", spec, err)
		}
		s, err := store()
		if err != nil {
			return err
		}
		chatName = name
		canonicalWorkDir, err := filepath.Abs(workDir)
		if err != nil {
			return fmt.Errorf("resolve working directory: %w", err)
		}
		sessions, err := s.List("", true)
		if err != nil {
			return err
		}
		taken := make(map[string]bool, len(sessions)*2)
		for _, existing := range sessions {
			taken[existing.Name] = true
			taken[existing.ID] = true
		}
		if chatName != "" && taken[chatName] {
			return fmt.Errorf("session name %q is already in use", chatName)
		}
		var sess *subagent.Session
		for {
			sessName := chatName
			if sessName == "" {
				sessName, err = subagent.GenerateSessionName(func(candidate string) bool { return taken[candidate] })
				if err != nil {
					return err
				}
			}
			now := time.Now()
			sess = &subagent.Session{
				ID: uuid.NewString(), Name: sessName, Provider: m.Provider, Model: m.Name, Tier: m.Tier, StartPrompt: "",
				WorkingDir: canonicalWorkDir, ParentSessionID: parent(), CallerPID: os.Getpid(),
				HarnessType: "interactive", Status: "active", CreatedAt: now, LastActiveAt: now,
			}
			sess.ControlSocket = filepath.Join(storeDir, sess.ID+".sock")
			if m.Provider == "claude" {
				sess.ProviderSessionID = sess.ID
			}
			err = s.Create(sess)
			if err == nil {
				break
			}
			if chatName != "" || !errors.Is(err, subagent.ErrSessionNameInUse) {
				return err
			}
			taken[sessName] = true
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Harnez Agent Chat: %s (%s)\n", sess.Name, sess.ID)
		opts := subagent.InteractiveOptions{Model: m, SessionID: sess.ID, Name: sess.Name, Dir: canonicalWorkDir, Stdin: cmd.InOrStdin(), Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr(), ControlSocket: sess.ControlSocket, Started: func(pid int) error {
			sess.ProcessPID = pid
			return s.Save(sess)
		}}
		err = agentInteractiveRunner.Chat(cmd.Context(), opts)
		current, getErr := s.Get(sess.ID)
		if getErr != nil {
			return err
		}
		sess = current
		sess.LastActiveAt = time.Now()
		sess.ControlSocket = ""
		sess.ProcessPID = 0
		if sess.Status != "stopped" {
			if err != nil {
				sess.Status = "failed"
			} else {
				sess.Status = "completed"
			}
		}
		if saveErr := s.Save(sess); saveErr != nil {
			if err != nil {
				return fmt.Errorf("%v; save session state: %w", err, saveErr)
			}
			return saveErr
		}
		return err
	}}

	attach := &cobra.Command{Use: "attach", Short: "Attach to an interactive agent session", Args: noArgs("session is now --name <session>"), RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store()
		if err != nil {
			return err
		}
		if name == "" {
			return fmt.Errorf("attach: --name <session> is required")
		}
		sess, err := find(cmd, s, name)
		if err != nil {
			return err
		}
		if !subagent.CanManage(parent(), sess) {
			return fmt.Errorf("session %q is outside caller lineage", sess.ID)
		}
		if sess.ProviderSessionID == "" {
			return fmt.Errorf("session %q cannot be attached: %s does not expose a provider session ID for foreground launches", sess.Name, sess.Provider)
		}
		sess.Status = "active"
		sess.ControlSocket = filepath.Join(storeDir, sess.ID+".sock")
		sess.LastActiveAt = time.Now()
		if err := s.Save(sess); err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Harnez Agent Attached: %s (%s)\n", sess.Name, sess.ID)
		opts := subagent.InteractiveOptions{Model: subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier}, SessionID: sess.ID, Name: sess.Name, Dir: sess.WorkingDir, Stdin: cmd.InOrStdin(), Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr(), ControlSocket: sess.ControlSocket, Started: func(pid int) error {
			sess.ProcessPID = pid
			return s.Save(sess)
		}}
		err = agentInteractiveRunner.Attach(cmd.Context(), opts, sess.ProviderID())
		current, getErr := s.Get(sess.ID)
		if getErr != nil {
			return err
		}
		sess = current
		sess.LastActiveAt = time.Now()
		sess.ControlSocket = ""
		sess.ProcessPID = 0
		if sess.Status != "stopped" {
			if err != nil {
				sess.Status = "failed"
			} else {
				sess.Status = "completed"
			}
		}
		if saveErr := s.Save(sess); saveErr != nil && err == nil {
			return saveErr
		}
		return err
	}}
	attach.ValidArgsFunction = agentSessionCompletion(storeDir, parent)
	chat.AddCommand(attach)

	var resumeFiles []string
	var continueResume bool
	resume := &cobra.Command{Use: "resume [prompt...]", Args: func(*cobra.Command, []string) error { return nil }, RunE: func(cmd *cobra.Command, args []string) error {
		words, tail := promptArgs(args, cmd.Flags().ArgsLenAtDash())
		sessionName := name
		resolved := "name"
		var sess *subagent.Session
		if continueResume && sessionName != "" {
			return fmt.Errorf("resume: --continue cannot be combined with --name")
		}
		if len(words) >= 2 && oldStyleModelWord(words[0]) {
			return fmt.Errorf("model is now --model <spec>; to send this text literally put it after --")
		}
		prompt, err := assemblePrompt(resumeFiles, words, tail, cmd.InOrStdin())
		if err != nil {
			return err
		}
		if err := checkStreamMode(streamMode); err != nil {
			return err
		}
		s, err := store()
		if err != nil {
			return err
		}
		if sessionName == "" {
			xs, listErr := s.List("", true)
			if listErr != nil {
				return listErr
			}
			candidates := attributable(xs, workDir, parent())
			if len(candidates) == 0 {
				return fmt.Errorf("no resumable agent in %s; start one with: harnez agent start --name <name> ...", workDir)
			}
			if !continueResume && len(candidates) != 1 {
				return fmt.Errorf("multiple resumable agents in %s; pass --name (candidates: %s)", workDir, candidateSummary(candidates))
			}
			sess = candidates[0]
			if continueResume {
				resolved = "continue"
			} else {
				resolved = "dir"
			}
		} else {
			sess, err = find(cmd, s, sessionName)
			if err != nil {
				return err
			}
		}
		if !subagent.CanManage(parent(), sess) {
			return fmt.Errorf("session %q is outside caller lineage", sess.ID)
		}
		if modelSpec != "" {
			m, modelErr := subagent.ResolveModel(modelSpec)
			if modelErr != nil || m.Provider != sess.Provider || m.Name != sess.Model || m.Tier != sess.Tier {
				return fmt.Errorf("--model %q conflicts with session %q model %s:%s:%s", modelSpec, sess.Name, sess.Provider, sess.Model, sess.Tier)
			}
		}
		if sess.Status == "active" && sess.HarnessType == "interactive" {
			if err := subagent.SendControl(cmd.Context(), sess.ControlSocket, "prompt", prompt); err != nil {
				return err
			}
			sess.LastActiveAt = time.Now()
			if err := s.Save(sess); err != nil {
				return err
			}
			return write(cmd, agentOutput{Session: sess, Response: "prompt delivered"})
		}
		if sess.HarnessType == "interactive" && sess.ProviderSessionID == "" {
			return fmt.Errorf("session %q cannot be resumed: %s did not expose a provider session ID", sess.Name, sess.Provider)
		}
		d := agentDriver(subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier})
		if checker, ok := d.(subagent.ResumeChecker); ok {
			if resumable, reason := checker.CheckResumable(sess.ProviderID()); !resumable {
				return fmt.Errorf("session %q cannot be resumed: %s; start a new session with: harnez agent start --name <new-name> ...", sess.Name, reason)
			}
		}
		tl := newTimeline(cmd)
		compacted, compactNote := false, ""
		if subagent.ShouldCompact(sess.TokensSinceCompact) {
			if _, err = d.Compact(cmd.Context(), sess.ProviderID()); err != nil {
				return err
			}
			compactNote = fmt.Sprintf("queued /compact at %s new tokens since the last compaction; the agent acknowledges it before its reply", humanCount(sess.TokensSinceCompact))
			sess.TokensSinceCompact = 0
			compacted = true
		}
		var ts *turnStream
		var r *subagent.TurnResult
		sd, streaming := d.(subagent.StreamingDriver)
		streaming = streaming && !jsonOut
		if streaming {
			ts = newTurnStream(cmd, streamMode, compacted)
			ts.info(sess.ID, sess.Provider+":"+sess.Model, "resume", resolved)
			if compacted {
				ts.printf("[compact: %s]\n", compactNote)
			}
			ts.stopCmd, ts.name, ts.planFirst = "harnez agent stop "+sess.Name, sess.Name, planFirst
			ts.watch()
			r, err = sd.ResumeStream(cmd.Context(), sess.ProviderID(), withProtocol(prompt, planFirst), ts.onEvent)
			if err != nil {
				ts.abort()
			}
		} else {
			if compacted {
				tl.log("compact", "%s", compactNote)
			}
			tl.announceTurn("resume", sess.Provider+":"+sess.Model, sess.Name)
			r, err = d.Resume(cmd.Context(), sess.ProviderID(), prompt)
		}
		if err != nil {
			recordResumeFailure(s, sess, err)
			return fmt.Errorf("agent resume %q (%s:%s:%s) failed: %w; verify the provider/model configuration or ask for guidance", sess.Name, sess.Provider, sess.Model, sess.Tier, err)
		}
		sess.LastError = ""
		sess.ResumeFailures = 0
		sess.TokensTurn = r.TokensTurn
		sess.TokensCumulative += r.TokensTurn
		sess.TokensSinceCompact += subagent.CompactionTokens(r)
		sess.CachedTokens = r.CachedTokens
		sess.LastActiveAt = time.Now()
		if err = s.Save(sess); err != nil {
			return err
		}
		if streaming {
			ts.finish(r)
			return nil
		}
		if compacted {
			var ack string
			if r.Messages, ack = subagent.SplitCompactionAck(r.Messages); ack != "" {
				tl.log("compact", "agent acknowledged: %s", firstLine(ack))
			}
		}
		tl.finishTurn(r, sess.ID, "harnez agent resume "+sess.ID+" \"<prompt>\"")
		return write(cmd, agentOutput{Session: sess, Response: r.Response, Messages: r.Messages})
	}}
	resume.ValidArgsFunction = agentSessionCompletion(storeDir, parent)
	resume.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	resume.Flags().BoolVar(&planFirst, "plan-first", false, "agent confirms and plans, then ends its turn without executing; resume to give the go-ahead")
	resume.Flags().StringVar(&streamMode, "stream", streamFull, "live output: full (all messages) or stats (heartbeats and final reply only)")
	resume.Flags().StringSliceVarP(&resumeFiles, "file", "f", nil, "prompt file (repeatable; - reads stdin)")
	resume.Flags().BoolVarP(&continueResume, "continue", "c", false, "resume the most recently active attributable session")

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
		fmt.Fprintln(cmd.OutOrStdout(), "ID\tNAME\tPROVIDER\tSTATUS\tRESUME\tTOKENS\tCACHED")
		for _, x := range xs {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\t%d\t%d\n", x.ID, x.Name, x.Provider, x.Status, resumeState(x), x.TokensCumulative, x.CachedTokens)
		}
		return nil
	}}
	list.Flags().BoolVar(&children, "children", false, "list child sessions")
	list.Flags().BoolVar(&all, "all-sessions", false, "list all sessions")
	list.Flags().BoolVar(&jsonOut, "json", false, "JSON output")

	status := &cobra.Command{Use: "status", Args: noArgs("session is now --name <session>"), RunE: func(cmd *cobra.Command, a []string) error {
		s, e := store()
		if e != nil {
			return e
		}
		if name == "" {
			return runAgentRepoStatus(cmd, s, workDir, jsonOut)
		}
		x, e := find(cmd, s, name)
		if e != nil {
			return e
		}
		if jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(x)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "ID: %s\nName: %s\nStatus: %s\nProvider: %s:%s\nTokens: %d (turn %d)\nCached: %d\n", x.ID, x.Name, x.Status, x.Provider, x.Model, x.TokensCumulative, x.TokensTurn, x.CachedTokens)
		if x.LastError != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Last error: %s\nResume failures: %d\n", x.LastError, x.ResumeFailures)
		}
		return nil
	}}
	status.ValidArgsFunction = agentSessionCompletion(storeDir, parent)
	status.Flags().BoolVar(&jsonOut, "json", false, "JSON output")

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
			path, changed, err := agentpolicy.Configure(workDir, mode, persist)
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
		policyCmd.Flags().BoolVar(&persist, "persist", false, "write the policy to AGENTS.md")
		root.AddCommand(policyCmd)
	}
	compact := &cobra.Command{Use: "compact", Args: noArgs("session is now --name <session>"), RunE: func(cmd *cobra.Command, a []string) error {
		if name == "" {
			return fmt.Errorf("compact: --name <session> is required")
		}
		s, e := store()
		if e != nil {
			return e
		}
		x, e := find(cmd, s, name)
		if e != nil {
			return e
		}
		if x.Status == "active" && x.HarnessType == "interactive" {
			if e := subagent.SendControl(cmd.Context(), x.ControlSocket, "compact", ""); e != nil {
				return e
			}
			x.LastActiveAt = time.Now()
			return s.Save(x)
		}
		if x.HarnessType == "interactive" && x.ProviderSessionID == "" {
			return fmt.Errorf("session %q cannot be compacted: %s did not expose a provider session ID", x.Name, x.Provider)
		}
		_, e = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model, Tier: x.Tier}).Compact(cmd.Context(), x.ProviderID())
		return e
	}}
	compact.ValidArgsFunction = agentSessionCompletion(storeDir, parent)
	stop := &cobra.Command{Use: "stop", Args: noArgs("session is now --name <session>"), RunE: func(cmd *cobra.Command, a []string) error {
		s, e := store()
		if e != nil {
			return e
		}
		if all {
			xs, err := s.List("", true)
			if err != nil {
				return err
			}
			for _, x := range xs {
				if !subagent.CanManage(parent(), x) {
					continue
				}
				if x.HarnessType == "interactive" {
					if x.Status != "active" {
						continue
					}
					if e = subagent.SendControl(cmd.Context(), x.ControlSocket, "stop", ""); e != nil {
						return e
					}
					x.Status = "stopped"
				} else {
					if e = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}).Stop(cmd.Context(), x.ProviderID()); e != nil {
						return e
					}
					x.Status = "stopped"
				}
				if e = s.Save(x); e != nil {
					return e
				}
			}
			return nil
		}
		if children && len(a) == 0 {
			xs, _ := s.List(parent(), false)
			for _, x := range xs {
				if x.HarnessType == "interactive" {
					if x.Status != "active" {
						continue
					}
					if e = subagent.SendControl(cmd.Context(), x.ControlSocket, "stop", ""); e != nil {
						return e
					}
					x.Status = "stopped"
					_ = s.Save(x)
					continue
				}
				if e = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}).Stop(cmd.Context(), x.ProviderID()); e != nil {
					return e
				}
				x.Status = "stopped"
				_ = s.Save(x)
			}
			return nil
		}
		if name == "" {
			return fmt.Errorf("stop: --name <session> is required")
		}
		x, e := find(cmd, s, name)
		if e != nil {
			return e
		}
		if !subagent.CanManage(parent(), x) {
			return fmt.Errorf("session %q is outside caller lineage", x.ID)
		}
		if x.HarnessType == "interactive" {
			if x.Status != "active" {
				return fmt.Errorf("session %q is not active", x.Name)
			}
			if e = subagent.SendControl(cmd.Context(), x.ControlSocket, "stop", ""); e != nil {
				return e
			}
			x.Status = "stopped"
			x.LastActiveAt = time.Now()
			return s.Save(x)
		}
		e = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}).Stop(cmd.Context(), x.ProviderID())
		x.Status = "stopped"
		if e == nil {
			e = s.Save(x)
		}
		return e
	}}
	stop.Flags().BoolVar(&children, "children", false, "stop child sessions")
	stop.Flags().BoolVar(&all, "all", false, "stop all manageable sessions")
	stop.ValidArgsFunction = agentSessionCompletion(storeDir, parent)
	remove := &cobra.Command{Use: "delete", Aliases: []string{"rm"}, Args: noArgs("session is now --name <session>"), RunE: func(cmd *cobra.Command, a []string) error {
		s, e := store()
		if e != nil {
			return e
		}
		if all {
			xs, err := s.List("", true)
			if err != nil {
				return err
			}
			for _, x := range xs {
				if !subagent.CanManage(parent(), x) {
					continue
				}
				if x.HarnessType == "interactive" {
					if x.Status == "active" {
						return fmt.Errorf("session %q is active; stop it before deletion", x.Name)
					}
					if e = s.Delete(x.ID); e != nil {
						return e
					}
					continue
				}
				if e = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}).Delete(cmd.Context(), x.ProviderID()); e != nil {
					return e
				}
				if e = s.Delete(x.ID); e != nil {
					return e
				}
			}
			return nil
		}
		if name == "" {
			return fmt.Errorf("delete: --name <session> is required")
		}
		x, e := find(cmd, s, name)
		if e != nil {
			return e
		}
		if !subagent.CanManage(parent(), x) {
			return fmt.Errorf("session %q is outside caller lineage", x.ID)
		}
		if x.HarnessType == "interactive" {
			if x.Status == "active" {
				return fmt.Errorf("session %q is active; stop it before deletion", x.Name)
			}
			return s.Delete(x.ID)
		}
		if e = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}).Delete(cmd.Context(), x.ProviderID()); e != nil {
			return e
		}
		return s.Delete(x.ID)
	}}
	remove.Flags().BoolVar(&all, "all", false, "delete all manageable sessions")
	remove.ValidArgsFunction = agentSessionCompletion(storeDir, parent)
	root.AddCommand(start, models, chat, resume, list, status, compact, stop, remove)
	silenceUsage(root)
	return root
}

// agentSessionCompletion returns names visible to the current caller. Cobra
// displays the text after the tab as a completion description.
// timeline reports what start/resume technically did on stderr as
// "[HH:MM:SS label] description" lines under a "[session timeline]" header.
type timeline struct {
	cmd     *cobra.Command
	began   time.Time
	printed bool
}

func newTimeline(cmd *cobra.Command) *timeline { return &timeline{cmd: cmd, began: time.Now()} }

func (t *timeline) log(label, format string, args ...any) {
	w := t.cmd.ErrOrStderr()
	if !t.printed {
		fmt.Fprintln(w, "[session timeline]")
		t.printed = true
	}
	fmt.Fprintf(w, "[%s %s] %s\n", time.Now().Format("15:04:05"), label, fmt.Sprintf(format, args...))
}

// announceTurn tells the calling agent what is about to happen and that it must wait.
func (t *timeline) announceTurn(verb, target, session string) {
	t.log(verb, "one synchronous turn on %s (session %q) via the provider CLI", target, session)
	t.log("wait", "caller must wait for the agent messages on stdout; no polling, no re-sending the prompt")
}

func (t *timeline) finishTurn(r *subagent.TurnResult, id, reconnect string) {
	t.log("done", "turn finished in %s, %d messages, %d tokens", time.Since(t.began).Round(time.Second), max(len(r.Messages), 1), r.TokensTurn)
	t.log("session", "%s; reconnect: %s", id, reconnect)
}

// printAgentMessages writes the agent's messages verbatim to stdout, each
// under its own "[msg N]" label line.
func printAgentMessages(cmd *cobra.Command, msgs []string, fallback string) error {
	if len(msgs) == 0 {
		msgs = []string{fallback}
	}
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "[agent messages]")
	for i, m := range msgs {
		if _, err := fmt.Fprintf(w, "[msg %d]\n%s\n", i+1, m); err != nil {
			return err
		}
	}
	return nil
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func agentSessionCompletion(storeDir string, parent func() string) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		activeStoreDir := storeDir
		if value, err := cmd.InheritedFlags().GetString("store-dir"); err == nil && value != "" {
			activeStoreDir = value
		}
		store, err := subagent.OpenSessionStore(activeStoreDir)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		sessions, err := store.List("", true)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		completions := make([]string, 0, len(sessions))
		for _, session := range sessions {
			if !subagent.CanManage(parent(), session) || !strings.HasPrefix(session.Name, toComplete) {
				continue
			}
			completions = append(completions, session.Name+"\t"+sessionPromptDescription(session.StartPrompt))
		}
		sort.Strings(completions)
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}

func agentModelCompletion(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var out []string
	for _, model := range subagent.KnownModels() {
		if strings.HasPrefix(model.Spec(), toComplete) {
			out = append(out, model.Spec())
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func sessionPromptDescription(prompt string) string {
	// Scrubbing is best-effort: completion descriptions are a convenience,
	// not a guarantee that arbitrary confidential prose is detected.
	description := strings.Join(strings.Fields(privacy.ScrubText(prompt)), " ")
	if description == "" {
		return "agent prompt unavailable"
	}
	if len([]rune(description)) > 100 {
		description = string([]rune(description)[:97]) + "..."
	}
	return description
}

func recordResumeFailure(store *subagent.FileSessionStore, sess *subagent.Session, err error) {
	sess.LastError = firstLine(err.Error())
	if len([]rune(sess.LastError)) > 300 {
		sess.LastError = string([]rune(sess.LastError)[:300])
	}
	sess.ResumeFailures++
	_ = store.Save(sess)
}

func resumeState(sess *subagent.Session) string {
	if sess.LastError != "" {
		return "failed"
	}
	d := agentDriver(subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier})
	if checker, ok := d.(subagent.ResumeChecker); ok {
		if resumable, _ := checker.CheckResumable(sess.ProviderID()); !resumable {
			return "terminal"
		}
	}
	return "ok"
}

func attributable(sessions []*subagent.Session, dir, callerParent string) []*subagent.Session {
	want, _ := filepath.Abs(dir)
	result := make([]*subagent.Session, 0, len(sessions))
	for _, sess := range sessions {
		if sess.Status != "completed" && !(sess.Status == "active" && sess.HarnessType == "interactive") {
			continue
		}
		have, _ := filepath.Abs(sess.WorkingDir)
		if have != want || !subagent.CanManage(callerParent, sess) || resumeState(sess) == "terminal" {
			continue
		}
		// Quarantined sessions will be excluded here when #306 lands.
		result = append(result, sess)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].LastActiveAt.Equal(result[j].LastActiveAt) {
			return result[i].Name < result[j].Name
		}
		return result[i].LastActiveAt.After(result[j].LastActiveAt)
	})
	return result
}

func candidateSummary(sessions []*subagent.Session) string {
	items := make([]string, len(sessions))
	for i, sess := range sessions {
		items[i] = fmt.Sprintf("%s (%s, %s)", sess.Name, sess.LastActiveAt.Format(time.RFC3339), sess.Provider+":"+sess.Model)
	}
	return strings.Join(items, ", ")
}

type agentRepoStatus struct {
	Policy   agentpolicy.State   `json:"policy"`
	Sessions []*subagent.Session `json:"sessions"`
}

func runAgentRepoStatus(cmd *cobra.Command, store *subagent.FileSessionStore, dir string, jsonOut bool) error {
	if dir == "" {
		dir = "."
	}
	policy, err := agentpolicy.Resolve(dir)
	if err != nil {
		return err
	}
	all, err := store.List("", true)
	if err != nil {
		return err
	}
	result := agentRepoStatus{Policy: policy, Sessions: agentpolicy.RepoSessions(dir, all)}
	if jsonOut {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
	}
	label := map[string]string{"harnez": "Enabled", "native": "Disabled", "unset": "Unset"}[policy.Mode]
	if label == "" {
		label = "Unset"
	}
	if policy.Source != "" {
		label += " (" + policy.Source + ")"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Subagent Policy State: %s\n", label)
	if policy.Conflict {
		fmt.Fprintf(cmd.OutOrStdout(), "Policy note: local %s overrides main %s\n", policy.LocalMode, policy.MainMode)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Repository Agent Sessions:")
	if len(result.Sessions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
		return nil
	}
	for _, sess := range result.Sessions {
		work, _ := filepath.Abs(sess.WorkingDir)
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\t%s\t%s\t%s\t%s\n", sess.ID, sess.Name, sess.Provider, sess.Status, work)
	}
	return nil
}

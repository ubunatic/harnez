package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/agentpolicy"
	"ubunatic.com/harnez/internal/privacy"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/telemetry"
)

var agentDriver = func(m subagent.Model, dir string) subagent.Driver {
	switch m.Provider {
	case "claude":
		return subagent.ClaudeDriver{Dir: dir}
	case "codex":
		return subagent.CodexDriver{}
	case "agy":
		return subagent.AgyDriver{Dir: dir}
	default:
		return subagent.UnsupportedDriver{Provider: m.Provider}
	}
}

func agentUsageDBPath() string {
	path, _ := telemetry.DefaultDBPath()
	return path
}

var agentInteractiveRunner subagent.InteractiveRunner = subagent.CLIInteractiveRunner{}

type agentOutput struct {
	*subagent.Session
	Response     string   `json:"response,omitempty"`
	Messages     []string `json:"messages,omitempty"`
	ReconnectCmd string   `json:"reconnect_cmd,omitempty"`
}

func newAgentCmd() *cobra.Command {
	var jsonOut, children, all, detach, allowExhaustedQuota bool
	var workerID string
	var storeDir, workDir, name, modelSpec, streamMode, roleSpec string
	var agentTimeout time.Duration
	var planSpec string
	var rootPrompt string
	var rootFiles []string
	var rootContinue bool
	root := &cobra.Command{Use: "agent", Short: "Manage subagent sessions", Args: cobra.ArbitraryArgs,
		Long: `Manage subagent sessions across supported providers.

Short forms:
  harnez agent -p "summarise the open tickets"
  harnez agent "update the changelog"
  harnez agent --name docs -d ~/projects/x "update the changelog"
  harnez agent -c -p "continue"
  harnez agent start --name w --model luna -f task.md -- "extra instructions"
  harnez agent start -i --model claude:haiku --name chat
  harnez agent resume --name w "next step"
  harnez agent resume -i --name w
  harnez agent --name w -p "/compact"
  harnez agent --model luna:low --role advisor -p "how does X work?"

Models: run "harnez agent models" for the known names. --model takes
provider:name[:tier], a short alias (luna, opus) or alias:tier (luna:low).
The model default comes from spec/agent.yaml. Sessions are chosen by --name,
attribution in -d, or -c. Use -- to send text literally. Slash commands are
/compact, /stop, and /status. Automatic compaction uses
agent.compact_threshold_tokens from ~/.harnez/config.yaml (default 200000).
agent.compact_thresholds may override provider:model[:tier] thresholds.`}
	root.PersistentFlags().StringVar(&storeDir, "store-dir", subagent.DefaultStoreDir(), "session store directory")
	root.PersistentFlags().StringVarP(&workDir, "dir", "d", ".", "working directory or session scope")
	root.PersistentFlags().StringVar(&name, "name", "", "session name")
	root.PersistentFlags().StringVar(&modelSpec, "model", "", "provider:model[:tier]")
	root.PersistentFlags().BoolVar(&allowExhaustedQuota, "allow-exhausted-quota", false, "start even when cached provider quota is exhausted")
	root.PersistentFlags().StringVar(&roleSpec, "role", "", "agent role for a new session: orchestrator, developer, reviewer or advisor (default from spec/agent.yaml)")
	root.PersistentFlags().DurationVar(&agentTimeout, "timeout", 0, "maximum foreground turn duration (0 waits without a limit)")
	_ = root.RegisterFlagCompletionFunc("role", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		names, _ := subagent.RoleNames()
		return names, cobra.ShellCompDirectiveNoFileComp
	})
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if err := sessionTipHook(cmd, args); err != nil {
			return err
		}
		return guardLeafRole(cmd.Name())
	}
	root.Flags().StringVarP(&rootPrompt, "prompt", "p", "", "prompt text")
	root.Flags().BoolVarP(&rootContinue, "continue", "c", false, "resume the most recently active attributable session")
	root.Flags().StringSliceVarP(&rootFiles, "file", "f", nil, "prompt file (repeatable; - reads stdin)")
	root.Flags().StringVar(&streamMode, "stream", streamFull, "live output: full (all messages) or stats (heartbeats and final reply only)")
	root.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	root.Flags().StringVar(&planSpec, "plan", "no", "planning gate: yes or no")
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
	_ = root.RegisterFlagCompletionFunc("plan", flagValueCompletion("yes", "no"))
	_ = root.RegisterFlagCompletionFunc("stream", flagValueCompletion(streamFull, streamStats))
	find := func(cmd *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
		dir := ""
		if cmd.Flags().Changed("dir") {
			dir = workDir
		}
		return resolveSession(s, id, dir)
	}
	root.RunE = func(cmd *cobra.Command, args []string) error {
		planFirst, err := parsePlanSpec(planSpec)
		if err != nil {
			return err
		}
		if workDir == "" {
			workDir = "."
		}
		dash := cmd.Flags().ArgsLenAtDash()
		words, tail := promptArgs(args, dash)
		if rootPrompt == "" && !rootContinue && dash < 0 && len(words) == 1 && !strings.ContainsAny(words[0], " \t\r\n") {
			known := false
			for _, child := range cmd.Commands() {
				if child.Name() == words[0] {
					known = true
					break
				}
				for _, alias := range child.Aliases {
					if alias == words[0] {
						known = true
						break
					}
				}
			}
			if !known {
				message := fmt.Sprintf("unknown command %q for %q; use -p \"<prompt>\" or -- <prompt> to send a prompt", words[0], "harnez agent")
				if suggestions := cmd.SuggestionsFor(words[0]); len(suggestions) > 0 {
					message += fmt.Sprintf("\nDid you mean this?\n\t%s", strings.Join(suggestions, "\n\t"))
				}
				return errors.New(message)
			}
		}
		if rootPrompt == "" && len(rootFiles) == 0 && len(words) == 0 && len(tail) == 0 && !rootContinue {
			return cmd.Help()
		}
		if rootPrompt != "" && strings.HasPrefix(rootPrompt, "--") {
			return fmt.Errorf("-p needs prompt text but got flag %q; put -p last or use -- \"<text>\"", rootPrompt)
		}
		if modelSpec == "" && len(words) >= 2 && oldStyleModelWord(words[0]) {
			return fmt.Errorf("model is now --model <spec>; to send this text literally put it after --")
		}
		promptWords := words
		if rootPrompt != "" {
			promptWords = append([]string{rootPrompt}, promptWords...)
		}
		prompt, err := assemblePrompt(rootFiles, promptWords, tail, cmd.InOrStdin())
		if err != nil {
			return err
		}
		deps := agentDeps{store: store, parent: parent, find: find, storeDir: storeDir, dbPath: agentUsageDBPath()}
		if rootContinue && name != "" {
			return fmt.Errorf("agent: --continue cannot be combined with --name")
		}
		trimmed := strings.TrimSpace(prompt)
		if len(tail) == 0 && strings.Count(trimmed, "\n") == 0 && strings.HasPrefix(trimmed, "/") {
			// Typos are rejected before any session is resolved.
			if !knownSlashCommands[trimmed] {
				return fmt.Errorf("unknown agent command %q; send it literally with: -- %s", trimmed, trimmed)
			}
			if e := guardLeafRole(strings.TrimPrefix(trimmed, "/")); e != nil {
				return e
			}
			s, e := store()
			if e != nil {
				return e
			}
			x, _, e := resolveResumeSession(cmd, deps, s, resumeRequest{Role: roleSpec, Dir: workDir, Name: name, Continue: rootContinue})
			if e != nil {
				return e
			}
			switch trimmed {
			case "/compact":
				return compactSession(cmd, s, x)
			case "/stop":
				return stopSession(cmd, s, x)
			default: // "/status"
				return statusSession(cmd, x, jsonOut)
			}
		}
		if name != "" {
			s, e := store()
			if e != nil {
				return e
			}
			if _, findErr := find(cmd, s, name); findErr == nil {
				return withAgentTimeout(cmd, agentTimeout, func() error {
					return runResume(cmd, deps, resumeRequest{Role: roleSpec, Prompt: prompt, Name: name, ModelSpec: modelSpec, Dir: workDir, StreamMode: streamMode, JSON: jsonOut, PlanFirst: planFirst})
				})
			} else if !strings.Contains(findErr.Error(), "not found") {
				return findErr
			}
			return withAgentTimeout(cmd, agentTimeout, func() error {
				return runStart(cmd, deps, startRequest{Role: roleSpec, Prompt: prompt, StoredPrompt: promptStorage(rootFiles, promptWords, tail, prompt), Name: name, ModelSpec: modelSpec, Dir: workDir, StreamMode: streamMode, JSON: jsonOut, PlanFirst: planFirst, AllowExhaustedQuota: allowExhaustedQuota})
			})
		}
		if rootContinue {
			s, e := store()
			if e != nil {
				return e
			}
			xs, e := s.List("", true)
			if e != nil {
				return e
			}
			candidates := attributable(xs, workDir, parent())
			if len(candidates) > 0 {
				return withAgentTimeout(cmd, agentTimeout, func() error {
					return runResume(cmd, deps, resumeRequest{Role: roleSpec, Prompt: prompt, ModelSpec: modelSpec, Dir: workDir, StreamMode: streamMode, Continue: true, JSON: jsonOut, PlanFirst: planFirst})
				})
			}
		}
		return withAgentTimeout(cmd, agentTimeout, func() error {
			return runStart(cmd, deps, startRequest{Role: roleSpec, Prompt: prompt, StoredPrompt: promptStorage(rootFiles, promptWords, tail, prompt), ModelSpec: modelSpec, Dir: workDir, StreamMode: streamMode, JSON: jsonOut, PlanFirst: planFirst, AllowExhaustedQuota: allowExhaustedQuota})
		})
	}
	var startFiles []string
	var startPrompt string
	var startInteractive bool
	start := &cobra.Command{Use: "start [prompt...]", Short: "Start a new agent session", Example: "  harnez agent start --name w --model luna -f task.md -- \"extra instructions\"\n  harnez agent start -i --model claude:haiku --name chat", Args: func(*cobra.Command, []string) error { return nil }, RunE: func(cmd *cobra.Command, args []string) error {
		planFirst, err := parsePlanSpec(planSpec)
		if err != nil {
			return err
		}
		words, tail := promptArgs(args, cmd.Flags().ArgsLenAtDash())
		if startPrompt != "" {
			words = append([]string{startPrompt}, words...)
		}
		if rootPrompt != "" && strings.HasPrefix(rootPrompt, "--") {
			return fmt.Errorf("-p needs prompt text but got flag %q; put -p last or use -- \"<text>\"", rootPrompt)
		}
		if modelSpec == "" && len(words) >= 2 && oldStyleModelWord(words[0]) {
			return fmt.Errorf("model is now --model <spec>; to send this text literally put it after --")
		}
		prompt := ""
		if !startInteractive || len(startFiles) > 0 || len(words) > 0 || len(tail) > 0 {
			prompt, err = assemblePrompt(startFiles, words, tail, cmd.InOrStdin())
			if err != nil {
				return err
			}
		}
		if startInteractive {
			if err := validateInteractiveFlags(cmd, detach, jsonOut, streamMode, agentTimeout, workerID != ""); err != nil {
				return err
			}
			return runInteractiveStart(cmd, interactiveDeps{store: store, parent: parent, storeDir: storeDir}, interactiveStartRequest{Prompt: prompt, Name: name, ModelSpec: modelSpec, Dir: workDir, Role: roleSpec})
		}
		req := startRequest{Role: roleSpec, Prompt: prompt, StoredPrompt: promptStorage(startFiles, words, tail, prompt), Name: name, ModelSpec: modelSpec, Dir: workDir, StreamMode: streamMode, JSON: jsonOut, PlanFirst: planFirst, SessionID: workerID, AllowExhaustedQuota: allowExhaustedQuota}
		if workerID != "" {
			return runDetachedWorker(cmd, req, storeDir)
		}
		if detach {
			return launchDetachedWithPreflight(cmd, req, storeDir, parent(), subagent.CheckCodexAuth)
		}
		return withAgentTimeout(cmd, agentTimeout, func() error {
			return runStart(cmd, agentDeps{store: store, parent: parent, find: find, storeDir: storeDir, dbPath: agentUsageDBPath()}, req)
		})
	}}
	start.Flags().StringSliceVarP(&startFiles, "file", "f", nil, "prompt file (repeatable; - reads stdin)")
	start.Flags().StringVarP(&startPrompt, "prompt", "p", "", "prompt text")
	start.Flags().BoolVarP(&startInteractive, "interactive", "i", false, "launch the provider's interactive terminal")
	start.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	start.Flags().BoolVar(&detach, "detach", false, "run the agent in the background")
	start.Flags().BoolVar(&detach, "async", false, "alias for --detach")
	start.Flags().BoolVar(&allowExhaustedQuota, "allow-exhausted-quota", false, "start even when cached provider quota is exhausted")
	start.Flags().StringVar(&workerID, "worker-session", "", "internal detached worker session ID")
	start.Flags().StringVar(&planSpec, "plan", "no", "planning gate: yes or no")
	_ = start.RegisterFlagCompletionFunc("plan", flagValueCompletion("yes", "no"))
	_ = start.RegisterFlagCompletionFunc("stream", flagValueCompletion(streamFull, streamStats))
	start.Flags().StringVar(&streamMode, "stream", streamFull, "live output: full (all messages) or stats (heartbeats and final reply only)")

	var modelNamesOnly bool
	models := &cobra.Command{Use: "models", Short: "List known agent models with roles and when to use them", RunE: func(cmd *cobra.Command, _ []string) error {
		defaultSpec, err := subagent.DefaultModelSpec()
		if err != nil {
			return err
		}
		if modelNamesOnly {
			for _, spec := range subagent.KnownModelSpecs() {
				fmt.Fprintln(cmd.OutOrStdout(), spec)
			}
			return nil
		}
		entries := subagent.ModelEntriesWithDriver(subagent.KnownModelEntries(), func(m subagent.Model) subagent.Driver {
			return agentDriver(m, workDir)
		})
		if jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(entries)
		}
		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
		fmt.Fprintln(tw, "SPEC\tMODEL\tAVAILABILITY\tEFFORT\tCOST\tEFF\tSKILLS\tROLES\tUSE")
		for _, e := range entries {
			spec := e.Spec
			if spec == defaultSpec {
				spec += " (default)"
			}
			if !e.Batch {
				spec += " (interactive only)"
			}
			effort := "yes"
			if !e.Effort {
				effort = "no"
			}
			availability := providerAvailability(e.Model.Provider, e.Model.Name)
			marker := availability.State
			if marker == "unknown" && availability.Age > 0 {
				marker = fmt.Sprintf("unknown (%s)", quotaAgeLabel(availability.Age))
			} else if marker == "exhausted" {
				marker = fmt.Sprintf("exhausted (resets %s)", quotaAgeLabel(availability.ResetIn))
			} else if marker == "stale" {
				marker = fmt.Sprintf("stale (%s)", quotaAgeLabel(availability.Age))
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\n", spec, e.Model.Name, marker, effort, e.Cost, e.Eff, e.Skills, e.Roles, e.Use)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		legend, err := subagent.ModelsLegend()
		if err != nil {
			return err
		}
		if legend != "" {
			fmt.Fprintln(cmd.OutOrStdout(), "\n"+legend)
		}
		return nil
	}}
	models.Flags().BoolVar(&modelNamesOnly, "names", false, "print only the model specs, one per line")
	models.Flags().BoolVar(&jsonOut, "json", false, "JSON output")

	var resumeFiles []string
	var resumeWorkerID string
	var continueResume bool
	var resumePrompt string
	var resumeInteractive bool
	resume := &cobra.Command{Use: "resume [session] [prompt...]", Short: "Resume an existing agent session", Example: "  harnez agent resume --name w \"next step\"\n  harnez agent resume <id> \"next step\"\n  harnez agent resume -i --name w", Args: func(*cobra.Command, []string) error { return nil }, RunE: func(cmd *cobra.Command, args []string) error {
		planFirst, err := parsePlanSpec(planSpec)
		if err != nil {
			return err
		}
		words, tail := promptArgs(args, cmd.Flags().ArgsLenAtDash())
		selector := ""
		if name == "" && len(words) > 0 {
			if s, storeErr := store(); storeErr != nil {
				return storeErr
			} else if matches, matchErr := matchResumeSelector(s, words[0]); matchErr != nil {
				return matchErr
			} else if matches {
				selector = words[0]
				words = words[1:]
			}
		}
		if resumePrompt != "" {
			words = append([]string{resumePrompt}, words...)
		}
		if continueResume && name != "" {
			return fmt.Errorf("resume: --continue cannot be combined with --name")
		}
		if len(words) >= 2 && oldStyleModelWord(words[0]) {
			return fmt.Errorf("model is now --model <spec>; to send this text literally put it after --")
		}
		prompt := ""
		if !resumeInteractive || len(resumeFiles) > 0 || len(words) > 0 || len(tail) > 0 {
			prompt, err = assemblePrompt(resumeFiles, words, tail, cmd.InOrStdin())
			if err != nil {
				return err
			}
		}
		if resumeInteractive {
			if err := validateInteractiveFlags(cmd, false, jsonOut, streamMode, agentTimeout, resumeWorkerID != ""); err != nil {
				return err
			}
			return runInteractiveResume(cmd, interactiveDeps{store: store, parent: parent, find: find, storeDir: storeDir}, interactiveResumeRequest{Name: name, Prompt: prompt, Dir: workDir})
		}
		if resumeWorkerID != "" {
			return runDetachedResumeWorker(cmd, resumeWorkerID, resumeRequest{Role: roleSpec, Prompt: prompt, Name: name, ModelSpec: modelSpec, Dir: workDir, StreamMode: streamMode, JSON: true, PlanFirst: planFirst}, storeDir)
		}
		return withAgentTimeout(cmd, agentTimeout, func() error {
			return runResume(cmd, agentDeps{store: store, parent: parent, find: find, storeDir: storeDir, dbPath: agentUsageDBPath()}, resumeRequest{Role: roleSpec, Prompt: prompt, Name: name, Selector: selector, ModelSpec: modelSpec, Dir: workDir, StreamMode: streamMode, Continue: continueResume, JSON: jsonOut, PlanFirst: planFirst})
		})
	}}
	resume.ValidArgsFunction = agentSessionCompletion(storeDir, parent)
	resume.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	resume.Flags().StringVar(&resumeWorkerID, "worker-session", "", "internal detached worker session ID")
	resume.Flags().StringVar(&planSpec, "plan", "no", "planning gate: yes or no")
	_ = resume.RegisterFlagCompletionFunc("plan", flagValueCompletion("yes", "no"))
	_ = resume.RegisterFlagCompletionFunc("stream", flagValueCompletion(streamFull, streamStats))
	resume.Flags().StringVar(&streamMode, "stream", streamFull, "live output: full (all messages) or stats (heartbeats and final reply only)")
	resume.Flags().StringSliceVarP(&resumeFiles, "file", "f", nil, "prompt file (repeatable; - reads stdin)")
	resume.Flags().StringVarP(&resumePrompt, "prompt", "p", "", "prompt text")
	resume.Flags().BoolVarP(&resumeInteractive, "interactive", "i", false, "resume in the provider's interactive terminal")
	resume.Flags().BoolVarP(&continueResume, "continue", "c", false, "resume the most recently active attributable session")

	list := &cobra.Command{Use: "list", Short: "List agent sessions", RunE: func(cmd *cobra.Command, _ []string) error {
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

	status := &cobra.Command{Use: "status", Short: "Show agent session status", Args: noArgs("session is now --name <session>"), RunE: func(cmd *cobra.Command, a []string) error {
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
		return statusSession(cmd, x, jsonOut)
	}}
	status.ValidArgsFunction = agentSessionCompletion(storeDir, parent)
	status.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	var waitTimeout time.Duration
	wait := &cobra.Command{Use: "wait <session>", Short: "Wait for an agent session to finish", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store()
		if err != nil {
			return err
		}
		sess, err := waitForAgent(cmd.Context(), s, args[0], waitTimeout)
		if err != nil {
			return err
		}
		if sess.Status == "running" {
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(sess)
			}
			return fmt.Errorf("timed out waiting for session %q", sess.ID)
		}
		return writeAgentOutput(cmd, jsonOut, agentOutput{Session: sess, Response: sess.Response, Messages: sess.Messages})
	}}
	wait.Flags().DurationVar(&waitTimeout, "timeout", 0, "maximum wait duration (0 waits indefinitely)")
	wait.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	wait.ValidArgsFunction = agentSessionCompletion(storeDir, parent)

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
		policyCmd.Flags().BoolVar(&persist, "persist", false, "accepted for compatibility; policy stays in .harnez/rules/Local.md")
		root.AddCommand(policyCmd)
	}
	compact := &cobra.Command{Use: "compact", Short: "Compact an agent session", Args: noArgs("session is now --name <session>"), RunE: func(cmd *cobra.Command, a []string) error {
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
		return compactSession(cmd, s, x)
	}}
	compact.ValidArgsFunction = agentSessionCompletion(storeDir, parent)
	stop := &cobra.Command{Use: "stop", Short: "Stop an agent session", Args: noArgs("session is now --name <session>"), RunE: func(cmd *cobra.Command, a []string) error {
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
				if x.HarnessType == "interactive" && x.Status != "active" {
					continue
				}
				if e = stopSession(cmd, s, x); e != nil {
					return e
				}
			}
			return nil
		}
		if children && len(a) == 0 {
			xs, _ := s.List(parent(), false)
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
		return stopSession(cmd, s, x)
	}}
	stop.Flags().BoolVar(&children, "children", false, "stop child sessions")
	stop.Flags().BoolVar(&all, "all", false, "stop all manageable sessions")
	stop.ValidArgsFunction = agentSessionCompletion(storeDir, parent)
	var allCompleted, force bool
	warnUnrated := func(cmd *cobra.Command, x *subagent.Session) bool {
		return !sessionLatestTurnRated(x)
	}
	remove := &cobra.Command{Use: "delete", Short: "Delete an agent session", Aliases: []string{"rm"}, Args: noArgs("session is now --name <session>"), RunE: func(cmd *cobra.Command, a []string) error {
		s, e := store()
		if e != nil {
			return e
		}
		if allCompleted {
			if all || name != "" {
				return fmt.Errorf("delete --all-completed cannot be combined with --all or --name")
			}
			xs, err := s.List("", true)
			if err != nil {
				return err
			}
			var deleted []string
			var failures []error
			for _, x := range xs {
				if !subagent.CanManage(parent(), x) {
					continue
				}
				if x.Status != "completed" {
					continue
				}
				if warnUnrated(cmd, x) {
					printUnratedDeleteWarning(cmd, []*subagent.Session{x})
					if !force {
						failures = append(failures, fmt.Errorf("%s: refusing to delete unrated session without --force", x.Name))
						continue
					}
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
		if all {
			xs, err := s.List("", true)
			if err != nil {
				return err
			}
			var failures []error
			var unrated []*subagent.Session
			for _, x := range xs {
				if !subagent.CanManage(parent(), x) {
					continue
				}
				if warnUnrated(cmd, x) {
					unrated = append(unrated, x)
					if !force {
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
					failures = append(failures, fmt.Errorf("%s: %w", x.Name, e))
					continue
				}
				if e = s.Delete(x.ID); e != nil {
					failures = append(failures, fmt.Errorf("%s: %w", x.Name, e))
					continue
				}
				fmt.Fprintln(cmd.OutOrStdout(), x.Name)
			}
			if len(unrated) > 0 {
				printUnratedDeleteWarning(cmd, unrated)
			}
			for _, err := range failures {
				fmt.Fprintln(cmd.ErrOrStderr(), "delete failed:", err)
			}
			if len(failures) > 0 {
				return errors.New("some sessions failed to delete")
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
		if warnUnrated(cmd, x) {
			printUnratedDeleteWarning(cmd, []*subagent.Session{x})
			if !force {
				return fmt.Errorf("delete: refusing to delete unrated session %q without --force", x.Name)
			}
		}
		if x.HarnessType == "interactive" {
			if x.Status == "active" {
				return fmt.Errorf("session %q is active; stop it before deletion", x.Name)
			}
			return s.Delete(x.ID)
		}
		if e = agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model}, x.WorkingDir).Delete(cmd.Context(), x.ProviderID()); e != nil {
			return e
		}
		return s.Delete(x.ID)
	}}
	remove.Flags().BoolVar(&all, "all", false, "delete all manageable sessions")
	remove.Flags().BoolVar(&allCompleted, "all-completed", false, "delete all completed sessions")
	remove.Flags().BoolVar(&force, "force", false, "delete unrated sessions after warning")
	remove.ValidArgsFunction = agentSessionCompletion(storeDir, parent)
	rate := &cobra.Command{Use: "rate <1-5> <reason>", Short: "Rate the latest turn of an agent session", Args: cobra.MinimumNArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		if name == "" {
			return fmt.Errorf("rate: --name <session> is required")
		}
		s, err := store()
		if err != nil {
			return err
		}
		sess, err := find(cmd, s, name)
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
	rate.ValidArgsFunction = agentSessionCompletion(storeDir, parent)
	root.AddCommand(start, models, resume, list, status, wait, compact, stop, remove, rate)
	silenceUsage(root)
	return root
}

func matchResumeSelector(store *subagent.FileSessionStore, selector string) (bool, error) {
	sessions, err := store.List("", true)
	if err != nil {
		return false, err
	}
	_, ok := findResumeSelector(sessions, selector)
	return ok, nil
}

func rateLatestSessionTurn(store *subagent.FileSessionStore, sess *subagent.Session, score int, reason string) error {
	if score < 1 || score > 5 {
		return fmt.Errorf("rate: score must be an integer from 1 to 5")
	}
	if sess.Turn < 1 {
		return fmt.Errorf("rate: session %q has no recorded turn", sess.Name)
	}
	latestIndex := -1
	for i := range sess.TurnRecords {
		if sess.TurnRecords[i].Turn == sess.Turn {
			latestIndex = i
		}
	}
	if latestIndex < 0 {
		sess.TurnRecords = append(sess.TurnRecords, subagent.TurnRecord{Turn: sess.Turn})
		latestIndex = len(sess.TurnRecords) - 1
	}
	latest := &sess.TurnRecords[latestIndex]
	rating := score
	latest.Rating, latest.RatingReason = &rating, reason
	return store.Save(sess)
}

func sessionLatestTurnRated(sess *subagent.Session) bool {
	if sess.Turn < 1 {
		return false
	}
	for i := range sess.TurnRecords {
		if sess.TurnRecords[i].Turn == sess.Turn {
			return sess.TurnRecords[i].Rating != nil
		}
	}
	return false
}

func printUnratedDeleteWarning(cmd *cobra.Command, sessions []*subagent.Session) {
	if len(sessions) == 0 {
		return
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "warning: unrated latest turns:")
	for _, sess := range sessions {
		fmt.Fprintf(cmd.ErrOrStderr(), "  %s: harnez agent rate --name %s <1-5> \"<reason>\"\n", sess.Name, sess.Name)
	}
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
	t.log("wait", "wait for the turn output; %s", waitHint(session))
}

func (t *timeline) finishTurn(r *subagent.TurnResult, id, reconnect string) {
	t.log("done", "turn finished in %s, %d messages, %d tokens (incl. cached)", time.Since(t.began).Round(time.Second), max(len(r.Messages), 1), r.TokensTurn)
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
	for _, spec := range subagent.KnownModelSpecs() {
		if strings.HasPrefix(spec, toComplete) {
			out = append(out, spec)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func flagValueCompletion(values ...string) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var result []string
		for _, value := range values {
			if strings.HasPrefix(value, toComplete) {
				result = append(result, value)
			}
		}
		return result, cobra.ShellCompDirectiveNoFileComp
	}
}

func parsePlanSpec(value string) (bool, error) {
	switch value {
	case "yes":
		return true, nil
	case "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid --plan %q: want \"yes\" or \"no\"", value)
	}
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
	sess.Status = "failed"
	sess.ProcessPID = 0
	sess.LastActiveAt = time.Now()
	sess.ResumeFailures++
	_ = store.Save(sess)
}

func resumeState(sess *subagent.Session) string {
	if sess.LastError != "" {
		return "failed"
	}
	d := agentDriver(subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier}, sess.WorkingDir)
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
		if have != want || !subagent.CanManage(callerParent, sess) || resumeState(sess) == "terminal" || sess.ResumeBlockedReason != "" || (sess.Provider == "codex" && sess.CodexQuarantine != nil) {
			continue
		}
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

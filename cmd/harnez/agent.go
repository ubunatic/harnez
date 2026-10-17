package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"ubunatic.com/harnez/internal/agentpolicy"
	"ubunatic.com/harnez/internal/privacy"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/telemetry"
)

var agentDriver = subagent.DriverFor

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
	// Resumed is false (and set) only when a resume found its provider
	// session gone and started a new one; PreviousSessionID and Note say which and why.
	Resumed           *bool  `json:"resumed,omitempty"`
	PreviousSessionID string `json:"previous_session_id,omitempty"`
	Note              string `json:"note,omitempty"`
}

// MarshalJSON keeps the session's own JSON (its MarshalJSON is promoted and
// would hide the fields above) and appends the fresh-session fields when set.
func (o agentOutput) MarshalJSON() ([]byte, error) {
	var base []byte
	var err error
	if o.Session != nil {
		base, err = o.Session.MarshalJSON()
	} else {
		base, err = json.Marshal(struct{}{})
	}
	if err != nil || o.Resumed == nil {
		return base, err
	}
	extra, err := json.Marshal(struct {
		Resumed           *bool  `json:"resumed"`
		PreviousSessionID string `json:"previous_session_id,omitempty"`
		Note              string `json:"note,omitempty"`
	}{o.Resumed, o.PreviousSessionID, o.Note})
	if err != nil {
		return nil, err
	}
	base = bytes.TrimRight(base, " \n")
	if len(base) > 2 {
		return append(append(base[:len(base)-1], ','), extra[1:]...), nil
	}
	return extra, nil
}

func newAgentCmd() *cobra.Command {
	ac := &agentCLI{}
	var rootFiles []string
	var rootContinue bool
	roleHelp, _ := subagent.RoleHelp()
	roleUsage := "agent role for a new session (default from spec/agent.yaml)"
	if roleHelp != "" {
		roleUsage += "; roles:\n" + roleHelp
	}
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
The model default comes from spec/agent.yaml. Agents run with -d <dir> as their
working directory (default: cwd); resume reuses the session's directory.
Sessions are chosen by --name, attribution in -d, or -c. Use -- to send text
literally. Slash commands are /compact, /stop, and /status. Automatic compaction uses
agent.compact_threshold_tokens from ~/.harnez/config.yaml (default from
spec/agent.yaml compact_threshold_tokens).
agent.compact_thresholds may override provider:model[:tier] thresholds.`}
	root.PersistentFlags().StringVar(&ac.storeDir, "store-dir", subagent.DefaultStoreDir(), "session store directory")
	root.PersistentFlags().StringVarP(&ac.workDir, "dir", "d", ".", "agent working directory (all providers) and session scope")
	root.PersistentFlags().StringVar(&ac.name, "name", "", "session name")
	root.PersistentFlags().StringVar(&ac.modelSpec, "model", "", "provider:model[:tier]")
	root.PersistentFlags().BoolVar(&ac.allowExhaustedQuota, "allow-exhausted-quota", false, "start even when cached provider quota is exhausted")
	root.PersistentFlags().StringVar(&ac.roleSpec, "role", "", roleUsage)
	root.PersistentFlags().DurationVar(&ac.agentTimeout, "timeout", 0, "maximum foreground turn duration (0 waits without a limit)")
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
	root.Flags().StringVarP(&ac.rootPrompt, "prompt", "p", "", "prompt text")
	root.Flags().BoolVarP(&rootContinue, "continue", "c", false, "resume the most recently active attributable session")
	root.Flags().StringSliceVarP(&rootFiles, "file", "f", nil, "prompt file (repeatable; - reads stdin)")
	root.Flags().StringVar(&ac.streamMode, "stream", streamFull, "live output: full (all messages) or stats (heartbeats and final reply only)")
	root.Flags().BoolVar(&ac.jsonOut, "json", false, "JSON output")
	root.Flags().StringVar(&ac.planSpec, "plan", "no", "planning gate: yes or no")
	ac.store = func() (*subagent.FileSessionStore, error) { return subagent.NewSessionStore(ac.storeDir) }
	ac.parent = currentHostSession
	_ = root.RegisterFlagCompletionFunc("name", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return agentSessionCompletion(ac.storeDir, ac.parent)(cmd, args, toComplete)
	})
	_ = root.RegisterFlagCompletionFunc("model", agentModelCompletion)
	_ = root.RegisterFlagCompletionFunc("plan", flagValueCompletion("yes", "no"))
	_ = root.RegisterFlagCompletionFunc("stream", flagValueCompletion(streamFull, streamStats))
	ac.find = func(cmd *cobra.Command, s *subagent.FileSessionStore, id string) (*subagent.Session, error) {
		dir := ""
		if cmd.Flags().Changed("dir") {
			dir = ac.workDir
		}
		return resolveSession(s, id, dir)
	}
	root.RunE = func(cmd *cobra.Command, args []string) error {
		planFirst, err := parsePlanSpec(ac.planSpec)
		if err != nil {
			return err
		}
		if ac.workDir == "" {
			ac.workDir = "."
		}
		dash := cmd.Flags().ArgsLenAtDash()
		words, tail := promptArgs(args, dash)
		if ac.rootPrompt == "" && !rootContinue && dash < 0 && len(words) == 1 && !strings.ContainsAny(words[0], " \t\r\n") {
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
		if ac.rootPrompt == "" && len(rootFiles) == 0 && len(words) == 0 && len(tail) == 0 && !rootContinue {
			return cmd.Help()
		}
		if ac.rootPrompt != "" && strings.HasPrefix(ac.rootPrompt, "--") {
			return fmt.Errorf("-p needs prompt text but got flag %q; put -p last or use -- \"<text>\"", ac.rootPrompt)
		}
		if ac.modelSpec == "" && len(words) >= 2 && oldStyleModelWord(words[0]) {
			return fmt.Errorf("model is now --model <spec>; to send this text literally put it after --")
		}
		promptWords := words
		if ac.rootPrompt != "" {
			promptWords = append([]string{ac.rootPrompt}, promptWords...)
		}
		prompt, err := assemblePrompt(rootFiles, promptWords, tail, cmd.InOrStdin())
		if err != nil {
			return err
		}
		deps := agentDeps{store: ac.store, parent: ac.parent, find: ac.find, storeDir: ac.storeDir, dbPath: agentUsageDBPath()}
		if rootContinue && ac.name != "" {
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
			s, e := ac.store()
			if e != nil {
				return e
			}
			x, _, e := resolveResumeSession(cmd, deps, s, resumeRequest{Role: ac.roleSpec, Dir: ac.workDir, Name: ac.name, Continue: rootContinue})
			if e != nil {
				return e
			}
			switch trimmed {
			case "/compact":
				return compactSession(cmd, s, x)
			case "/stop":
				return stopSession(cmd, s, x)
			default: // "/status"
				return statusSession(cmd, x, ac.jsonOut, false, 0, false)
			}
		}
		if ac.name != "" {
			s, e := ac.store()
			if e != nil {
				return e
			}
			if _, findErr := ac.find(cmd, s, ac.name); findErr == nil {
				return withAgentTimeout(cmd, ac.agentTimeout, func(ctx context.Context) error {
					return runResume(ctx, cmd, deps, resumeRequest{Role: ac.roleSpec, Prompt: prompt, Name: ac.name, ModelSpec: ac.modelSpec, Dir: ac.workDir, StreamMode: ac.streamMode, JSON: ac.jsonOut, PlanFirst: planFirst})
				})
			} else if !strings.Contains(findErr.Error(), "not found") {
				return findErr
			}
			return withAgentTimeout(cmd, ac.agentTimeout, func(ctx context.Context) error {
				return runStart(ctx, cmd, deps, startRequest{Role: ac.roleSpec, Prompt: prompt, StoredPrompt: promptStorage(rootFiles, promptWords, tail, prompt), Name: ac.name, ModelSpec: ac.modelSpec, Dir: ac.workDir, StreamMode: ac.streamMode, JSON: ac.jsonOut, PlanFirst: planFirst, AllowExhaustedQuota: ac.allowExhaustedQuota})
			})
		}
		if rootContinue {
			s, e := ac.store()
			if e != nil {
				return e
			}
			xs, e := s.List("", true)
			if e != nil {
				return e
			}
			candidates := attributable(xs, ac.workDir, ac.parent())
			if len(candidates) > 0 {
				return withAgentTimeout(cmd, ac.agentTimeout, func(ctx context.Context) error {
					return runResume(ctx, cmd, deps, resumeRequest{Role: ac.roleSpec, Prompt: prompt, ModelSpec: ac.modelSpec, Dir: ac.workDir, StreamMode: ac.streamMode, Continue: true, JSON: ac.jsonOut, PlanFirst: planFirst})
				})
			}
		}
		return withAgentTimeout(cmd, ac.agentTimeout, func(ctx context.Context) error {
			return runStart(ctx, cmd, deps, startRequest{Role: ac.roleSpec, Prompt: prompt, StoredPrompt: promptStorage(rootFiles, promptWords, tail, prompt), ModelSpec: ac.modelSpec, Dir: ac.workDir, StreamMode: ac.streamMode, JSON: ac.jsonOut, PlanFirst: planFirst, AllowExhaustedQuota: ac.allowExhaustedQuota})
		})
	}

	start := ac.startCmd()
	models := ac.modelsCmd()
	resume := ac.resumeCmd()
	list := ac.listCmd()
	status := ac.statusCmd()
	wait := ac.waitCmd()
	ac.addPolicyCmds(root)
	compact := ac.compactCmd()
	stop := ac.stopCmd()
	remove := ac.deleteCmd()
	rate := ac.rateCmd()
	root.AddCommand(start, models, resume, list, status, wait, compact, stop, remove, rate, newAgentSelftestCmd(), newAgentSendCmd(&ac.storeDir, &ac.name, &ac.workDir))
	silenceUsage(root)
	return root
}

func countVisibleAgentSessions(sessions []*subagent.Session, includeDead bool) int {
	if includeDead {
		return len(sessions)
	}
	count := 0
	for _, sess := range sessions {
		if sess.Status == "running" || sess.Status == "active" {
			count++
		}
	}
	return count
}

func formatAgentContextTokens(tokens int) string {
	if tokens <= 0 {
		return "-"
	}
	return formatAgentTokenCount(tokens)
}

func formatAgentSessionUsedTokens(tokens int, known bool) string {
	if !known || tokens <= 0 {
		return "-"
	}
	return formatAgentTokenCount(tokens)
}

func formatAgentTokenCount(tokens int) string {
	switch {
	case tokens >= 10_000_000:
		return fmt.Sprintf("%.0fM", float64(tokens)/1_000_000)
	case tokens >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(tokens)/1_000_000)
	case tokens >= 100_000:
		return fmt.Sprintf("%.0fk", float64(tokens)/1_000)
	case tokens >= 10_000:
		return fmt.Sprintf("%.1fk", float64(tokens)/1_000)
	default:
		return strconv.Itoa(tokens)
	}
}

func agentListColorEnabled(mode string, out io.Writer) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "always":
		return true, nil
	case "never":
		return false, nil
	case "", "auto":
		if os.Getenv("NO_COLOR") != "" {
			return false, nil
		}
		file, ok := out.(*os.File)
		return ok && term.IsTerminal(int(file.Fd())), nil
	default:
		return false, fmt.Errorf("invalid --color %q: choose auto, always, or never", mode)
	}
}

func agentListColor(code, text string, enabled bool) string {
	if !enabled {
		return text
	}
	return code + text + "\x1b[0m"
}

func agentListStatusColor(status string, enabled bool) string {
	if !enabled {
		return status
	}
	switch status {
	case "active", "running":
		return agentListColor("\x1b[32m", status, true)
	case "completed", "available":
		return agentListColor("\x1b[36m", status, true)
	case "failed", "delete-failed":
		return agentListColor("\x1b[31m", status, true)
	default:
		return status
	}
}

func appendExternalAgentSessions(listed, managed, external, deleted []*subagent.Session) []*subagent.Session {
	seen := make(map[string]struct{}, len(managed)+len(external)+len(deleted))
	byKey := make(map[string]*subagent.Session, len(managed)+len(external)+len(deleted))
	for _, sess := range managed {
		if sess == nil {
			continue
		}
		key := sess.Provider + "\x00" + sess.ProviderID()
		seen[key] = struct{}{}
		byKey[key] = sess
	}
	for _, sess := range deleted {
		if sess != nil {
			seen[sess.Provider+"\x00"+sess.ProviderID()] = struct{}{}
		}
	}
	for _, sess := range listed {
		if sess == nil {
			continue
		}
		seen[sess.Provider+"\x00"+sess.ProviderID()] = struct{}{}
	}
	for _, sess := range external {
		if sess == nil {
			continue
		}
		key := sess.Provider + "\x00" + sess.ProviderID()
		if _, ok := seen[key]; ok {
			if existing := byKey[key]; existing != nil {
				if sess.ContextTokens != 0 {
					existing.ContextTokens = sess.ContextTokens
				}
				if sess.ContextWindowSize > 0 {
					existing.ContextWindowSize = sess.ContextWindowSize
				}
				if sess.SessionUsedKnown {
					existing.SessionUsedTokens = sess.SessionUsedTokens
					existing.SessionUsedKnown = true
				}
			}
			continue
		}
		seen[key] = struct{}{}
		byKey[key] = sess
		listed = append(listed, sess)
	}
	return listed
}

func enrichExternalAgentTokenCounts(sessions []*subagent.Session) {
	if len(sessions) == 0 {
		return
	}
	db, err := telemetry.OpenReadOnly(agentUsageDBPath())
	if err != nil {
		return
	}
	defer db.Close()

	ids := make([]string, 0, len(sessions))
	for _, session := range sessions {
		if session != nil && session.Provider == "codex" {
			ids = append(ids, session.ProviderID())
		}
	}
	snapshots, err := db.LatestTokenSnapshots(ids)
	if err != nil {
		return
	}
	for _, session := range sessions {
		if session == nil || session.Provider != "codex" {
			continue
		}
		snapshot, ok := snapshots[session.ProviderID()]
		if !ok {
			continue
		}
		if snapshot.TotalTokens != nil {
			session.TokensCumulative = int(*snapshot.TotalTokens)
		} else if snapshot.LastTotalTokens != nil {
			session.TokensCumulative = int(*snapshot.LastTotalTokens)
		}
		if snapshot.CachedInputTokens != nil {
			session.CachedTokens = int(*snapshot.CachedInputTokens)
		} else if snapshot.LastCachedInputTokens != nil {
			session.CachedTokens = int(*snapshot.LastCachedInputTokens)
		}
	}
}

func matchResumeSelector(store *subagent.FileSessionStore, selector string) (bool, error) {
	sessions, err := store.List("", true)
	if err != nil {
		return false, err
	}
	_, ok, err := findResumeSelector(sessions, selector)
	return ok, err
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
			if !canManageTarget(parent(), session) || !strings.HasPrefix(session.Name, toComplete) {
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

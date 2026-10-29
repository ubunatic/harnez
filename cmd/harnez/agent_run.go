package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/agent"
	"ubunatic.com/harnez/internal/procs"
	"ubunatic.com/harnez/internal/quota1"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/internal/usagestore"
)

// This file holds the turn logic behind `agent start` and `agent resume`.
// runStart and runResume read only their request and deps, never cobra flag
// values, so every entry point (the verbs and the runnable `agent` root form)
// shares one implementation.

// agentDeps are the session-store and caller-identity hooks the runners need.
type agentDeps struct {
	store        func() (*subagent.FileSessionStore, error)
	storeDir     string
	dbPath       string
	parent       func() string
	find         func(*cobra.Command, *subagent.FileSessionStore, string) (*subagent.Session, error)
	quota        func(context.Context, string, bool) usage.TurnQuotaReading
	availability func(string, string) usage.ProviderQuotaAvailability
	preflight    func(context.Context) error
	modelEnabled func(string, string, string) (bool, error)
}

func sessionModelEnabled(sess *subagent.Session, check func(string, string, string) (bool, error)) (bool, error) {
	if check == nil {
		check = subagent.ModelEnabled
	}
	return check(sess.Provider, sess.Model, sess.Tier)
}

func warnDisabledSessionModel(cmd *cobra.Command, sess *subagent.Session, check func(string, string, string) (bool, error)) error {
	enabled, err := sessionModelEnabled(sess, check)
	if err != nil {
		return err
	}
	if !enabled {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: session %q uses model %s disabled by current policy; continuing the existing session.\n", sess.Name, subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier}.Spec())
	}
	return nil
}

func preflightCodex(ctx context.Context, provider string, driver subagent.Driver, check func(context.Context) error) error {
	if provider != "codex" {
		return nil
	}
	if check == nil {
		if _, ok := driver.(subagent.CodexDriver); !ok {
			return nil
		}
		check = subagent.CheckCodexAuth
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return check(ctx)
}

func recordTurnQuota(dbPath string, capture func(context.Context, string, bool) usage.TurnQuotaReading, sessionID, provider string, turn int, boundary string, force bool, turnStarted, baselineCacheAt time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), usage.TurnQuotaTimeoutForProvider(provider))
	defer cancel()
	var reading usage.TurnQuotaReading
	if capture != nil {
		reading = capture(ctx, provider, force)
	} else {
		reading = agentTurnQuotaReader(ctx, provider, force, turnStarted, baselineCacheAt)
	}
	stored := usage.TurnQuotaBoundaryFromReading(sessionID, provider, turn, boundary, reading)
	if dbPath != "" {
		_ = usage.PersistTurnQuotaBoundary(context.Background(), dbPath, stored)
	}
}

func captureTurnQuota(parent context.Context, capture func(context.Context, string, bool) usage.TurnQuotaReading, provider string, force bool, turnStarted time.Time) usage.TurnQuotaReading {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, usage.TurnQuotaTimeoutForProvider(provider))
	defer cancel()
	if capture != nil {
		return capture(ctx, provider, force)
	}
	return agentTurnQuotaReader(ctx, provider, force, turnStarted, time.Time{})
}

func storeTurnQuota(dbPath, sessionID, provider string, turn int, boundary string, reading usage.TurnQuotaReading) {
	stored := usage.TurnQuotaBoundaryFromReading(sessionID, provider, turn, boundary, reading)
	if dbPath != "" {
		_ = usage.PersistTurnQuotaBoundary(context.Background(), dbPath, stored)
	}
}

func persistAgentTurnTokens(dbPath string, session *subagent.Session, result *subagent.TurnResult, observedAt time.Time) {
	if session == nil || result == nil || dbPath == "" {
		return
	}
	input, cached, output := int64(result.InputTokens), int64(result.CachedTokens), int64(result.OutputTokens)
	token := usagestore.TurnTokenUsage{
		SessionID: session.ID, Turn: session.Turn, Provider: session.Provider, CounterKind: "delta",
		InputTokens: &input, CachedInputTokens: &cached, OutputTokens: &output,
		InputQuality: "measured", CachedInputQuality: "measured", OutputQuality: "measured", ReasoningQuality: "unknown",
		ObservedAt: observedAt,
	}
	if result.ReasoningTokensKnown {
		reasoning := int64(result.ReasoningTokens)
		token.ReasoningTokens = &reasoning
		token.ReasoningQuality = "measured"
	}
	_ = usage.PersistTurnTokenUsage(context.Background(), dbPath, token)
	cumulative := usagestore.TurnTokenUsage{
		SessionID: session.ID, Turn: session.Turn, Provider: session.Provider, CounterKind: "cumulative",
		InputQuality: "measured", CachedInputQuality: "measured", OutputQuality: "measured", ReasoningQuality: "unknown",
		ObservedAt: observedAt,
	}
	cumulativeInput, cumulativeCached, cumulativeOutput := int64(session.InputTokensTotal), int64(session.CachedTokensTotal), int64(session.OutputTokensTotal)
	cumulative.InputTokens, cumulative.CachedInputTokens, cumulative.OutputTokens = &cumulativeInput, &cumulativeCached, &cumulativeOutput
	if session.ReasoningTokensKnown {
		reasoning := int64(session.ReasoningTokensTotal)
		cumulative.ReasoningTokens = &reasoning
		cumulative.ReasoningQuality = "measured"
	}
	_ = usage.PersistTurnTokenUsage(context.Background(), dbPath, cumulative)
}

func agentCommandContext(cmd *cobra.Command) context.Context {
	if cmd == nil || cmd.Context() == nil {
		return context.Background()
	}
	return cmd.Context()
}

// withAgentTimeout hands run the command context, bounded by --timeout when
// set. It never mutates cmd's context: goroutines started by run may still
// read their ctx after run returns, and cmd.SetContext would race them.
func withAgentTimeout(cmd *cobra.Command, timeout time.Duration, run func(context.Context) error) error {
	ctx := agentCommandContext(cmd)
	if timeout <= 0 || cmd == nil || (!cmd.Flags().Changed("timeout") && !cmd.InheritedFlags().Changed("timeout")) {
		return run(ctx)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return run(ctx)
}

func turnQuotaBaseline(turnStarted time.Time, before usage.TurnQuotaReading) time.Time {
	if !before.HasCache || before.CacheAgeMS >= int64(usage.MinWatchInterval/time.Millisecond) {
		return time.Time{}
	}
	if !before.CapturedAt.IsZero() {
		return before.CapturedAt.Add(-time.Duration(before.CacheAgeMS) * time.Millisecond)
	}
	return turnStarted.Add(-time.Duration(before.CacheAgeMS) * time.Millisecond)
}

func warnQuota1Changes(cmd *cobra.Command, dir string, turnStarted time.Time) {
	files, since, err := quota1.ChangesSinceLastRunAfter(dir, turnStarted)
	if err != nil || len(files) == 0 {
		return
	}
	const shown = 5
	names := files
	if len(names) > shown {
		names = names[:shown]
	}
	for i := range names {
		names[i], _ = filepath.Rel(dir, names[i])
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "harnez: %d file(s) changed after the last quota-1 run at %s: %s — code is untested, run make test-q1\n", len(files), since.Format(time.RFC3339), strings.Join(names, ", "))
}

// startRequest describes one new agent turn. Prompt is sent to the agent;
// StoredPrompt is what the session records (files as "path (N bytes)").
type startRequest struct {
	Prompt, StoredPrompt, Name, ModelSpec, Dir, StreamMode, Role string
	JSON, PlanFirst, AllowExhaustedQuota                         bool
	SessionID                                                    string
}

func providerAvailability(provider, model string) usage.ProviderQuotaAvailability {
	return usage.CachedProviderQuotaAvailability(provider, model, time.Now())
}

func quotaAgeLabel(age time.Duration) string {
	return agent.QuotaAgeLabel(age)
}

func rejectExhaustedQuota(spec string, model subagent.Model, override bool, availability func(string, string) usage.ProviderQuotaAvailability) error {
	return agent.RejectExhaustedQuota(spec, model, override, availability)
}

func selectStartModel(role, spec string, override bool, availability func(string, string) usage.ProviderQuotaAvailability, stderr io.Writer) (subagent.Model, error) {
	selection, err := selectStartSelection(role, spec, override, availability, stderr)
	return selection.Model, err
}

func selectStartSelection(role, spec string, override bool, availability func(string, string) usage.ProviderQuotaAvailability, stderr io.Writer) (subagent.ModelSelection, error) {
	if availability == nil {
		availability = providerAvailability
	}
	selection, err := subagent.ResolveModelWithAvailability(role, spec, func(m subagent.Model) bool { return availability(m.Provider, m.Name).State == "exhausted" }, override)
	if err != nil {
		return selection, err
	}
	if selection.Trait == "" {
		if err := rejectExhaustedQuota(spec, selection.Model, override, availability); err != nil {
			return subagent.ModelSelection{}, err
		}
	} else if len(selection.Skipped) > 0 {
		fmt.Fprintf(stderr, "%s → %s (%s)\n", selection.Trait, selection.Target, strings.Join(selection.Skipped, "; "))
	}
	return selection, nil
}

func startModelError(spec string, selection subagent.ModelSelection, err error) error {
	if selection.Trait != "" {
		return fmt.Errorf("agent start %q rejected: %w", spec, err)
	}
	return fmt.Errorf("agent start %q rejected: %w; ask for guidance rather than using a different model", spec, err)
}

func workerSelection(sess *subagent.Session, spec string) (subagent.ModelSelection, error) {
	model := subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier}
	if model.Spec() != spec {
		return subagent.ModelSelection{}, fmt.Errorf("worker model %q conflicts with recorded model %q", spec, model.Spec())
	}
	return subagent.ModelSelection{Model: model, Trait: sess.ModelAlias}, nil
}

func checkResumeModel(role, spec string, sess *subagent.Session, availability func(string, string) usage.ProviderQuotaAvailability, stderr io.Writer) error {
	selection, err := selectStartSelection(role, spec, false, availability, stderr)
	if err != nil || selection.Model.Provider != sess.Provider || selection.Model.Name != sess.Model || selection.Model.Tier != sess.Tier {
		return fmt.Errorf("--model %q conflicts with session %q model %s:%s:%s", spec, sess.Name, sess.Provider, sess.Model, sess.Tier)
	}
	return nil
}

func isCodexUsageLimitError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "usage limit") || strings.Contains(message, "usage_limit")
}

func recordCodexResumeFailure(sess *subagent.Session, err error, quotaState string, now time.Time) bool {
	if sess.Provider != "codex" || !isCodexUsageLimitError(err) {
		return false
	}
	if sess.CodexQuotaResumePending && quotaState == "available" {
		sess.CodexQuarantine = &subagent.CodexQuarantine{Reason: firstLine(err.Error()), At: now.UTC(), Attempts: 1}
		sess.CodexQuotaResumePending = false
		return true
	}
	sess.CodexQuotaResumePending = true
	return true
}

func shortCause(err error) string {
	if err == nil {
		return "provider failed"
	}
	const maxCause = 180
	for _, line := range strings.Split(err.Error(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		runes := []rune(line)
		if len(runes) > maxCause {
			line = string(runes[:maxCause-3]) + "..."
		}
		return line
	}
	return "provider failed"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func quotaAlternatives(selected subagent.Model, availability func(string, string) usage.ProviderQuotaAvailability) []string {
	return agent.QuotaAlternatives(selected, availability)
}

func quotaModelCost(model subagent.Model) int {
	for _, entry := range subagent.KnownModelEntries() {
		if entry.Model.Provider == model.Provider && entry.Model.Name == model.Name {
			return entry.Cost
		}
	}
	return 0
}

// resumeRequest describes one turn on an existing session: chosen by Name,
// else by attribution in Dir (Continue picks the most recent).
type resumeRequest struct {
	Role                                     string // must match the stored role when given
	Prompt, Name, ModelSpec, Dir, StreamMode string
	SessionID, Selector                      string
	Continue, JSON, PlanFirst                bool
}

// resolveResumeSession picks the session and reports how: name, id, dir or continue.
func resolveResumeSession(cmd *cobra.Command, d agentDeps, s *subagent.FileSessionStore, req resumeRequest) (*subagent.Session, string, error) {
	if req.Name != "" {
		x, e := d.find(cmd, s, req.Name)
		return x, "name", e
	}
	xs, e := s.List("", true)
	if e != nil {
		return nil, "", e
	}
	if req.Selector != "" {
		if selected, ok, matchErr := findResumeSelector(xs, req.Selector); matchErr != nil {
			return nil, "", matchErr
		} else if ok {
			return selected, "id", nil
		}
	}
	c := attributable(xs, req.Dir, d.parent())
	if len(c) == 0 {
		return nil, "", fmt.Errorf("no resumable agent in %s; start one with: harnez agent start --name <name> ...", req.Dir)
	}
	if req.Continue {
		return c[0], "continue", nil
	}
	return c[0], "dir", nil
}

func findResumeSelector(sessions []*subagent.Session, selector string) (*subagent.Session, bool, error) {
	return agent.FindResumeSelector(sessions, selector)
}

// runStart starts a new session, streams or prints the turn and saves the session.
// runStart runs one start turn under ctx. ctx is passed explicitly rather than
// stored on cmd so goroutines started here never race a cmd.SetContext.
func runStart(ctx context.Context, cmd *cobra.Command, d agentDeps, req startRequest) error {
	quotaTurnStarted := time.Now().Add(-time.Hour)
	spec := req.ModelSpec
	if err := checkStreamMode(req.StreamMode); err != nil {
		return err
	}
	role, err := startRole(req.Role, req.Prompt, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	var selection subagent.ModelSelection
	if req.SessionID != "" {
		store, storeErr := d.store()
		if storeErr != nil {
			return storeErr
		}
		sess, storeErr := store.Get(req.SessionID)
		if storeErr != nil {
			return storeErr
		}
		selection, err = workerSelection(sess, spec)
	} else {
		selection, err = selectStartSelection(role, spec, req.AllowExhaustedQuota, d.availability, cmd.ErrOrStderr())
	}
	if err != nil {
		return startModelError(spec, selection, err)
	}
	m := selection.Model
	if spec == "" {
		spec = m.Spec()
	}
	s, err := d.store()
	if err != nil {
		return err
	}
	id := req.SessionID
	if id == "" {
		id = uuid.NewString()
	}
	sessName := req.Name
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
	if req.Name != "" && req.SessionID == "" {
		if existing, findErr := resolveSession(s, req.Name, ""); findErr == nil {
			return fmt.Errorf("session name %q is already in use by %s in %s", req.Name, existing.ID, existing.WorkingDir)
		}
	}
	canonicalWorkDir, err := filepath.Abs(req.Dir)
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	autoApprove, err := subagent.RoleAutoApproveTools(role)
	if err != nil {
		return err
	}
	baseDriver := withAgyAutoApprove(agentDriver(m, canonicalWorkDir), autoApprove)
	if err := preflightCodex(ctx, m.Provider, baseDriver, d.preflight); err != nil {
		return fmt.Errorf("agent start %q preflight: %w", spec, err)
	}
	if m.Provider == "pi" || m.Provider == "lmcoder" {
		warnSmallPiWindow(cmd.ErrOrStderr(), m, piAgentDir())
	}
	tl := newTimeline(cmd)
	opts := subagent.RunOptions{Prompt: req.Prompt, Model: m, Dir: canonicalWorkDir}
	parentID := d.parent() // read before the child's environment replaces it
	defer setAgentEnv(role, sessName, spec)()
	initialSession := func(status string) *subagent.Session {
		now := time.Now()
		sessionID := id
		if req.SessionID == "" && sessionID == "" {
			sessionID = uuid.NewString()
		}
		return &subagent.Session{ID: sessionID, Name: sessName, StartPrompt: req.StoredPrompt, Provider: m.Provider, Model: m.Name, ModelAlias: selection.Trait, Tier: m.Tier, WorkingDir: canonicalWorkDir, ParentSessionID: parentID, CallerPID: os.Getpid(), HarnessType: "harnez", Status: status, Role: role, CreatedAt: now, LastActiveAt: now}
	}
	if req.SessionID == "" && d.storeDir != "" {
		if timeout := foregroundDetachTimeout(cmd); timeout > 0 {
			now := time.Now()
			sess := &subagent.Session{ID: id, Name: sessName, StartPrompt: req.StoredPrompt, Provider: m.Provider, Model: m.Name, ModelAlias: selection.Trait, Tier: m.Tier, WorkingDir: canonicalWorkDir, ParentSessionID: parentID, CallerPID: os.Getpid(), HarnessType: "harnez", Status: "running", Role: role, CreatedAt: now, LastActiveAt: now}
			sess.StdoutLog = filepath.Join(d.storeDir, id+".stdout.log")
			sess.StderrLog = filepath.Join(d.storeDir, id+".stderr.log")
			if err := s.Create(sess); err != nil {
				return err
			}
			workerSpec := m.Spec()
			args := []string{"--store-dir", d.storeDir, "agent", "start", "--worker-session", id, "--json", "--model", workerSpec, "--role", role, "--name", sessName, "--dir", canonicalWorkDir, "--plan", map[bool]string{true: "yes", false: "no"}[req.PlanFirst]}
			if req.AllowExhaustedQuota {
				args = append(args, "--allow-exhausted-quota")
			}
			args = append(args, "--", req.Prompt)
			started, err := launchForegroundWorker(cmd, s, sess, args, timeout, req.JSON)
			if !started && err != nil {
				sess.Status = "failed"
				sess.LastError = err.Error()
				_ = s.Save(sess)
			}
			return err
		}
	}
	if req.SessionID == "" {
		reserved := initialSession("running")
		reserved.ProcessPID = os.Getpid()
		reserved.ProcessStarttime = procs.ProcessStarttime(reserved.ProcessPID)
		if err := s.Create(reserved); err != nil {
			return err
		}
		id = reserved.ID
	}
	ctx = observeSessionProcess(ctx, s, id, nil)
	driver := withAgyMeterSession(baseDriver, id)
	sd, streaming := driver.(subagent.StreamingDriver)
	streaming = streaming && !req.JSON
	turn := 1
	turnStarted := time.Now().UTC()
	warnModelQuota(cmd, d.availability, m.Provider, m.Name, turnStarted.Local())
	beforeCapture, stopCapture := startTurnQuotaCapture(ctx, d.quota, m.Provider)
	defer stopCapture()
	var ts *turnStream
	var r *subagent.TurnResult
	observedSessionID := ""
	if streaming {
		ts = newTurnStream(cmd, req.StreamMode, false)
		ts.stopCmd, ts.name, ts.planFirst = "harnez agent stop "+sessName, sessName, req.PlanFirst
		opts.Prompt = withProtocol(req.Prompt, req.PlanFirst, role)
		ts.watch()
		watchCtx, cancelWatch := context.WithCancel(ctx)
		defer cancelWatch()
		threshold, thresholdErr := subagent.CompactThreshold(m)
		watchdog := &subagent.TokenWatchdog{Threshold: threshold}
		watchdogCrossed := false
		observedTokens := 0
		r, err = sd.RunStream(watchCtx, opts, func(ev subagent.Event) {
			if ev.Kind == "session" {
				observedSessionID = ev.Text
				if req.SessionID == "" {
					sess, getErr := s.Get(id)
					if getErr != nil {
						sess = initialSession("running")
						sess.ID = id
					}
					sess.ProviderSessionID = ev.Text
					sess.LastActiveAt = time.Now()
					_ = s.Save(sess)
				}
			}
			if thresholdErr == nil {
				if tokens, crossed := watchdog.Observe(ev); crossed {
					watchdogCrossed = true
					observedTokens = tokens
					cancelWatch()
				}
			}
			if ev.Kind == "session" {
				extra := []string{fmt.Sprintf("name=%s dir=%s", sessName, canonicalWorkDir)}
				modelLine := "model: " + m.Spec()
				if selection.Trait != "" {
					modelLine = "model: " + selection.Trait + " → " + m.Spec()
				}
				if req.ModelSpec == "" {
					modelLine += " (default)"
				}
				extra = append(extra, modelLine)
				ts.info(ev.Text, m.Provider+":"+m.Name, "start", "new", append(extra, "reconnect: harnez agent resume "+ev.Text+" \"<prompt>\"")...)
			}
			ts.onEvent(ev)
		})
		if err != nil {
			ts.abort()
			if watchdogCrossed {
				providerSessionID := observedSessionID
				if providerSessionID != "" {
					if req.SessionID != "" {
						id = req.SessionID
					}
					now := time.Now()
					sess, getErr := s.Get(id)
					if getErr != nil {
						sess = initialSession("stopped")
						id = sess.ID
					}
					sess.ProviderSessionID = providerSessionID
					sess.Status = "stopped"
					sess.ProcessPID, sess.ProcessStarttime = 0, 0
					sess.LastError = "runtime token watchdog interrupted the active turn"
					sess.LastActiveAt = now
					sess.Turn = turn
					if saveErr := s.Save(sess); saveErr != nil {
						return fmt.Errorf("agent start %q crossed live token threshold at %d tokens (limit %d); session recovery record failed: %w", spec, observedTokens, threshold, saveErr)
					}
				}
				return fmt.Errorf("agent start %q crossed live token threshold at %d tokens (limit %d); turn interrupted and session %q can be resumed", spec, observedTokens, threshold, sessName)
			}
		}
	} else {
		tl.announceTurn("start", m.Provider+":"+m.Name, sessName)
		r, err = driver.Run(ctx, opts)
	}
	if err != nil {
		before := <-beforeCapture
		baselineCacheAt := turnQuotaBaseline(turnStarted, before)
		storeTurnQuota(d.dbPath, id, m.Provider, turn, "before", before)
		recordTurnQuota(d.dbPath, d.quota, id, m.Provider, turn, "after", false, turnStarted, baselineCacheAt)
		if req.SessionID == "" {
			sess, getErr := s.Get(id)
			if getErr != nil {
				sess = initialSession("failed")
			}
			sess.ProviderSessionID = firstNonEmpty(sess.ProviderSessionID, observedSessionID)
			sess.Status = "failed"
			sess.ProcessPID, sess.ProcessStarttime = 0, 0
			sess.LastError = err.Error()
			sess.LastActiveAt = time.Now()
			if saveErr := s.Save(sess); saveErr != nil {
				return fmt.Errorf("agent start %q failed: %s; session recovery record failed: %w", spec, shortCause(err), saveErr)
			}
		}
		return fmt.Errorf("agent start %q failed: %s; verify the provider/model configuration or ask for guidance", spec, shortCause(err))
	}
	providerSessionID := r.SessionID
	reservedID := id
	if r.SessionID != "" && req.SessionID == "" {
		id = r.SessionID
	}
	before := <-beforeCapture
	baselineCacheAt := turnQuotaBaseline(turnStarted, before)
	storeTurnQuota(d.dbPath, id, m.Provider, turn, "before", before)
	recordTurnQuota(d.dbPath, d.quota, id, m.Provider, turn, "after", false, turnStarted, baselineCacheAt)
	now := time.Now()
	sess := &subagent.Session{ID: id, ProviderSessionID: providerSessionID, Name: sessName, StartPrompt: req.StoredPrompt, Role: role, Provider: m.Provider, Model: m.Name, ModelAlias: selection.Trait, Tier: m.Tier, WorkingDir: canonicalWorkDir, ParentSessionID: parentID, CallerPID: os.Getpid(), HarnessType: "harnez", Status: "completed", Response: r.Response, Messages: r.Messages, TokensCumulative: r.TokensCumulative, InputTokensTotal: r.InputTokens, CachedTokensTotal: r.CachedTokens, OutputTokensTotal: r.OutputTokens, ReasoningTokensTotal: r.ReasoningTokens, ReasoningTokensKnown: r.ReasoningTokensKnown, TokenTotalsKnown: true, TokensSinceCompact: subagent.CompactionTokens(r), ContextTokens: r.ContextTokens, TokensTurn: r.TokensTurn, CachedTokens: r.CachedTokens, CreatedAt: now, LastActiveAt: now, Turn: turn, TurnRecords: []subagent.TurnRecord{{Turn: turn, NewInputTokens: subagent.CompactionTokens(r), CachedInputTokens: r.CachedTokens, OutputTokens: r.OutputTokens}}}
	sess.ContextWindowSize = r.ContextWindowSize
	persistAgentTurnTokens(d.dbPath, sess, r, now)
	if current, getErr := s.Get(reservedID); getErr == nil {
		sess.ProviderPID, sess.ProviderStarttime = current.ProviderPID, current.ProviderStarttime
	}
	if req.SessionID != "" {
		if current, getErr := s.Get(req.SessionID); getErr == nil {
			sess.ProcessPID = current.ProcessPID
			sess.ProcessStarttime = current.ProcessStarttime
			sess.StdoutLog = current.StdoutLog
			sess.StderrLog = current.StderrLog
			sess.CallerPID = current.CallerPID
			sess.CreatedAt = current.CreatedAt
			sess.ReplyHistory = current.ReplyHistory
		}
	}
	if err := recordAgentReply(sess, turn, r.Response, r.Messages, now); err != nil {
		return err
	}
	if err := s.Save(sess); err != nil {
		return err
	}
	if req.SessionID == "" && reservedID != id {
		_ = s.Delete(reservedID)
	}
	out := agentOutput{Session: sess, Response: r.Response, Messages: r.Messages, ReconnectCmd: "harnez agent resume " + id + " \"<prompt>\""}
	if streaming {
		ts.finish(r)
		warnQuota1Changes(cmd, canonicalWorkDir, quotaTurnStarted)
		return nil
	}
	tl.finishTurn(r, id, out.ReconnectCmd)
	if req.JSON {
		err := json.NewEncoder(cmd.OutOrStdout()).Encode(out)
		warnQuota1Changes(cmd, canonicalWorkDir, quotaTurnStarted)
		return err
	}
	err = printAgentMessages(cmd, r.Messages, r.Response)
	warnQuota1Changes(cmd, canonicalWorkDir, quotaTurnStarted)
	return err
}

// runResume runs one more turn on an existing session.
// runResume runs one resume turn under ctx; see runStart for why ctx is explicit.
func runResume(ctx context.Context, cmd *cobra.Command, d agentDeps, req resumeRequest) error {
	quotaTurnStarted := time.Now().Add(-time.Hour)

	sessionName := req.Name
	resolved := "name"
	var sess *subagent.Session
	if req.Continue && sessionName != "" {
		return fmt.Errorf("resume: --continue cannot be combined with --name")
	}
	if err := checkStreamMode(req.StreamMode); err != nil {
		return err
	}
	s, err := d.store()
	if err != nil {
		return err
	}
	sess, resolved, err = resolveResumeSession(cmd, d, s, req)
	if err != nil {
		return err
	}
	resumeIdle := time.Duration(0)
	if err := warnDisabledSessionModel(cmd, sess, d.modelEnabled); err != nil {
		return err
	}
	if !sess.LastActiveAt.IsZero() {
		resumeIdle = time.Since(sess.LastActiveAt)
	}
	if sess.Provider == "codex" && sess.HarnessType == "harnez" {
		modelSpec := fmt.Sprintf("%s:%s", sess.Provider, sess.Model)
		if sess.Tier != "" {
			modelSpec += ":" + sess.Tier
		}
		if _, err := subagent.ResolveModel(modelSpec); err != nil {
			if enabled, enabledErr := sessionModelEnabled(sess, d.modelEnabled); enabledErr == nil && !enabled {
				// An in-flight session remains resumable after policy changes.
			} else {
				return fmt.Errorf("session %q uses unavailable model %s; start a new session with harnez agent start: %w", sess.Name, modelSpec, err)
			}
		}
	}
	if req.SessionID == "" && (sess.HarnessType != "interactive" || sess.Status != "active") {
		writers, err := sessionWriters(sess)
		if err != nil {
			return fmt.Errorf("check session writer: %w", err)
		}
		if len(writers) > 0 {
			return fmt.Errorf("session %q already has a resume turn running (PID %d); writer is still alive; stop it or run `harnez agent wait --name %s` before resuming", sess.Name, writers[0].PID, sess.Name)
		}
	}
	if sess.Status == "running" && req.SessionID == "" {
		sess.Status = "failed"
		sess.ProcessPID = 0
		sess.LastError = "previous resume process is no longer running; treating session as stale"
		sess.LastActiveAt = time.Now()
		if err := s.Save(sess); err != nil {
			return fmt.Errorf("session %q has a stale running record, but could not update it: %w", sess.Name, err)
		}
	}
	if sess.ResumeBlockedReason != "" {
		return fmt.Errorf("session %q cannot be resumed: %s; start a fresh session with harnez agent start --name", sess.Name, sess.ResumeBlockedReason)
	}
	if sess.CodexQuarantine != nil {
		return fmt.Errorf("session %q is quarantined: %s; start a fresh session with harnez agent start --name", sess.Name, sess.CodexQuarantine.Reason)
	}
	ctx = observeSessionProcess(ctx, s, sess.ID, sess)
	if sess.Provider == "codex" && sess.CodexQuotaResumePending && d.availability != nil && d.availability(sess.Provider, sess.Model).State == "exhausted" {
		return fmt.Errorf("session %q is waiting for Codex quota recovery; resume once quota is available", sess.Name)
	}

	if !canManageTarget(d.parent(), sess) {
		return fmt.Errorf("session %q is outside caller lineage", sess.ID)
	}
	role, err := resumeRole(req.Role, sess)
	if err != nil {
		return err
	}
	if req.ModelSpec != "" {
		if err := checkResumeModel(role, req.ModelSpec, sess, d.availability, cmd.ErrOrStderr()); err != nil {
			return err
		}
	}
	defer setAgentEnv(role, sess.Name, subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier}.Spec())()
	interactive := sess.Status == "active" && sess.HarnessType == "interactive"
	if sess.HarnessType == "interactive" && sess.ProviderSessionID == "" {
		return fmt.Errorf("session %q cannot be resumed: %s did not expose a provider session ID", sess.Name, sess.Provider)
	}
	var driver subagent.Driver
	if !interactive {
		autoApprove, err := subagent.RoleAutoApproveTools(role)
		if err != nil {
			return err
		}
		baseDriver := withAgyAutoApprove(agentDriver(subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier}, sess.WorkingDir), autoApprove)
		if err := preflightCodex(ctx, sess.Provider, baseDriver, d.preflight); err != nil {
			return fmt.Errorf("agent resume %q preflight: %w", sess.Name, err)
		}
		driver = withAgyMeterSession(baseDriver, sess.ID)
		// CheckResumable cannot tell a gone session from a lookup in another
		// CODEX_HOME, so it refuses instead of starting fresh (issue 791).
		if checker, ok := driver.(subagent.ResumeChecker); ok {
			if resumable, reason := checker.CheckResumable(sess.ProviderID()); !resumable {
				return fmt.Errorf("session %q cannot be resumed: %s; start a new session with: harnez agent start --name <new-name> ...", sess.Name, reason)
			}
		}
	}
	tl := newTimeline(cmd)
	compacted, compactNote := false, ""
	threshold, err := subagent.CompactThreshold(subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier})
	if err != nil {
		return err
	}
	compactMode, err := subagent.AutoCompactMode(sess.Provider)
	if err != nil {
		return err
	}
	contextTokens := sess.LastContextTokens()
	if interactive && contextTokens >= threshold {
		return fmt.Errorf("session %q has %d context tokens, over the limit %d (setting agent.compact_threshold_tokens in ~/.harnez/config.yaml); an active interactive session cannot be compacted and verified from outside; next step: resume it non-interactively, or start a fresh session with harnez agent start", sess.Name, contextTokens, threshold)
	}
	compactFn := func() (*subagent.TurnResult, error) {
		if interactive {
			if err := subagent.SendControl(ctx, sess.ControlSocket, "compact", ""); err != nil {
				return nil, err
			}
			return nil, nil // interactive output has no verified completion/token signal
		}
		return driver.Compact(ctx, sess.ProviderID())
	}
	// Interactive sessions are guarded above; native compaction is verified after resume.
	if contextTokens >= 0 && (compactMode == "manual" || interactive) {
		compacted, err = subagent.EnsureContextUnderThreshold(contextTokens, threshold, compactFn)
		if err != nil {
			return fmt.Errorf("refusing to send resume prompt: %w", err)
		}
	}
	if compacted {
		compactNote = fmt.Sprintf("verified /compact at %s context tokens", humanCount(contextTokens))
		sess.TokensSinceCompact = 0
	}
	queuedSession, err := s.Get(sess.ID)
	if err != nil {
		return err
	}
	queuedMessages := queuedSession.PendingMessages
	req.Prompt = subagent.PrependQueuedMessages(queuedMessages, req.Prompt)
	if interactive {
		action := "prompt"
		if sess.Provider == "agy" {
			action = "enqueue"
		}
		if err := subagent.SendControl(ctx, sess.ControlSocket, action, req.Prompt); err != nil {
			return err
		}
		sess.LastActiveAt = time.Now()
		if err := s.SaveQueuedTurn(sess, queuedMessages); err != nil {
			return err
		}
		return writeAgentOutput(cmd, req.JSON, agentOutput{Session: sess, Response: "prompt delivered"})
	}
	var ts *turnStream
	var r *subagent.TurnResult
	_, streaming := driver.(subagent.StreamingDriver)
	streaming = streaming && !req.JSON
	turn := sess.Turn + 1
	model := subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier}
	// resumeTurn resumes the provider session natively (no history replay);
	// a verified stale one starts a new session with the same prompt (issue 791).
	previousID := sess.ProviderID()
	resumeTurn := subagent.ResumeTurn{Driver: driver, ID: previousID, Opts: subagent.RunOptions{Prompt: req.Prompt, Model: model, Dir: sess.WorkingDir}}
	resumed, freshNote := true, ""
	if req.SessionID == "" && d.storeDir != "" {
		if timeout := foregroundDetachTimeout(cmd); timeout > 0 {
			previous := *sess
			logID := uuid.NewString()
			sess.StdoutLog = filepath.Join(d.storeDir, sess.ID+".resume-"+logID+".stdout.log")
			sess.StderrLog = filepath.Join(d.storeDir, sess.ID+".resume-"+logID+".stderr.log")
			sess.Status = "running"
			if err := s.Save(sess); err != nil {
				return err
			}
			args := []string{"--store-dir", d.storeDir, "agent", "resume", "--worker-session", sess.ID, "--json", "--name", sess.Name, "--role", role, "--dir", sess.WorkingDir, "--plan", map[bool]string{true: "yes", false: "no"}[req.PlanFirst], "--", req.Prompt}
			started, err := launchForegroundWorker(cmd, s, sess, args, timeout, req.JSON)
			if !started && err != nil {
				*sess = previous
				_ = s.Save(sess)
			}
			return err
		}
	}
	parentID := d.parent()
	if parentID != "" && sess.ParentSessionID != parentID {
		sess.LastHostSessionID = parentID
	}
	if req.SessionID == "" {
		sess.Status = "running"
		sess.ProcessPID = os.Getpid()
		sess.ProcessStarttime = procs.ProcessStarttime(sess.ProcessPID)
		if err := s.Save(sess); err != nil {
			return fmt.Errorf("could not persist running state for session %q: %w", sess.Name, err)
		}
	}
	resumeTurn.Opts.Prompt = withResumeNote(req.Prompt, resumeIdle)
	turnStarted := time.Now().UTC()
	warnModelQuota(cmd, d.availability, sess.Provider, sess.Model, turnStarted.Local())
	beforeCapture, stopCapture := startTurnQuotaCapture(ctx, d.quota, sess.Provider)
	defer stopCapture()
	if streaming {
		ts = newTurnStream(cmd, req.StreamMode, compacted)
		ts.info(sess.ID, sess.Provider+":"+sess.Model, "resume", resolved)
		if compacted {
			ts.printf("[compact: %s]\n", compactNote)
		}
		ts.stopCmd, ts.name, ts.planFirst = "harnez agent stop "+sess.Name, sess.Name, req.PlanFirst
		ts.watch()
		watchCtx, cancelWatch := context.WithCancel(ctx)
		defer cancelWatch()
		watchdog := &subagent.TokenWatchdog{Threshold: threshold}
		watchdogCrossed := false
		observedTokens := 0
		resumeTurn.Opts.Prompt = withResumeNote(withProtocol(req.Prompt, req.PlanFirst, role), resumeIdle)
		resumeTurn.OnFresh = func(note string) { freshNote = note; ts.printf("[%s]\n", note) }
		resumeTurn.Events = func(ev subagent.Event) {
			if tokens, crossed := watchdog.Observe(ev); crossed {
				watchdogCrossed = true
				observedTokens = tokens
				cancelWatch()
			}
			ts.onEvent(ev)
		}
		r, resumed, err = resumeTurn.Do(watchCtx)
		if err != nil {
			ts.abort()
			if watchdogCrossed {
				sess.Status = "stopped"
				sess.ProcessPID = 0
				sess.LastError = fmt.Sprintf("runtime token watchdog interrupted the active turn at %d tokens (limit %d)", observedTokens, threshold)
				sess.LastActiveAt = time.Now()
				if saveErr := s.Save(sess); saveErr != nil {
					return fmt.Errorf("agent resume %q crossed live token threshold at %d tokens (limit %d); session recovery record failed: %w", sess.Name, observedTokens, threshold, saveErr)
				}
				return fmt.Errorf("agent resume %q crossed live token threshold at %d tokens (limit %d); turn interrupted and session can be resumed", sess.Name, observedTokens, threshold)
			}
		}
	} else {
		if compacted {
			tl.log("compact", "%s", compactNote)
		}
		tl.announceTurn("resume", sess.Provider+":"+sess.Model, sess.Name)
		resumeTurn.OnFresh = func(note string) { freshNote = note; tl.log("resume", "%s", note) }
		r, resumed, err = resumeTurn.Do(ctx)
	}
	if !resumed && r != nil && r.SessionID != "" {
		sess.ProviderSessionID = r.SessionID
	}
	if err != nil {
		before := <-beforeCapture
		baselineCacheAt := turnQuotaBaseline(turnStarted, before)
		storeTurnQuota(d.dbPath, sess.ID, sess.Provider, turn, "before", before)
		recordTurnQuota(d.dbPath, d.quota, sess.ID, sess.Provider, turn, "after", false, turnStarted, baselineCacheAt)
		recordResumeFailure(s, sess, err)
		quotaState := "unknown"
		if d.availability != nil {
			quotaState = d.availability(sess.Provider, sess.Model).State
		}
		if recordCodexResumeFailure(sess, err, quotaState, time.Now()) {
			_ = s.Save(sess)
		}
		return fmt.Errorf("agent resume %q (%s:%s:%s) failed: %w; verify the provider/model configuration or ask for guidance", sess.Name, sess.Provider, sess.Model, sess.Tier, err)
	}
	for _, warning := range r.Warnings {
		fmt.Fprintln(cmd.ErrOrStderr(), warning)
	}
	if compactMode == "native" && contextTokens >= threshold && (!r.CompactionObserved || r.ContextTokens < 0 || r.ContextTokens >= threshold) {
		context := fmt.Sprintf("%d", r.ContextTokens)
		if r.ContextTokens < 0 {
			context = "unknown"
		}
		reason := fmt.Sprintf("Codex did not auto-compact session %s: context %s, limit %d; start a fresh session with harnez agent start", sess.ProviderID(), context, threshold)
		sess.ResumeBlockedReason = reason
		sess.Status = "completed"
		sess.ProcessPID = 0
		sess.LastActiveAt = time.Now()
		if saveErr := s.Save(sess); saveErr != nil {
			return fmt.Errorf("%s (also failed to save resume block: %w)", reason, saveErr)
		}
		return fmt.Errorf("%s", reason)
	}
	sess.LastError = ""
	sess.ProcessPID = 0
	sess.CodexQuotaResumePending = false
	sess.ResumeFailures = 0
	sess.TokensTurn = r.TokensTurn
	sess.TokensCumulative += r.TokensTurn
	sess.TokensSinceCompact += subagent.CompactionTokens(r)
	sess.ReasoningTokensTotal += r.ReasoningTokens
	sess.ReasoningTokensKnown = sess.ReasoningTokensKnown && r.ReasoningTokensKnown
	sess.ContextTokens = r.ContextTokens
	sess.ContextWindowSize = r.ContextWindowSize
	sess.CachedTokens = r.CachedTokens
	sess.InputTokensTotal += r.InputTokens
	sess.CachedTokensTotal += r.CachedTokens
	sess.OutputTokensTotal += r.OutputTokens
	sess.TokenTotalsKnown = true
	sess.LastActiveAt = time.Now()
	sess.Status = "completed"
	sess.Turn = turn
	if err = recordAgentReply(sess, turn, r.Response, r.Messages, sess.LastActiveAt); err != nil {
		return err
	}
	sess.TurnRecords = append(sess.TurnRecords, subagent.TurnRecord{Turn: turn, NewInputTokens: subagent.CompactionTokens(r), CachedInputTokens: r.CachedTokens, OutputTokens: r.OutputTokens})
	before := <-beforeCapture
	baselineCacheAt := turnQuotaBaseline(turnStarted, before)
	storeTurnQuota(d.dbPath, sess.ID, sess.Provider, turn, "before", before)
	recordTurnQuota(d.dbPath, d.quota, sess.ID, sess.Provider, turn, "after", false, turnStarted, baselineCacheAt)
	persistAgentTurnTokens(d.dbPath, sess, r, sess.LastActiveAt)
	if err = s.SaveQueuedTurn(sess, queuedMessages); err != nil {
		return err
	}
	if streaming {
		ts.finish(r)
		warnQuota1Changes(cmd, sess.WorkingDir, quotaTurnStarted)
		return nil
	}
	if compacted {
		var ack string
		if r.Messages, ack = subagent.SplitCompactionAck(r.Messages); ack != "" {
			tl.log("compact", "agent acknowledged: %s", firstLine(ack))
		}
	}
	tl.finishTurn(r, sess.ID, "harnez agent resume "+sess.ID+" \"<prompt>\"")
	out := agentOutput{Session: sess, Response: r.Response, Messages: r.Messages}
	if !resumed {
		out.Resumed, out.PreviousSessionID, out.Note = &resumed, previousID, freshNote
	}
	err = writeAgentOutput(cmd, req.JSON, out)
	warnQuota1Changes(cmd, sess.WorkingDir, quotaTurnStarted)
	return err
}

func writeAgentOutput(cmd *cobra.Command, jsonOut bool, v agentOutput) error {
	if jsonOut {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(v)
	}
	return printAgentMessages(cmd, v.Messages, v.Response)
}

func recordAgentReply(sess *subagent.Session, turn int, response string, messages []string, at time.Time) error {
	limit, err := subagent.AgentReplyHistoryLimit()
	if err != nil {
		return err
	}
	sess.Response = response
	sess.Messages = messages
	sess.ReplyHistory = append(sess.ReplyHistory, subagent.ReplyRecord{Turn: turn, At: at, Text: response})
	if len(sess.ReplyHistory) > limit {
		sess.ReplyHistory = append([]subagent.ReplyRecord(nil), sess.ReplyHistory[len(sess.ReplyHistory)-limit:]...)
	}
	return nil
}

// knownSlashCommands are intercepted by the root form and never sent to the model.
var knownSlashCommands = map[string]bool{"/compact": true, "/stop": true, "/status": true}

func compactSession(cmd *cobra.Command, s *subagent.FileSessionStore, x *subagent.Session) error {
	if x.Status == "active" && x.HarnessType == "interactive" {
		if err := subagent.SendControl(cmd.Context(), x.ControlSocket, "compact", ""); err != nil {
			return err
		}
		x.LastActiveAt = time.Now()
		return s.Save(x)
	}
	if x.HarnessType == "interactive" && x.ProviderSessionID == "" {
		return fmt.Errorf("session %q cannot be compacted: %s did not expose a provider session ID", x.Name, x.Provider)
	}
	driver := withAgyMeterSession(agentDriver(subagent.Model{Provider: x.Provider, Name: x.Model, Tier: x.Tier}, x.WorkingDir), x.ID)
	result, err := driver.Compact(cmd.Context(), x.ProviderID())
	if err != nil {
		return err
	}
	if result != nil {
		if err := saveCompactResult(s, x, result); err != nil {
			return err
		}
		if result.ContextTokensBeforeCompact > 0 && result.ContextTokens > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Compacted: context usage %d -> %d tokens\n", result.ContextTokensBeforeCompact, result.ContextTokens)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Compacted: context usage unknown")
		}
		if result.Response != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Status: %s\n", result.Response)
		}
	}
	return nil
}

func saveCompactResult(s *subagent.FileSessionStore, x *subagent.Session, result *subagent.TurnResult) error {
	if !result.CompactionObserved {
		return nil
	}
	x.ContextTokens = result.ContextTokens
	x.CompactionCount++
	x.TokensSinceCompact = 0
	x.LastActiveAt = time.Now()
	return s.Save(x)
}

func withAgyMeterSession(driver subagent.Driver, sessionID string) subagent.Driver {
	switch d := driver.(type) {
	case subagent.AgyDriver:
		d.SessionID = sessionID
		return d
	case *subagent.AgyDriver:
		copy := *d
		copy.SessionID = sessionID
		return &copy
	default:
		return driver
	}
}

// withAgyAutoApprove sets whether an agy driver approves tool calls for the
// session's role (issue 1000); other drivers are returned unchanged.
func withAgyAutoApprove(driver subagent.Driver, autoApprove bool) subagent.Driver {
	if d, ok := driver.(subagent.AgyDriver); ok {
		d.AutoApproveTools = autoApprove
		return d
	}
	return driver
}

func statusSession(cmd *cobra.Command, x *subagent.Session, jsonOut, showReply bool, turn int, allReplies bool) error {
	if jsonOut {
		var pair *telemetry.CompactionPair
		db, err := telemetry.OpenReadOnly(agentUsageDBPath())
		if err == nil {
			defer db.Close()
			pair, err = db.LatestCompactionPair(x.ProviderID())
			if err != nil {
				return err
			}
		} else if _, statErr := os.Stat(agentUsageDBPath()); !os.IsNotExist(statErr) {
			return err
		}
		data, err := json.Marshal(x)
		if err != nil {
			return err
		}
		var output map[string]json.RawMessage
		if err := json.Unmarshal(data, &output); err != nil {
			return err
		}
		output["compaction_pair"], err = json.Marshal(pair)
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(output)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "ID: %s\nName: %s\nStatus: %s\nProvider: %s:%s\nTokens: %d (turn %d)\nCached: %d\n", x.ID, x.Name, x.Status, x.Provider, x.Model, x.TokensCumulative, x.TokensTurn, x.CachedTokens)
	if x.Role != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Role: %s\n", x.Role)
	}
	if x.LastError != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Last error: %s\nResume failures: %d\n", x.LastError, x.ResumeFailures)
	}
	if showReply {
		var replies []subagent.ReplyRecord
		switch {
		case allReplies:
			replies = x.ReplyHistory
		case turn > 0:
			for _, reply := range x.ReplyHistory {
				if reply.Turn == turn {
					replies = []subagent.ReplyRecord{reply}
					break
				}
			}
			if len(replies) == 0 {
				return fmt.Errorf("reply for turn %d is not saved for session %q", turn, x.Name)
			}
		default:
			if len(x.ReplyHistory) > 0 {
				replies = x.ReplyHistory[len(x.ReplyHistory)-1:]
			}
		}
		if len(replies) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Reply: (no saved reply)")
		}
		for _, reply := range replies {
			text := reply.Text
			if text == "" {
				text = "(empty reply)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Reply (turn %d, %s):\n%s\n", reply.Turn, reply.At.Format(time.RFC3339), text)
		}
	}
	return nil
}

const (
	agentRoleEnv     = "HARNEZ_AGENT_ROLE"
	agentSessionEnv  = "HARNEZ_SESSION_ID"
	agentModelEnv    = "HARNEZ_AGENT_MODEL"
	agentLauncherEnv = "HARNEZ_AGENT_LAUNCHER_PID"
)

// startRole picks the role of a new session and checks that the caller's own
// role (from the environment harnez gave it) may start it.
func startRole(requested, prompt string, errOut io.Writer) (string, error) {
	return agent.StartRole(requested, prompt, callerAgentRole(), errOut)
}

func firstPromptSentence(prompt string) string {
	return agent.FirstPromptSentence(prompt)
}

// resumeRole returns the stored role of sess (default role for old records),
// rejects a conflicting --role and checks the caller may manage that role.
func resumeRole(requested string, sess *subagent.Session) (string, error) {
	return agent.ResumeRole(requested, sess, callerAgentRole())
}

// setAgentEnv gives the provider process it is about to spawn its role and its
// session name (the parent id of anything it starts) and returns a restore func.
func setAgentEnv(role, name, model string) func() {
	prev := map[string]*string{}
	for _, kv := range [][2]string{{agentRoleEnv, role}, {agentSessionEnv, name}, {agentModelEnv, model}, {agentLauncherEnv, strconv.Itoa(os.Getpid())}} {
		if v, ok := os.LookupEnv(kv[0]); ok {
			old := v
			prev[kv[0]] = &old
		} else {
			prev[kv[0]] = nil
		}
		_ = os.Setenv(kv[0], kv[1])
	}
	return func() {
		for k, v := range prev {
			if v == nil {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *v)
			}
		}
	}
}

// guardLeafRole refuses mutating agent verbs when the caller is a leaf worker.
// Diagnostic verbs and channel send do not spawn or control workers.
func guardLeafRole(verb string) error {
	role := callerAgentRole()
	if !subagent.IsLeafRole(role) {
		return nil
	}
	switch verb {
	case "list", "status", "models", "help", "agent", "send":
		return nil // the root form checks each start/resume/slash itself
	}
	return fmt.Errorf("%w (role source: %s; worker session: %q; Codex thread: %q; launcher PID: %q). If this is an interactive host, check inherited Harnez variables and restart the host; Harnez cannot change the host-managed sandbox or writable roots", subagent.CheckSpawn(role, "developer"), agentRoleEnv, os.Getenv(agentSessionEnv), os.Getenv("CODEX_THREAD_ID"), os.Getenv(agentLauncherEnv))
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/agent"
	"ubunatic.com/harnez/internal/modelblock"
	"ubunatic.com/harnez/internal/procs"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/internal/usagestore"
	"ubunatic.com/harnez/modelcatalog"
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

func recordTurnQuota(dbPath string, capture func(context.Context, string, bool) usage.TurnQuotaReading, sessionID, provider string, turn int, boundary string, force bool, turnStarted, baselineCacheAt time.Time) usage.TurnQuotaReading {
	var reading usage.TurnQuotaReading
	timeout, err := usage.TurnQuotaTimeoutForProvider(provider)
	if err != nil {
		reading = usage.TurnQuotaReading{CapturedAt: time.Now().UTC(), Error: err.Error()}
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if capture != nil {
			reading = capture(ctx, provider, force)
		} else {
			reading = agentTurnQuotaReader(ctx, provider, force, turnStarted, baselineCacheAt)
		}
	}
	stored := usage.TurnQuotaBoundaryFromReading(sessionID, provider, turn, boundary, reading)
	if dbPath != "" {
		_ = usage.PersistTurnQuotaBoundary(context.Background(), dbPath, stored)
	}
	return reading
}

func captureTurnQuota(parent context.Context, capture func(context.Context, string, bool) usage.TurnQuotaReading, provider string, force bool, turnStarted time.Time) usage.TurnQuotaReading {
	if parent == nil {
		parent = context.Background()
	}
	timeout, err := usage.TurnQuotaTimeoutForProvider(provider)
	if err != nil {
		return usage.TurnQuotaReading{CapturedAt: time.Now().UTC(), Error: err.Error()}
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
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

// turnAPICost prices one turn in micro-USD. Claude is priced per API call on
// each call's own prompt (ticket 1301); any call without a rate or a TTL split
// makes the turn unknown. Other providers use the flat catalog entry.
func turnAPICost(sess *subagent.Session, r *subagent.TurnResult) (*int64, string, bool) {
	catalog, err := telemetry.LoadPricingCatalog("")
	if err != nil {
		return nil, "", false
	}
	if sess.Provider == "claude" {
		return claudeTurnCost(catalog, r)
	}
	pricing, ok := catalog.Lookup(sess.Model)
	if !ok || (pricing.Provider != "" && pricing.Provider != sess.Provider) {
		return nil, "", false
	}
	uncached := int64(r.InputTokens)
	cached := int64(r.CachedTokens)
	if r.CacheWriteTokensKnown && cached >= int64(r.CacheWriteTokens) {
		cached -= int64(r.CacheWriteTokens)
	}
	output := int64(r.OutputTokens)
	usageVec := telemetry.TokenUsage{
		UncachedInputTokens: &uncached,
		CachedInputTokens:   &cached,
		OutputTokens:        &output,
	}
	if r.ReasoningTokensKnown {
		reasoning := int64(r.ReasoningTokens)
		usageVec.ReasoningTokens = &reasoning
	}
	costMicros, costOK := telemetry.CalculateTurnCost(pricing.Rates, usageVec)
	if !costOK {
		return nil, "", false
	}
	return costMicros, pricing.Revision, true
}

// claudeTurnCost prices each API call by the model in its own message, so a
// sub-agent on Haiku under an Opus session is priced as Haiku. The revision is
// the sorted set of revisions used. Any call without a split, a model, or a
// rate makes the turn unknown.
func claudeTurnCost(catalog telemetry.PricingCatalog, r *subagent.TurnResult) (*int64, string, bool) {
	if len(r.RequestUsage) == 0 {
		return nil, "", false
	}
	var total int64
	revisions := map[string]bool{}
	for _, q := range r.RequestUsage {
		if !q.CacheWriteSplitKnown {
			return nil, "", false
		}
		pricing, ok := telemetry.ClaudeCallPricing(catalog, q.Model)
		if !ok {
			return nil, "", false
		}
		cost, ok := telemetry.RequestCostMicros(pricing, telemetry.RequestTokens{
			Uncached:     int64(q.Input),
			CacheRead:    int64(q.CacheRead),
			CacheWrite5m: int64(q.CacheWrite5m),
			CacheWrite1h: int64(q.CacheWrite1h),
			Output:       int64(q.Output),
		})
		if !ok {
			return nil, "", false
		}
		total += *cost
		revisions[pricing.Revision] = true
	}
	keys := make([]string, 0, len(revisions))
	for rev := range revisions {
		keys = append(keys, rev)
	}
	sort.Strings(keys)
	return &total, strings.Join(keys, ","), true
}

func populateTurnCostRecord(dbPath string, sess *subagent.Session, turn int, r *subagent.TurnResult, before, after usage.TurnQuotaReading) *subagent.TurnRecord {
	if sess == nil || len(sess.TurnRecords) == 0 {
		return nil
	}
	rec := &sess.TurnRecords[len(sess.TurnRecords)-1]
	// The record names the main-chain model the calls reported; the session
	// alias is the fallback (ticket 1302 M1, as persistAgentTurnTokens does).
	rec.Model = sess.Model
	if r != nil && r.Model != "" {
		rec.Model = r.Model
	}
	rec.Effort = sess.Tier
	if r != nil && !r.TurnTokensUnknown {
		if costMicros, revision, ok := turnAPICost(sess, r); ok {
			rec.CostMicros = costMicros
			rec.PricingRevision = revision
		}
	}
	monthlyPrice, _ := subagent.PlanPriceFor(sess.Provider)
	// A broken spec leaves the share unknown (nil), never a guessed tolerance.
	resetTolerance, tolErr := usage.TurnQuotaResetTolerance()
	share, window, beforePct, afterPct := usage.ComputeTurnQuotaShare(before, after, monthlyPrice, resetTolerance)
	if tolErr != nil {
		share = nil
	}
	rec.QuotaWindow = window
	rec.QuotaBeforeUsedPercent = beforePct
	rec.QuotaAfterUsedPercent = afterPct
	rec.SubscriptionPriceShareMicros = share
	if share != nil && dbPath != "" && window != "" {
		_ = usage.PersistTurnQuotaShare(context.Background(), dbPath, sess.ID, turn, sess.Provider, window, share)
	}
	subagent.RecomputeSessionCostTotals(sess)
	return rec
}

func persistAgentTurnTokens(dbPath string, session *subagent.Session, result *subagent.TurnResult, observedAt time.Time) {
	if session == nil || result == nil || dbPath == "" {
		return
	}
	var rec *subagent.TurnRecord
	if n := len(session.TurnRecords); n > 0 && session.TurnRecords[n-1].Turn == session.Turn {
		rec = &session.TurnRecords[n-1]
	}
	// The rows name the main-chain model the calls reported; the session's
	// alias (for example "opus") is only the fallback (ticket 1301 M4).
	model := session.Model
	if result.Model != "" {
		model = result.Model
	}
	if !result.TurnTokensUnknown {
		input, cached, output := int64(result.InputTokens), int64(result.CachedTokens), int64(result.OutputTokens)
		// CachedTokens includes cache writes for Claude; the row stores cache reads only.
		if result.CacheWriteTokensKnown && cached >= int64(result.CacheWriteTokens) {
			cached -= int64(result.CacheWriteTokens)
		}
		token := usagestore.TurnTokenUsage{
			SessionID: session.ID, Turn: session.Turn, Provider: session.Provider, CounterKind: "delta",
			Model: model, Effort: session.Tier,
			InputTokens: &input, CachedInputTokens: &cached, OutputTokens: &output,
			InputQuality: "measured", CachedInputQuality: "measured", OutputQuality: "measured", ReasoningQuality: "unknown",
			ObservedAt: observedAt,
		}
		if result.CacheWriteTokensKnown {
			cw := int64(result.CacheWriteTokens)
			token.CacheWriteTokens = &cw
		}
		if result.ReasoningTokensKnown {
			reasoning := int64(result.ReasoningTokens)
			token.ReasoningTokens = &reasoning
			token.ReasoningQuality = "measured"
		}
		if rec != nil {
			token.CostMicros = rec.CostMicros
			token.PricingRevision = rec.PricingRevision
		}
		_ = usage.PersistTurnTokenUsage(context.Background(), dbPath, token)
	}
	if !session.TokenTotalsKnown {
		return
	}
	cumulative := usagestore.TurnTokenUsage{
		SessionID: session.ID, Turn: session.Turn, Provider: session.Provider, CounterKind: "cumulative",
		Model: model, Effort: session.Tier,
		InputQuality: "measured", CachedInputQuality: "measured", OutputQuality: "measured", ReasoningQuality: "unknown",
		ObservedAt: observedAt,
	}
	cumulativeInput, cumulativeCached, cumulativeOutput := int64(session.InputTokensTotal), int64(session.CachedTokensTotal), int64(session.OutputTokensTotal)
	// Like the delta row, the cumulative cached count excludes cache writes
	// when the write total is known (ticket 1301 M3). A session resumed from
	// before M3 has CacheWriteTokensKnown false, so its cumulative cached count
	// keeps the old meaning (cache writes included). This is a known limit with
	// no migration (ticket 1301 M4).
	if session.CacheWriteTokensKnown && cumulativeCached >= int64(session.CacheWriteTokensTotal) {
		cumulativeCached -= int64(session.CacheWriteTokensTotal)
		cw := int64(session.CacheWriteTokensTotal)
		cumulative.CacheWriteTokens = &cw
	}
	cumulative.InputTokens, cumulative.CachedInputTokens, cumulative.OutputTokens = &cumulativeInput, &cumulativeCached, &cumulativeOutput
	if session.ReasoningTokensKnown {
		reasoning := int64(session.ReasoningTokensTotal)
		cumulative.ReasoningTokens = &reasoning
		cumulative.ReasoningQuality = "measured"
	}
	if session.TotalCostMicros != nil {
		totalCost := *session.TotalCostMicros
		cumulative.CostMicros = &totalCost
		if rec != nil {
			cumulative.PricingRevision = rec.PricingRevision
		}
	}
	_ = usage.PersistTurnTokenUsage(context.Background(), dbPath, cumulative)
}

func buildTurnDoneMeta(sess *subagent.Session, quotaUnknown bool) turnDoneMeta {
	meta := turnDoneMeta{sess: sess, quotaUnknown: quotaUnknown}
	if spec, err := subagent.EffectiveCostCheckSpec(); err == nil {
		meta.costSpec = spec
	}
	if sess == nil {
		return meta
	}
	mode, modeErr := subagent.AutoCompactMode(sess.Provider)
	if modeErr == nil && mode != "disabled" {
		if limit, err := subagent.CompactThreshold(subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier}); err == nil && limit > 0 {
			meta.compactLimit = limit
		}
	}
	return meta
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
// A signal cancels the context too, so the provider child dies with the turn.
func withAgentTimeout(cmd *cobra.Command, timeout time.Duration, run func(context.Context) error) error {
	ctx, stop := agentSignalContext(agentCommandContext(cmd))
	defer stop()
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

// startRequest describes one new agent turn. Prompt is sent to the agent;
// StoredPrompt is what the session records (files as "path (N bytes)").
type startRequest struct {
	DryRun                                                                    bool
	PromptSource                                                              string
	Prompt, StoredPrompt, Name, ModelSpec, Dir, StreamMode, Role, ContextMode string
	JSON, PlanFirst, AllowExhaustedQuota, AcceptCost                          bool
	SessionID                                                                 string
	MaxToolCalls                                                              int
	MaxToolCallsSet                                                           bool
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
	return selectStartSelectionWithOptions(role, spec, override, false, availability, stderr)
}

func selectStartSelectionWithOptions(role, spec string, override, acceptCost bool, availability func(string, string) usage.ProviderQuotaAvailability, stderr io.Writer) (subagent.ModelSelection, error) {
	if availability == nil {
		availability = providerAvailability
	}
	selection, err := subagent.ResolveModelWithOptions(role, spec, subagent.ResolveOptions{
		Exhausted:  func(m subagent.Model) bool { return availability(m.Provider, m.Name).State == "exhausted" },
		Override:   override,
		AcceptCost: acceptCost,
	})
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

func checkResumeModel(role, spec string, sess *subagent.Session, override bool, availability func(string, string) usage.ProviderQuotaAvailability, stderr io.Writer) error {
	return checkResumeModelWithCost(role, spec, sess, override, false, availability, stderr)
}

func checkResumeModelWithCost(role, spec string, sess *subagent.Session, override, acceptCost bool, availability func(string, string) usage.ProviderQuotaAvailability, stderr io.Writer) error {
	selection, err := selectStartSelectionWithOptions(role, spec, override, acceptCost, availability, stderr)
	if err != nil || selection.Model.Provider != sess.Provider || selection.Model.Name != sess.Model || selection.Model.Tier != sess.Tier {
		return fmt.Errorf("--model %q conflicts with session %q model %s:%s:%s", spec, sess.Name, sess.Provider, sess.Model, sess.Tier)
	}
	return nil
}

// autoBlockQuotaError records a local model block when a failed turn carries a
// known quota error (issue 1122), so the next start or resume refuses the
// model until the reset time. Failures to record never mask the turn error.
func autoBlockQuotaError(stderr io.Writer, provider, model string, turnErr error) {
	spec, err := modelblock.LoadSpec()
	if err != nil || turnErr == nil {
		return
	}
	b, ok, err := modelblock.AutoBlock(modelblock.Path(), spec, provider, model, turnErr.Error(), time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "harnez: could not record quota block: %v\n", err)
		return
	}
	if ok {
		fmt.Fprintf(stderr, "harnez: blocked %s:%s until %s (known quota error); `harnez models unblock` clears it\n", provider, model, b.Until.Format(time.RFC3339))
	}
}

func recordCodexAccess(stderr io.Writer, provider, model string, turnErr error) {
	if provider != "codex" {
		return
	}
	status := "accessible"
	if turnErr != nil {
		message := strings.ToLower(turnErr.Error())
		modelPattern := `(^|[^a-z0-9._-])` + regexp.QuoteMeta(strings.ToLower(model)) + `($|[^a-z0-9._-])`
		modelFound, _ := regexp.MatchString(modelPattern, message)
		unsupportedCodex := strings.Contains(message, "not supported when using codex")
		modelNotFound := model != "" && modelFound && strings.Contains(message, "model_not_found")
		if !unsupportedCodex && !modelNotFound {
			return
		}
		status = "unsupported"
	}
	if err := modelcatalog.RecordAccess(provider, model, status); err != nil {
		fmt.Fprintf(stderr, "harnez: could not record model access: %v\n", err)
	}
}

// modelAccessDir returns the models directory for access records. An explicit
// --store-dir, or a non-blank HARNEZ_STORE_DIR, keeps access.yaml and its lock
// beneath that store. Otherwise it returns "" and the default state directory
// is used.
func modelAccessDir(cmd *cobra.Command) string {
	if f := cmd.InheritedFlags().Lookup("store-dir"); f != nil && f.Changed && strings.TrimSpace(f.Value.String()) != "" {
		return filepath.Join(f.Value.String(), "models")
	}
	if v := strings.TrimSpace(os.Getenv(subagent.StoreDirEnv)); v != "" {
		return filepath.Join(v, "models")
	}
	return ""
}

func providerErrorLine(err error) string {
	for _, line := range strings.Split(err.Error(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return "provider failed"
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

// resumeRequest describes one turn on an existing session: chosen by Name,
// else by attribution in Dir (Continue picks the most recent).
type resumeRequest struct {
	DryRun                                   bool
	PromptSource                             string
	Role                                     string // must match the stored role when given
	Prompt, Name, ModelSpec, Dir, StreamMode string
	SessionID, Selector                      string
	Continue, JSON, PlanFirst                bool
	AllowExhaustedQuota, AcceptCost          bool
	MaxToolCalls                             int
	MaxToolCallsSet                          bool
}

// rejectBlockedSessionModel refuses a resume while a local model block (issue
// 1122) covers the session's model, whatever the provider or session state.
func rejectBlockedSessionModel(sess *subagent.Session, override bool, availability func(string, string) usage.ProviderQuotaAvailability) error {
	if override {
		return nil
	}
	if availability == nil {
		availability = providerAvailability
	}
	if a := availability(sess.Provider, sess.Model); !a.BlockedUntil.IsZero() {
		return fmt.Errorf("session %q model %s:%s is blocked until %s (%s); `harnez models unblock` clears it or pass --allow-exhausted-quota", sess.Name, sess.Provider, sess.Model, a.BlockedUntil.Format(time.RFC3339), firstNonEmpty(a.BlockReason, "no reason"))
	}
	return nil
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
	spec := req.ModelSpec
	if err := checkStreamMode(req.StreamMode); err != nil {
		return err
	}
	role, inferred, err := startRole(req.Role, req.Prompt, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	if req.DryRun && d.availability == nil {
		d.availability = func(provider, model string) usage.ProviderQuotaAvailability {
			return usage.CachedProviderQuotaAvailabilityReadOnly(provider, model, time.Now())
		}
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
		selection, err = selectStartSelectionWithOptions(role, spec, req.AllowExhaustedQuota, req.AcceptCost, d.availability, cmd.ErrOrStderr())
	}
	if err != nil {
		return startModelError(spec, selection, err)
	}
	m := selection.Model
	if err := subagent.ValidateContextMode(m.Provider, req.ContextMode); err != nil {
		return err
	}
	tcGuard, err := newToolCallGuard(m, role, req.MaxToolCalls, req.MaxToolCallsSet, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	if !req.DryRun {
		if warning, err := agent.CheckCostlyModel(m, req.AcceptCost, d.availability, time.Now()); err != nil {
			return err
		} else if warning != "" && req.SessionID == "" {
			fmt.Fprintln(cmd.ErrOrStderr(), warning)
		}
	}
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
		if existing, findErr := resolveSession(s, req.Name, ""); findErr == nil && (req.DryRun || !releaseFailedStartName(s, existing)) {
			// 1186: names are global across repos; suffix a taken explicit name.
			if sessName, err = suffixTakenName(s, req.Name, cmd.ErrOrStderr()); err != nil {
				return err
			}
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
	if req.DryRun {
		return printAgentPreview(cmd, m, role, sessName, canonicalWorkDir, "", req.Prompt, req.PromptSource, req.ContextMode, autoApprove, false, req.AcceptCost, d.availability)
	}
	baseDriver := withAgyAutoApprove(agentDriver(m, canonicalWorkDir), autoApprove)
	if err := preflightCodex(ctx, m.Provider, baseDriver, d.preflight); err != nil {
		return fmt.Errorf("agent start %q preflight: %w", spec, err)
	}
	if m.Provider == "pi" || m.Provider == "lmcoder" {
		warnSmallPiWindow(cmd.ErrOrStderr(), m, piAgentDir())
	}
	tl := newTimeline(cmd)
	opts := subagent.RunOptions{Prompt: req.Prompt, Model: m, Dir: canonicalWorkDir, ContextMode: req.ContextMode}
	parentID := d.parent() // read before the child's environment replaces it
	defer setAgentEnv(role, sessName, spec)()
	initialSession := func(status string) *subagent.Session {
		now := time.Now()
		sessionID := id
		if req.SessionID == "" && sessionID == "" {
			sessionID = uuid.NewString()
		}
		return &subagent.Session{ID: sessionID, Name: sessName, StartPrompt: req.StoredPrompt, Provider: m.Provider, Model: m.Name, ModelAlias: selection.Trait, Tier: m.Tier, WorkingDir: canonicalWorkDir, ParentSessionID: parentID, CallerPID: os.Getpid(), CallerStarttime: procs.ProcessStarttime(os.Getpid()), HarnessType: "harnez", Status: status, Role: role, CreatedAt: now, LastActiveAt: now}
	}
	if req.SessionID == "" && d.storeDir != "" {
		if timeout := foregroundDetachTimeout(cmd); timeout > 0 {
			now := time.Now()
			sess := &subagent.Session{ID: id, Name: sessName, StartPrompt: req.StoredPrompt, Provider: m.Provider, Model: m.Name, ModelAlias: selection.Trait, Tier: m.Tier, WorkingDir: canonicalWorkDir, ParentSessionID: parentID, CallerPID: os.Getpid(), CallerStarttime: procs.ProcessStarttime(os.Getpid()), HarnessType: "harnez", Status: "running", Role: role, CreatedAt: now, LastActiveAt: now}
			sess.StdoutLog = filepath.Join(d.storeDir, id+".stdout.log")
			sess.StderrLog = filepath.Join(d.storeDir, id+".stderr.log")
			if err := s.Create(sess); err != nil {
				return err
			}
			workerSpec := m.Spec()
			args := []string{"--store-dir", d.storeDir, "agent", "start", "--worker-session", id, "--json", "--model", workerSpec, "--role", role, "--name", sessName, "--dir", canonicalWorkDir, "--plan", map[bool]string{true: "yes", false: "no"}[req.PlanFirst]}
			if req.ContextMode != "" {
				args = append(args, "--context-mode", req.ContextMode)
			}
			if req.AllowExhaustedQuota {
				args = append(args, "--allow-exhausted-quota")
			}
			if req.AcceptCost {
				args = append(args, "--accept-cost")
			}
			if req.MaxToolCallsSet {
				args = append(args, "--max-tool-calls", fmt.Sprint(req.MaxToolCalls))
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
	// Claude streams on every path, --json and detached included, so the token
	// watchdog sees live usage (ticket 1264 M1, M2).
	if tcGuard != nil || m.Provider == "claude" {
		driver = asStreamingDriverForToolLimit(driver)
	}
	if tcGuard != nil && tcGuard.isPi {
		driver = subagent.WithToolCallLimit(driver, tcGuard.cfg.Limit)
		defer setToolCallEnv(tcGuard.cfg)()
	}
	sd, canStream := driver.(subagent.StreamingDriver)
	streaming := canStream && !req.JSON
	// Claude streams on --json too, so the watchdog covers it (ticket 1264 M2).
	useStream := canStream && (streaming || tcGuard != nil || m.Provider == "claude")
	turn := 1
	turnStarted := time.Now().UTC()
	quotaUnknown := warnModelQuota(cmd, d.availability, m.Provider, m.Name, turnStarted.Local())
	beforeCapture, stopCapture := startTurnQuotaCapture(ctx, d.quota, m.Provider)
	defer stopCapture()
	var ts *turnStream
	var r *subagent.TurnResult
	observedSessionID := ""
	if useStream {
		if streaming {
			ts = newTurnStream(cmd, req.StreamMode, false)
			ts.stopCmd, ts.name, ts.planFirst = "harnez agent stop "+sessName, sessName, req.PlanFirst
			opts.Prompt = withProtocol(req.Prompt, req.PlanFirst, role, inferred, opts.Model.Provider)
			ts.watch()
		}
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
			if tcGuard != nil && tcGuard.observe(ev) {
				cancelWatch()
			}
			if streaming {
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
			}
		})
		if err != nil {
			synthRes, partialRes, handled, toolErr := tcGuard.handleStoppedError(ctx, sd, observedSessionID, m, opts, ts, err)
			if handled {
				if toolErr == nil && synthRes != nil {
					r, err = synthRes, nil
				} else {
					if ts != nil {
						ts.abort()
					}
					before := <-beforeCapture
					baselineCacheAt := turnQuotaBaseline(turnStarted, before)
					storeTurnQuota(d.dbPath, id, m.Provider, turn, "before", before)
					recordTurnQuota(d.dbPath, d.quota, id, m.Provider, turn, "after", false, turnStarted, baselineCacheAt)
					if req.SessionID != "" {
						id = req.SessionID
					}
					sess, getErr := s.Get(id)
					if getErr != nil {
						sess = initialSession("stopped")
						id = sess.ID
					}
					sess.ProviderSessionID = firstNonEmpty(sess.ProviderSessionID, observedSessionID)
					return saveStoppedToolLimitTurn(s, sess, turn, partialRes, tcGuard, "start", sessName)
				}
			} else {
				if ts != nil {
					ts.abort()
				}
				if watchdogCrossed {
					stopErr := fmt.Errorf("agent start %q crossed live token threshold at %d tokens (limit %d); turn interrupted and session %q can be resumed", spec, observedTokens, threshold, sessName)
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
						// The resume gate reads ContextTokens; store the crossing so it compacts first (1264 M1).
						sess.ContextTokens = observedTokens
						subagent.SyncCodexRolloutTotals(sess)
						meterStoppedTurn(d, sess, turn, partialTurn(err), beforeCapture, turnStarted)
						if saveErr := s.Save(sess); saveErr != nil {
							return fmt.Errorf("agent start %q crossed live token threshold at %d tokens (limit %d); session recovery record failed: %w", spec, observedTokens, threshold, saveErr)
						}
						if req.JSON {
							if encErr := writeStoppedAgentOutput(cmd, sess, partialTurn(err)); encErr != nil {
								return encErr
							}
						}
					}
					return stopErr
				}
			}
		}
	} else {
		tl.announceTurn("start", m.Provider+":"+m.Name, sessName)
		r, err = driver.Run(subagent.WithProviderSessionObserver(ctx, func(providerID string) {
			if sess, getErr := s.Get(id); getErr == nil {
				sess.ProviderSessionID = providerID
				sess.LastActiveAt = time.Now()
				_ = s.Save(sess)
			}
		}), opts)
	}
	if err != nil {
		before := <-beforeCapture
		baselineCacheAt := turnQuotaBaseline(turnStarted, before)
		storeTurnQuota(d.dbPath, id, m.Provider, turn, "before", before)
		recordTurnQuota(d.dbPath, d.quota, id, m.Provider, turn, "after", false, turnStarted, baselineCacheAt)
		sig, signalled := agentSignalCause(ctx)
		if req.SessionID == "" || signalled {
			sess, getErr := s.Get(id)
			if getErr != nil {
				sess = initialSession("failed")
			}
			sess.ProviderSessionID = firstNonEmpty(sess.ProviderSessionID, observedSessionID)
			sess.Status = "failed"
			if signalled {
				sess.Status, sess.ProviderPID, sess.ProviderStarttime = "stopped", 0, 0
				err = sig
			}
			sess.ProcessPID, sess.ProcessStarttime = 0, 0
			sess.LastError = err.Error()
			sess.LastActiveAt = time.Now()
			subagent.SyncCodexRolloutTotals(sess)
			if saveErr := s.Save(sess); saveErr != nil {
				return fmt.Errorf("agent start %q failed: %s; session recovery record failed: %w", spec, providerErrorLine(err), saveErr)
			}
		}
		if signalled {
			return fmt.Errorf("agent start %q %s; provider stopped and session %q can be resumed", spec, sig, sessName)
		}
		recordCodexAccess(cmd.ErrOrStderr(), m.Provider, m.Name, err)
		autoBlockQuotaError(cmd.ErrOrStderr(), m.Provider, m.Name, err)
		return fmt.Errorf("agent start %q failed: %s; verify the provider/model configuration or ask for guidance", spec, providerErrorLine(err))
	}
	recordCodexAccess(cmd.ErrOrStderr(), m.Provider, m.Name, nil)
	tcGuard.attachResult(r)
	for _, warning := range r.Warnings {
		fmt.Fprintln(cmd.ErrOrStderr(), warning)
	}
	providerSessionID := r.SessionID
	reservedID := id
	if r.SessionID != "" && req.SessionID == "" {
		id = r.SessionID
	}
	before := <-beforeCapture
	baselineCacheAt := turnQuotaBaseline(turnStarted, before)
	storeTurnQuota(d.dbPath, id, m.Provider, turn, "before", before)
	after := recordTurnQuota(d.dbPath, d.quota, id, m.Provider, turn, "after", false, turnStarted, baselineCacheAt)
	now := time.Now()
	sess := &subagent.Session{ID: id, ProviderSessionID: providerSessionID, Name: sessName, StartPrompt: req.StoredPrompt, Role: role, Provider: m.Provider, Model: m.Name, ModelAlias: selection.Trait, Tier: m.Tier, WorkingDir: canonicalWorkDir, ParentSessionID: parentID, CallerPID: os.Getpid(), CallerStarttime: procs.ProcessStarttime(os.Getpid()), HarnessType: "harnez", Status: "completed", Response: r.Response, Messages: r.Messages, TokensCumulative: r.TokensCumulative, InputTokensTotal: r.InputTokens, CachedTokensTotal: r.CachedTokens, CacheWriteTokensTotal: r.CacheWriteTokens, CacheWriteTokensKnown: r.CacheWriteTokensKnown, OutputTokensTotal: r.OutputTokens, ReasoningTokensTotal: r.ReasoningTokens, ReasoningTokensKnown: r.ReasoningTokensKnown, TokenTotalsKnown: !r.TurnTokensUnknown || r.CumulativeTotalsKnown, TokensSinceCompact: subagent.CompactionTokens(r), ContextTokens: r.ContextTokens, FreshStartContextTokens: max(r.ContextTokens, 0), TokensTurn: r.TokensTurn, CachedTokens: r.CachedTokens, CreatedAt: now, LastActiveAt: now, Turn: turn, TurnRecords: []subagent.TurnRecord{subagent.NewTurnRecord(turn, r)}}
	sess.ContextWindowSize = r.ContextWindowSize
	populateTurnCostRecord(d.dbPath, sess, turn, r, before, after)
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
			sess.CallerStarttime = current.CallerStarttime
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
	out := agentOutput{
		Session:              sess,
		Response:             r.Response,
		Messages:             r.Messages,
		ReconnectCmd:         "harnez agent resume " + id + " \"<prompt>\"",
		ToolCalls:            r.ToolCalls,
		ToolCallLimit:        r.ToolCallLimit,
		ToolCallLimitReached: r.ToolCallLimitReached,
		PeakContextTokens:    r.PeakContextTokens,
	}
	if streaming {
		ts.doneMeta = buildTurnDoneMeta(sess, quotaUnknown)
		ts.finish(r)
		return stillRunningExit(cmd, r, nil)
	}
	if !req.JSON && req.StreamMode == streamStats && !req.PlanFirst {
		ts = newTurnStream(cmd, streamStats, false)
		ts.began = turnStarted
		ts.doneMeta = buildTurnDoneMeta(sess, quotaUnknown)
		ts.finish(r)
		return stillRunningExit(cmd, r, nil)
	}
	tl.finishTurn(r, id, out.ReconnectCmd)
	if req.JSON {
		err := json.NewEncoder(cmd.OutOrStdout()).Encode(out)
		return stillRunningExit(cmd, r, err)
	}
	err = printAgentMessages(cmd, r.Messages, r.Response)
	return stillRunningExit(cmd, r, err)
}

// runResume runs one more turn on an existing session.
// runResume runs one resume turn under ctx; see runStart for why ctx is explicit.
func runResume(ctx context.Context, cmd *cobra.Command, d agentDeps, req resumeRequest) error {

	sessionName := req.Name
	var resolved string
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
	if req.DryRun {
		if d.availability == nil {
			d.availability = func(provider, model string) usage.ProviderQuotaAvailability {
				return usage.CachedProviderQuotaAvailabilityReadOnly(provider, model, time.Now())
			}
		}
		role, err := resumeRole(req.Role, sess)
		if err != nil {
			return err
		}
		if req.ModelSpec != "" {
			if err := checkResumeModelWithCost(role, req.ModelSpec, sess, req.AllowExhaustedQuota, req.AcceptCost, d.availability, cmd.ErrOrStderr()); err != nil {
				return err
			}
		}
		approve, err := subagent.RoleAutoApproveTools(role)
		if err != nil {
			return err
		}
		m := subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier}
		return printAgentPreview(cmd, m, role, sess.Name, sess.WorkingDir, sess.ProviderID(), req.Prompt, req.PromptSource, "", approve, true, req.AcceptCost, d.availability)
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
	if warning, err := agent.CheckCostlyModel(subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier}, req.AcceptCost, d.availability, time.Now()); err != nil {
		return err
	} else if warning != "" && req.SessionID == "" {
		fmt.Fprintln(cmd.ErrOrStderr(), warning)
	}
	if req.SessionID == "" && (sess.HarnessType != "interactive" || sess.Status != "active") {
		writers, err := sessionWriters(sess)
		if err != nil {
			return fmt.Errorf("check session writer: %w", err)
		}
		if len(writers) > 0 {
			return fmt.Errorf("session %q already has a resume turn running (PID %d); writer is still alive; stop it or run `harnez agent wait %s` before resuming", sess.Name, writers[0].PID, sess.Name)
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
	if err := rejectBlockedSessionModel(sess, req.AllowExhaustedQuota, d.availability); err != nil {
		return err
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
		if err := checkResumeModelWithCost(role, req.ModelSpec, sess, req.AllowExhaustedQuota, req.AcceptCost, d.availability, cmd.ErrOrStderr()); err != nil {
			return err
		}
	}
	model := subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier}
	tcGuard, err := newToolCallGuard(model, role, req.MaxToolCalls, req.MaxToolCallsSet, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
	defer setAgentEnv(role, sess.Name, model.Spec())()
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
		baseDriver := withAgyAutoApprove(agentDriver(model, sess.WorkingDir), autoApprove)
		if err := preflightCodex(ctx, sess.Provider, baseDriver, d.preflight); err != nil {
			return fmt.Errorf("agent resume %q preflight: %w", sess.Name, err)
		}
		driver = withAgyMeterSession(baseDriver, sess.ID)
		// Claude streams on every path so the token watchdog sees live usage (ticket 1264 M1, M2).
		if tcGuard != nil || model.Provider == "claude" {
			driver = asStreamingDriverForToolLimit(driver)
		}
		if tcGuard != nil && tcGuard.isPi {
			driver = subagent.WithToolCallLimit(driver, tcGuard.cfg.Limit)
			defer setToolCallEnv(tcGuard.cfg)()
		}
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
		return driver.Compact(ctx, sess.ProviderID(), subagent.Model{Provider: sess.Provider, Name: sess.Model, Tier: sess.Tier})
	}
	// Interactive sessions are guarded above; native compaction is verified after resume.
	if compactMode == "manual" || interactive {
		if contextTokens == 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "harnez: warning: session %q has no recorded context token count; skipping compaction check\n", sess.Name)
		} else if contextTokens > 0 {
			compacted, err = subagent.EnsureContextUnderThreshold(contextTokens, threshold, compactFn)
			if err != nil {
				return fmt.Errorf("refusing to send resume prompt: %w", err)
			}
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
	if subagent.IsControlPrompt(driver, req.Prompt) {
		// A command carries no text; queued messages stay for the next real message.
		queuedMessages, resumeIdle = nil, 0
	}
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
	sd, canStream := driver.(subagent.StreamingDriver)
	streaming := canStream && !req.JSON
	useStream := canStream && (streaming || tcGuard != nil || model.Provider == "claude")
	turn := sess.Turn + 1
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
			args := []string{"--store-dir", d.storeDir, "agent", "resume", "--worker-session", sess.ID, "--json", "--name", sess.Name, "--role", role, "--dir", sess.WorkingDir, "--plan", map[bool]string{true: "yes", false: "no"}[req.PlanFirst]}
			if req.AllowExhaustedQuota {
				args = append(args, "--allow-exhausted-quota")
			}
			if req.AcceptCost {
				args = append(args, "--accept-cost")
			}
			if req.MaxToolCallsSet {
				args = append(args, "--max-tool-calls", fmt.Sprint(req.MaxToolCalls))
			}
			args = append(args, "--", req.Prompt)
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
	quotaUnknown := warnModelQuota(cmd, d.availability, sess.Provider, sess.Model, turnStarted.Local())
	beforeCapture, stopCapture := startTurnQuotaCapture(ctx, d.quota, sess.Provider)
	defer stopCapture()
	observedSessionID := ""
	if useStream {
		if streaming {
			ts = newTurnStream(cmd, req.StreamMode, compacted)
			ts.info(sess.ID, sess.Provider+":"+sess.Model, "resume", resolved)
			if compacted {
				ts.printf("[compact: %s]\n", compactNote)
			}
			ts.stopCmd, ts.name, ts.planFirst = "harnez agent stop "+sess.Name, sess.Name, req.PlanFirst
			ts.watch()
			resumeTurn.Opts.Prompt = withResumeNote(withProtocol(req.Prompt, req.PlanFirst, role, false, ""), resumeIdle)
			resumeTurn.OnFresh = func(note string) { freshNote = note; ts.printf("[%s]\n", note) }
		} else {
			if compacted {
				tl.log("compact", "%s", compactNote)
			}
			tl.announceTurn("resume", sess.Provider+":"+sess.Model, sess.Name)
			resumeTurn.OnFresh = func(note string) { freshNote = note; tl.log("resume", "%s", note) }
		}
		watchCtx, cancelWatch := context.WithCancel(ctx)
		defer cancelWatch()
		watchdog := &subagent.TokenWatchdog{Threshold: threshold}
		watchdogCrossed := false
		observedTokens := 0
		resumeTurn.Events = func(ev subagent.Event) {
			if ev.Kind == "session" {
				observedSessionID = ev.Text
			}
			if tokens, crossed := watchdog.Observe(ev); crossed {
				watchdogCrossed = true
				observedTokens = tokens
				cancelWatch()
			}
			if tcGuard != nil && tcGuard.observe(ev) {
				cancelWatch()
			}
			if streaming {
				ts.onEvent(ev)
			}
		}
		r, resumed, err = resumeTurn.Do(watchCtx)
		if err != nil {
			providerID := firstNonEmpty(observedSessionID, sess.ProviderID())
			synthRes, partialRes, handled, toolErr := tcGuard.handleStoppedError(ctx, sd, providerID, model, resumeTurn.Opts, ts, err)
			if handled {
				if toolErr == nil && synthRes != nil {
					r, err = synthRes, nil
				} else {
					if ts != nil {
						ts.abort()
					}
					before := <-beforeCapture
					baselineCacheAt := turnQuotaBaseline(turnStarted, before)
					storeTurnQuota(d.dbPath, sess.ID, sess.Provider, turn, "before", before)
					recordTurnQuota(d.dbPath, d.quota, sess.ID, sess.Provider, turn, "after", false, turnStarted, baselineCacheAt)
					sess.ProviderSessionID = firstNonEmpty(sess.ProviderSessionID, observedSessionID)
					return saveStoppedToolLimitTurn(s, sess, turn, partialRes, tcGuard, "resume", sess.Name)
				}
			} else {
				if ts != nil {
					ts.abort()
				}
				if watchdogCrossed {
					sess.Status = "stopped"
					sess.ProcessPID = 0
					sess.LastError = fmt.Sprintf("runtime token watchdog interrupted the active turn at %d tokens (limit %d)", observedTokens, threshold)
					sess.LastActiveAt = time.Now()
					sess.ContextTokens = observedTokens // the resume gate compacts before the next prompt (1264 M1)
					subagent.SyncCodexRolloutTotals(sess)
					sess.Turn = turn
					meterStoppedTurn(d, sess, turn, partialTurn(err), beforeCapture, turnStarted)
					if saveErr := s.Save(sess); saveErr != nil {
						return fmt.Errorf("agent resume %q crossed live token threshold at %d tokens (limit %d); session recovery record failed: %w", sess.Name, observedTokens, threshold, saveErr)
					}
					if req.JSON {
						if encErr := writeStoppedAgentOutput(cmd, sess, partialTurn(err)); encErr != nil {
							return encErr
						}
					}
					return fmt.Errorf("agent resume %q crossed live token threshold at %d tokens (limit %d); turn interrupted and session can be resumed", sess.Name, observedTokens, threshold)
				}
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
		if sig, signalled := agentSignalCause(ctx); signalled {
			// A signal is the caller's stop, not a provider failure.
			sess.Status, sess.LastError = "stopped", sig.Error()
			sess.ProcessPID, sess.ProviderPID, sess.ProviderStarttime = 0, 0, 0
			sess.LastActiveAt = time.Now()
			subagent.SyncCodexRolloutTotals(sess)
			if saveErr := s.Save(sess); saveErr != nil {
				return fmt.Errorf("agent resume %q %s; session recovery record failed: %w", sess.Name, sig, saveErr)
			}
			return fmt.Errorf("agent resume %q %s; provider stopped and session can be resumed", sess.Name, sig)
		}
		recordResumeFailure(s, sess, err)
		recordCodexAccess(cmd.ErrOrStderr(), sess.Provider, sess.Model, err)
		autoBlockQuotaError(cmd.ErrOrStderr(), sess.Provider, sess.Model, err)
		quotaState := "unknown"
		if d.availability != nil {
			quotaState = d.availability(sess.Provider, sess.Model).State
		}
		if recordCodexResumeFailure(sess, err, quotaState, time.Now()) {
			_ = s.Save(sess)
		}
		if subagent.SyncCodexRolloutTotals(sess) {
			_ = s.Save(sess)
		}
		return fmt.Errorf("agent resume %q (%s:%s:%s) failed: %w; verify the provider/model configuration or ask for guidance", sess.Name, sess.Provider, sess.Model, sess.Tier, err)
	}
	recordCodexAccess(cmd.ErrOrStderr(), sess.Provider, sess.Model, nil)
	tcGuard.attachResult(r)
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
	if r.CumulativeTotalsKnown {
		// Codex reports monotonic session totals across turns; convert to a
		// turn delta if no rollout baseline already subtracted prior turns,
		// and store the cumulative total directly instead of summing.
		if !r.TurnTokensUnknown {
			if r.CumulativeInputTokens < sess.InputTokensTotal || r.CumulativeCachedTokens < sess.CachedTokensTotal || r.CumulativeOutputTokens < sess.OutputTokensTotal {
				r.TurnTokensUnknown = true
				r.InputTokens = 0
				r.CachedTokens = 0
				r.OutputTokens = 0
				r.ReasoningTokens = 0
				r.ReasoningTokensKnown = false
				r.TokensTurn = 0
			} else if r.InputTokens == r.CumulativeInputTokens && (sess.InputTokensTotal > 0 || sess.OutputTokensTotal > 0) {
				r.InputTokens = r.CumulativeInputTokens - sess.InputTokensTotal
				r.CachedTokens = r.CumulativeCachedTokens - sess.CachedTokensTotal
				r.OutputTokens = r.CumulativeOutputTokens - sess.OutputTokensTotal
				if r.ReasoningTokensKnown {
					r.ReasoningTokens = max(r.CumulativeReasoningTokens-sess.ReasoningTokensTotal, 0)
				}
				r.TokensTurn = r.InputTokens + r.OutputTokens
			}
		}
		sess.TokensTurn = r.TokensTurn
		sess.TokensCumulative = r.CumulativeInputTokens + r.CumulativeOutputTokens
		sess.InputTokensTotal = r.CumulativeInputTokens
		sess.CachedTokensTotal = r.CumulativeCachedTokens
		sess.CacheWriteTokensKnown = false
		sess.OutputTokensTotal = r.CumulativeOutputTokens
		sess.ReasoningTokensTotal = r.CumulativeReasoningTokens
		sess.ReasoningTokensKnown = sess.ReasoningTokensKnown && r.ReasoningTokensKnown
		sess.TokenTotalsKnown = true
	} else {
		addTurnTotals(sess, r)
	}
	sess.TokensSinceCompact += subagent.CompactionTokens(r)
	sess.ContextTokens = r.ContextTokens
	sess.ContextWindowSize = r.ContextWindowSize
	sess.CachedTokens = r.CachedTokens
	sess.LastActiveAt = time.Now()
	sess.Status = "completed"
	sess.Turn = turn
	if err = recordAgentReply(sess, turn, r.Response, r.Messages, sess.LastActiveAt); err != nil {
		return err
	}
	sess.TurnRecords = append(sess.TurnRecords, subagent.NewTurnRecord(turn, r))
	before := <-beforeCapture
	baselineCacheAt := turnQuotaBaseline(turnStarted, before)
	storeTurnQuota(d.dbPath, sess.ID, sess.Provider, turn, "before", before)
	after := recordTurnQuota(d.dbPath, d.quota, sess.ID, sess.Provider, turn, "after", false, turnStarted, baselineCacheAt)
	populateTurnCostRecord(d.dbPath, sess, turn, r, before, after)
	persistAgentTurnTokens(d.dbPath, sess, r, sess.LastActiveAt)
	if err = s.SaveQueuedTurn(sess, queuedMessages); err != nil {
		return err
	}
	if streaming {
		ts.doneMeta = buildTurnDoneMeta(sess, quotaUnknown)
		ts.finish(r)
		return stillRunningExit(cmd, r, nil)
	}
	if !req.JSON && req.StreamMode == streamStats && !req.PlanFirst {
		ts = newTurnStream(cmd, streamStats, false)
		ts.began = turnStarted
		ts.doneMeta = buildTurnDoneMeta(sess, quotaUnknown)
		ts.finish(r)
		return stillRunningExit(cmd, r, nil)
	}
	if compacted {
		var ack string
		if r.Messages, ack = subagent.SplitCompactionAck(r.Messages); ack != "" {
			tl.log("compact", "agent acknowledged: %s", firstLine(ack))
		}
	}
	tl.finishTurn(r, sess.ID, "harnez agent resume "+sess.ID+" \"<prompt>\"")
	out := agentOutput{
		Session:              sess,
		Response:             r.Response,
		Messages:             r.Messages,
		ToolCalls:            r.ToolCalls,
		ToolCallLimit:        r.ToolCallLimit,
		ToolCallLimitReached: r.ToolCallLimitReached,
		PeakContextTokens:    r.PeakContextTokens,
	}
	if !resumed {
		out.Resumed, out.PreviousSessionID, out.Note = &resumed, previousID, freshNote
	}
	err = writeAgentOutput(cmd, req.JSON, out)
	return stillRunningExit(cmd, r, err)
}

// exitStillRunning is the exit code for "the wait ended, the session still
// runs", the same code harnez pr wait uses for "still open at the timeout".
const exitStillRunning = 3

// stillRunningExit keeps err, and otherwise turns a still-running turn
// result into a silent exit 3 after its output was printed (issue 1111).
func stillRunningExit(cmd *cobra.Command, r *subagent.TurnResult, err error) error {
	if err != nil || r == nil || !r.StillRunning {
		return err
	}
	return silenceIfExitCode(cmd, &exitCodeError{Code: exitStillRunning})
}

func writeAgentOutput(cmd *cobra.Command, jsonOut bool, v agentOutput) error {
	if jsonOut {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(v)
	}
	return printAgentMessages(cmd, v.Messages, v.Response)
}

// writeStoppedAgentOutput prints the one JSON document of a --json turn the
// token watchdog stopped: the stopped session and the peak the turn reached
// before the kill (ticket 1264 M2). The message goes to stderr via the error.
func writeStoppedAgentOutput(cmd *cobra.Command, sess *subagent.Session, partial *subagent.TurnResult) error {
	out := agentOutput{Session: sess, ReconnectCmd: "harnez agent resume " + sess.ID + " \"<prompt>\""}
	if partial != nil {
		out.PeakContextTokens = partial.PeakContextTokens
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
}

// partialTurn returns the usage a turn reported before the watchdog killed it,
// or nil when err carries none (ticket 1264 M2).
func partialTurn(err error) *subagent.TurnResult {
	var partialErr *subagent.PartialTurnError
	if errors.As(err, &partialErr) {
		return partialErr.Partial
	}
	return nil
}

// meterStoppedTurn records what a watchdog-stopped turn consumed before the
// kill: its quota readings always, and a turn record with the Claude totals
// when the stream reported calls (ticket 1264 M2). Call it before the session
// is saved; sess.Turn must already be turn.
func meterStoppedTurn(d agentDeps, sess *subagent.Session, turn int, partial *subagent.TurnResult, beforeCapture <-chan usage.TurnQuotaReading, turnStarted time.Time) {
	before := <-beforeCapture
	baselineCacheAt := turnQuotaBaseline(turnStarted, before)
	storeTurnQuota(d.dbPath, sess.ID, sess.Provider, turn, "before", before)
	after := recordTurnQuota(d.dbPath, d.quota, sess.ID, sess.Provider, turn, "after", false, turnStarted, baselineCacheAt)
	if partial == nil || len(partial.RequestUsage) == 0 {
		return
	}
	sess.TurnRecords = append(sess.TurnRecords, subagent.NewTurnRecord(turn, partial))
	addTurnTotals(sess, partial)
	sess.TokensSinceCompact += subagent.CompactionTokens(partial)
	populateTurnCostRecord(d.dbPath, sess, turn, partial, before, after)
	persistAgentTurnTokens(d.dbPath, sess, partial, time.Now())
}

// addTurnTotals adds a turn's token counts to the session totals for providers
// that report per-turn counts. The Codex cumulative branch stays with its caller.
func addTurnTotals(sess *subagent.Session, r *subagent.TurnResult) {
	if r.TurnTokensUnknown {
		return
	}
	sess.TokensTurn = r.TokensTurn
	sess.TokensCumulative += r.TokensTurn
	sess.ReasoningTokensTotal += r.ReasoningTokens
	sess.ReasoningTokensKnown = sess.ReasoningTokensKnown && r.ReasoningTokensKnown
	sess.InputTokensTotal += r.InputTokens
	sess.CachedTokensTotal += r.CachedTokens
	sess.CacheWriteTokensTotal += r.CacheWriteTokens
	sess.CacheWriteTokensKnown = sess.CacheWriteTokensKnown && r.CacheWriteTokensKnown
	sess.OutputTokensTotal += r.OutputTokens
	sess.TokenTotalsKnown = true
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
	result, err := driver.Compact(cmd.Context(), x.ProviderID(), subagent.Model{Provider: x.Provider, Name: x.Model, Tier: x.Tier})
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
func startRole(requested, prompt string, errOut io.Writer) (string, bool, error) {
	return agent.StartRole(requested, prompt, callerAgentRole(), errOut)
}

// resumeRole returns the stored role of sess (default role for old records),
// rejects a conflicting --role and checks the caller may manage that role.
func resumeRole(requested string, sess *subagent.Session) (string, error) {
	return agent.ResumeRole(requested, sess, callerAgentRole())
}

// setEnvWithRestore sets pairs in the process environment and returns a restore func.
func setEnvWithRestore(pairs [][2]string) func() {
	prev := make(map[string]*string, len(pairs))
	for _, kv := range pairs {
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

// setAgentEnv gives the provider process it is about to spawn its role and its
// session name (the parent id of anything it starts) and returns a restore func.
func setAgentEnv(role, name, model string) func() {
	return setEnvWithRestore([][2]string{{agentRoleEnv, role}, {agentSessionEnv, name}, {agentModelEnv, model}, {agentLauncherEnv, strconv.Itoa(os.Getpid())}})
}

func isPiProvider(provider string) bool {
	return provider == "pi" || provider == "lmcoder" || provider == "strata" || provider == "strata-coder"
}

func asStreamingDriverForToolLimit(d subagent.Driver) subagent.Driver {
	switch v := d.(type) {
	case subagent.ClaudeDriver:
		return subagent.ClaudeStreamDriver{ClaudeDriver: v}
	case subagent.PiDriver:
		return subagent.PiStreamDriver{PiDriver: v}
	default:
		return d
	}
}

func setToolCallEnv(cfg subagent.ToolCallConfig) func() {
	if cfg.Limit <= 0 {
		return func() {}
	}
	return setEnvWithRestore([][2]string{
		{"HARNEZ_TOOL_CALL_LIMIT", strconv.Itoa(cfg.Limit)},
		{"HARNEZ_TOOL_CALL_LIMIT_PROMPT", cfg.SynthesisPrompt},
	})
}

type toolCallGuard struct {
	cfg     subagent.ToolCallConfig
	limiter *subagent.ToolCallLimiter
	isPi    bool
	crossed bool
}

func newToolCallGuard(m subagent.Model, role string, flagMax int, flagSet bool, errOut io.Writer) (*toolCallGuard, error) {
	cfg, err := subagent.EffectiveToolCallConfig(m, role, flagMax, flagSet)
	if err != nil {
		return nil, err
	}
	if cfg.Limit <= 0 {
		return nil, nil
	}
	if m.Provider == "agy" {
		if errOut != nil {
			fmt.Fprintf(errOut, "harnez: warning: %s does not enforce --max-tool-calls; limit %d is informational only\n", m.Provider, cfg.Limit)
		}
		return nil, nil
	}
	isPi := isPiProvider(m.Provider)
	streamLimit := cfg.Limit
	if isPi {
		streamLimit = cfg.Limit + cfg.NativeBackstopCalls
	}
	return &toolCallGuard{
		cfg:     cfg,
		limiter: subagent.NewToolCallLimiter(streamLimit),
		isPi:    isPi,
	}, nil
}

func (g *toolCallGuard) observe(ev subagent.Event) bool {
	if g == nil {
		return false
	}
	if g.limiter.Observe(ev) {
		g.crossed = true
		return true
	}
	return false
}

func (g *toolCallGuard) attempted() int {
	if g == nil {
		return 0
	}
	return g.limiter.Attempted()
}

func (g *toolCallGuard) reached() bool {
	if g == nil {
		return false
	}
	return g.attempted() > g.cfg.Limit || g.crossed
}

func (g *toolCallGuard) attachResult(r *subagent.TurnResult) {
	if g == nil || r == nil {
		return
	}
	n := g.attempted()
	r.ToolCalls = &n
	r.ToolCallLimit = g.cfg.Limit
	r.ToolCallLimitReached = g.reached()
}

func (g *toolCallGuard) handleStoppedError(ctx context.Context, sd subagent.StreamingDriver, providerSessionID string, model subagent.Model, opts subagent.RunOptions, ts *turnStream, err error) (*subagent.TurnResult, *subagent.TurnResult, bool, error) {
	if g == nil {
		return nil, nil, false, err
	}
	var emptyErr *subagent.PiEmptyResponseError
	if errors.As(err, &emptyErr) && g.reached() {
		return nil, emptyErr.Result, true, err
	}
	if !g.crossed {
		return nil, nil, false, err
	}
	if g.isPi {
		return nil, nil, true, err
	}
	if sd != nil && providerSessionID != "" && ctx.Err() == nil {
		if ts != nil {
			ts.printf("[tool-call limit %d reached; requesting final answer]\n", g.cfg.Limit)
		}
		synthCtx, cancelSynth := context.WithCancel(ctx)
		defer cancelSynth()
		synthToolCalled := false
		synthRes, synthErr := sd.ResumeStream(synthCtx, providerSessionID, g.cfg.SynthesisPrompt, model, func(ev subagent.Event) {
			if ev.ToolCall {
				synthToolCalled = true
				g.limiter.Observe(ev)
				cancelSynth()
				return
			}
			if ts != nil {
				ts.onEvent(ev)
			}
		})
		if synthErr == nil && !synthToolCalled && synthRes != nil && strings.TrimSpace(synthRes.Response) != "" {
			return synthRes, nil, true, nil
		}
		var partial *subagent.TurnResult
		if synthRes != nil && len(synthRes.Messages) > 0 {
			partial = synthRes
		}
		return nil, partial, true, err
	}
	return nil, nil, true, err
}

func saveStoppedToolLimitTurn(s *subagent.FileSessionStore, sess *subagent.Session, turn int, partial *subagent.TurnResult, tcGuard *toolCallGuard, verb, target string) error {
	now := time.Now()
	sess.Status = "stopped"
	sess.ProcessPID, sess.ProcessStarttime = 0, 0
	sess.ProviderPID, sess.ProviderStarttime = 0, 0
	attempted := tcGuard.attempted()
	sess.LastError = fmt.Sprintf("tool-call limit reached: %d calls (limit %d), no answer; session can be resumed", attempted, tcGuard.cfg.Limit)
	sess.LastActiveAt = now
	sess.Turn = turn
	recordRes := &subagent.TurnResult{ContextTokens: -1}
	if partial != nil {
		if partial.SessionID != "" {
			sess.ProviderSessionID = partial.SessionID
		}
		copyRes := *partial
		copyRes.Response = ""
		recordRes = &copyRes
		if len(partial.Messages) > 0 {
			sess.Messages = append([]string(nil), partial.Messages...)
		}
		sess.Response = ""
		if partial.ContextTokens >= 0 {
			sess.ContextTokens = partial.ContextTokens
		}
		if partial.ContextWindowSize > 0 {
			sess.ContextWindowSize = partial.ContextWindowSize
		}
	} else {
		sess.Response = ""
	}
	tcGuard.attachResult(recordRes)
	sess.TurnRecords = append(sess.TurnRecords, subagent.NewTurnRecord(turn, recordRes))
	subagent.SyncCodexRolloutTotals(sess)
	if saveErr := s.Save(sess); saveErr != nil {
		return fmt.Errorf("agent %s %q %s; session recovery record failed: %w", verb, target, sess.LastError, saveErr)
	}
	return fmt.Errorf("agent %s %q %s", verb, target, sess.LastError)
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
	case "selftest":
		return nil // prints steps and writes its own state file; it never starts or controls agents (1107)
	}
	return fmt.Errorf("%w (role source: %s; worker session: %q; Codex thread: %q; launcher PID: %q). If this is an interactive host, check inherited Harnez variables and restart the host; Harnez cannot change the host-managed sandbox or writable roots", subagent.CheckSpawn(role, "developer"), agentRoleEnv, os.Getenv(agentSessionEnv), os.Getenv("CODEX_THREAD_ID"), os.Getenv(agentLauncherEnv))
}

// releaseFailedStartName frees a name held by a start that failed before any
// provider session existed (issue 1108): the old record keeps its ID and is
// renamed, never deleted. It reports whether the name is free now.
func releaseFailedStartName(s *subagent.FileSessionStore, existing *subagent.Session) bool {
	if existing.Status != "failed" || existing.ProviderSessionID != "" {
		return false
	}
	short := existing.ID
	if len(short) > 8 {
		short = short[:8]
	}
	existing.Name = existing.Name + "-failed-" + short
	return s.Save(existing) == nil
}

// suffixTakenName picks the first free "<name>-N" for a taken explicit --name
// and says so on stderr, so the host resumes by the real name (1186).
func suffixTakenName(s *subagent.FileSessionStore, name string, errOut io.Writer) (string, error) {
	sessions, err := s.List("", true)
	if err != nil {
		return "", err
	}
	taken := make(map[string]bool, len(sessions)*2)
	for _, existing := range sessions {
		taken[existing.Name] = true
		taken[existing.ID] = true
	}
	free, err := subagent.SuffixedName(name, func(c string) bool { return taken[c] })
	if err != nil {
		return "", err
	}
	if errOut != nil {
		fmt.Fprintf(errOut, "harnez: name %s taken; using %s\n", name, free)
	}
	return free, nil
}

func printAgentPreview(cmd *cobra.Command, model subagent.Model, role, name, dir, id, prompt, source, mode string, approve, resume, acceptCost bool, availability func(string, string) usage.ProviderQuotaAvailability) error {
	placeholder := fmt.Sprintf("<prompt: %d bytes from %s>", len(prompt), source)
	args, err := subagent.PreviewArgs(model, dir, id, placeholder, mode, approve, resume)
	if err != nil {
		return err
	}
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = strconv.Quote(arg)
	}
	acceptLine := agent.CostlyModelDryRunLine(model, acceptCost, availability, time.Now())
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "dry-run: %s\nrole: %s\nname: %s\ndir: %s\naccept-cost: %s\nargv: %s\n", model.Spec(), role, name, dir, acceptLine, strings.Join(quoted, " "))
	return err
}

// Package usagereport answers, for a date range or the weekly quota window,
// what Claude calls cost in API terms, how much of the weekly quota they used,
// and what the plan cost them (the subsidy factor). It is read-only: transcripts
// and quota history are read, nothing is written (ticket 1302 M1).
package usagereport

import (
	"math"
	"sort"
	"time"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usage"
)

const weeklyWindowLength = 7 * 24 * time.Hour

// Call is one deduplicated Claude API call. CostMicros is filled by Build; nil
// means unpriced (unknown model or no cache-write TTL split), never zero.
type Call struct {
	Session    string                `json:"session"`
	MessageID  string                `json:"message_id"`
	At         time.Time             `json:"at"`
	Usage      subagent.RequestUsage `json:"usage"`
	CostMicros *int64                `json:"cost_micros,omitempty"`
}

// Interval is one rise of the weekly quota between two quota-history samples
// of the same window. Start runs from the previous rise, or the window's first
// sample (the window start when that sample is 0%); it is clamped to --since
// for display. Share is each session's percentage points of the rise, split by
// its API cost in the rise; a rise with no priced call, or a quiet one (below
// min_local_factor), is not attributed and counts as "not explained locally".
type Interval struct {
	Start      time.Time          `json:"start"`
	End        time.Time          `json:"end"`
	DeltaPct   float64            `json:"delta_pct"`
	Attributed bool               `json:"attributed"`
	CostMicros map[string]int64   `json:"cost_micros_by_session,omitempty"`
	Share      map[string]float64 `json:"share_pct_by_session,omitempty"`
}

// SessionRow is one session's (or the totals') numbers. Nil means n/a.
type SessionRow struct {
	Session       string `json:"session"`
	Calls         int    `json:"calls"`
	Input         int64  `json:"input_tokens"`
	Output        int64  `json:"output_tokens"`
	CacheRead     int64  `json:"cache_read_tokens"`
	CacheWrite    int64  `json:"cache_write_tokens"`
	APICostMicros *int64 `json:"api_cost_micros_usd,omitempty"`
	// AttributedCostMicros is the API cost of calls inside a rise; the factor
	// uses this base. UnattributedCostMicros is the rest of API cost (API − ATTRIB):
	// quiet, before-first, after-older and pending calls together.
	AttributedCostMicros   *int64   `json:"attributed_api_cost_micros_usd,omitempty"`
	UnattributedCostMicros *int64   `json:"unattributed_api_cost_micros_usd,omitempty"`
	QuotaPct               *float64 `json:"quota_pct,omitempty"`
	// UnpricedCalls counts the session's calls without a price (M4): a session
	// with any has no API, attributed or pending cost and no factor.
	UnpricedCalls   int      `json:"unpriced_calls,omitempty"`
	PlanNetMicros   *int64   `json:"plan_net_micros_usd,omitempty"`
	PlanGrossMicros *int64   `json:"plan_gross_micros_usd,omitempty"`
	Factor          *float64 `json:"subsidy_factor,omitempty"`
}

// Options selects the range and the prices. WeeklyPlanMicros nil means the
// plan price is unknown; VATRate is the spec's plan_vat_rate.
type Options struct {
	Since, Until            time.Time
	Session                 string
	Catalog                 telemetry.PricingCatalog
	ResetTolerance          time.Duration
	WeeklyPlanMicros        *int64
	VATRate                 float64
	UnexplainedTolerancePct float64
	// MinSessionQuotaPct is usage_report.min_session_quota_pct: a session with
	// less attributed quota has no factor (M3).
	MinSessionQuotaPct float64
	// MinLocalFactor is usage_report.min_local_factor: a rise whose local cost
	// over its plan cost is below this is quiet, not explained locally (M4).
	// Zero turns the check off.
	MinLocalFactor float64
	// Location is the zone for the hour of day; nil means the local zone.
	Location *time.Location
}

// Buckets is where each priced in-range call's cost lands (M4). Every call is
// in exactly one bucket, so the five sum to the total priced API cost. Attributed
// is inside a rise that got a share. Quiet is inside a rise below
// min_local_factor. BeforeFirst is before a window's first quota sample.
// AfterOlder is after the last rise of an older window. Pending is after the
// newest window's last rise, outside every window, or inside a rise that ends
// after --until (that rise is outside the range).
type Buckets struct {
	Attributed  int64 `json:"attributed_api_cost_micros_usd"`
	Quiet       int64 `json:"quiet_rise_api_cost_micros_usd"`
	BeforeFirst int64 `json:"before_first_sample_api_cost_micros_usd"`
	AfterOlder  int64 `json:"after_older_window_api_cost_micros_usd"`
	Pending     int64 `json:"pending_api_cost_micros_usd"`
}

// WindowLine is all local calls in range over the whole weekly rise. Other
// machines add to the rise, so this is an upper bound on the local quota share.
type WindowLine struct {
	APICostMicros int64    `json:"api_cost_micros_usd"`
	RisePct       float64  `json:"weekly_rise_pct"`
	Factor        *float64 `json:"subsidy_factor,omitempty"`
}

// Report is the range result. Sessions is never nil.
type Report struct {
	Since          time.Time    `json:"since"`
	Until          time.Time    `json:"until"`
	Sessions       []SessionRow `json:"sessions"`
	Totals         SessionRow   `json:"totals"`
	RisePct        float64      `json:"weekly_rise_pct"`
	UnexplainedPct float64      `json:"not_explained_locally_pct"`
	Unexplained    bool         `json:"not_explained_locally"`
	Intervals      []Interval   `json:"intervals"`
	// Window is the all-calls line; nil when there is no weekly history or a
	// call is unpriced.
	Window *WindowLine `json:"window,omitempty"`
	// MinSessionQuotaPct is the threshold under which a session has no factor.
	MinSessionQuotaPct float64 `json:"min_session_quota_pct"`
	// Hours always has 24 rows, one per local hour of day (M2).
	Hours []HourRow `json:"hours"`
	// Series has one point per attributed weekly rise, in time order (M2).
	Series []SeriesPoint `json:"series"`
	// Session is the --session filter ("" for all). Hours, Series and Window
	// always cover all sessions (M4).
	Session string `json:"session,omitempty"`
	// PlanPriced is false when the weekly plan price is unknown: no factor.
	PlanPriced bool `json:"plan_priced"`
	// SkippedSamples counts Claude weekly rows with no reset_at (M4); they never
	// join or split a window.
	SkippedSamples int `json:"skipped_samples_no_reset,omitempty"`
	// UnpricedCalls counts in-range calls without a price, over all sessions.
	UnpricedCalls int `json:"unpriced_calls,omitempty"`
	// Buckets splits the priced in-range API cost (M4), over all sessions.
	Buckets Buckets `json:"buckets"`
}

// HourRow is one local hour of day. Nil means n/a: no attributed cost in the
// hour, or no quota rise, or no plan price for the factor.
type HourRow struct {
	Hour          int      `json:"hour"`
	Samples       int      `json:"samples"`
	APICostMicros *int64   `json:"api_cost_micros_usd,omitempty"`
	QuotaPct      *float64 `json:"quota_pct,omitempty"`
	Factor        *float64 `json:"subsidy_factor,omitempty"`
}

// SeriesPoint is one attributed weekly rise: its API cost, its quota points and
// the subsidy factor (nil without a plan price).
type SeriesPoint struct {
	Start         time.Time `json:"start"`
	End           time.Time `json:"end"`
	APICostMicros int64     `json:"api_cost_micros_usd"`
	QuotaPct      float64   `json:"quota_pct"`
	Factor        *float64  `json:"subsidy_factor,omitempty"`
}

// inRise is a rise inside the range with the priced calls assigned to it.
type inRise struct {
	iv    Interval
	calls []Call
}

// Build computes the report from calls and quota-history samples. Calls outside
// [Since, Until] are ignored. A rise is kept when it ends inside the range;
// attribution uses every session's calls, and Session only narrows the rows.
func Build(calls []Call, samples []usage.QuotaHistoryEntry, opts Options) Report {
	rep := Report{Since: opts.Since, Until: opts.Until, Sessions: []SessionRow{}, Session: opts.Session, PlanPriced: opts.WeeklyPlanMicros != nil}
	var inRange []Call
	for _, c := range calls {
		if c.At.IsZero() || c.At.Before(opts.Since) || c.At.After(opts.Until) {
			continue
		}
		c.CostMicros = priceCall(opts.Catalog, c)
		if c.CostMicros == nil {
			rep.UnpricedCalls++
		}
		inRange = append(inRange, c)
	}
	tl := buildTimeline(samples, opts.ResetTolerance)
	rep.SkippedSamples = tl.skipped
	keep := func(iv Interval) bool { return !iv.End.Before(opts.Since) && !iv.End.After(opts.Until) }

	// Every priced call lands in exactly one bucket (M4 item 3): a kept rise's
	// calls are attributed below, the rest are binned by the span that holds them.
	riseCalls := make([][]Call, len(tl.rises))
	for _, c := range inRange {
		if c.CostMicros == nil {
			continue
		}
		cost := *c.CostMicros
		sp, ok := tl.spanAt(c.At)
		switch {
		case ok && sp.kind == spanRise && keep(tl.rises[sp.rise]):
			riseCalls[sp.rise] = append(riseCalls[sp.rise], c)
		case ok && sp.kind == spanBeforeFirst:
			rep.Buckets.BeforeFirst += cost
		case ok && sp.kind == spanAfterOlder:
			rep.Buckets.AfterOlder += cost
		default:
			rep.Buckets.Pending += cost
		}
	}
	var rises []inRise
	for r, iv := range tl.rises {
		if !keep(iv) {
			continue
		}
		if iv.Start.Before(opts.Since) {
			iv.Start = opts.Since
		}
		calls := riseCalls[r]
		attribute(&iv, calls, opts)
		var total int64
		for _, c := range calls {
			total += *c.CostMicros
		}
		if iv.Attributed {
			rep.Buckets.Attributed += total
		} else {
			rep.Buckets.Quiet += total
		}
		rep.Intervals = append(rep.Intervals, iv)
		rises = append(rises, inRise{iv: iv, calls: calls})
		rep.RisePct += iv.DeltaPct
		if !iv.Attributed {
			rep.UnexplainedPct += iv.DeltaPct
		}
	}
	rep.Unexplained = rep.UnexplainedPct > opts.UnexplainedTolerancePct
	rep.Series, rep.Hours = hourlyView(rises, opts)

	rows := map[string]*SessionRow{}
	costs := map[string]int64{}
	var order []string
	for _, c := range inRange {
		row, ok := rows[c.Session]
		if !ok {
			row = &SessionRow{Session: c.Session}
			rows[c.Session] = row
			order = append(order, c.Session)
		}
		addTokens(row, c)
		if c.CostMicros == nil {
			row.UnpricedCalls++
		} else {
			costs[c.Session] += *c.CostMicros
		}
	}
	pct := map[string]float64{}
	attributed := map[string]int64{}
	for _, r := range rises {
		for s, share := range r.iv.Share {
			pct[s] += share
		}
		for s, c := range r.iv.CostMicros {
			attributed[s] += c
		}
	}
	hasHistory := len(rep.Intervals) > 0
	sort.Strings(order)
	rep.Totals = SessionRow{Session: "total"}
	var totalCost, totalAttr int64
	var totalPct float64
	totalUnpriced, anyAttributed := false, false
	for _, s := range order {
		if opts.Session != "" && s != opts.Session {
			continue
		}
		row := rows[s]
		priced := row.UnpricedCalls == 0
		if priced {
			cost := costs[s]
			row.APICostMicros = &cost
		}
		if p, ok := pct[s]; ok {
			row.QuotaPct = &p
			totalPct += p
			anyAttributed = true
		}
		if hasHistory && priced {
			a := attributed[s]
			unattr := costs[s] - a
			row.AttributedCostMicros = &a
			row.UnattributedCostMicros = &unattr
		}
		finishPlan(row, opts, true)
		rep.Sessions = append(rep.Sessions, *row)
		rep.Totals.Calls += row.Calls
		rep.Totals.Input += row.Input
		rep.Totals.Output += row.Output
		rep.Totals.CacheRead += row.CacheRead
		rep.Totals.CacheWrite += row.CacheWrite
		totalUnpriced = totalUnpriced || !priced
		totalCost += costs[s]
		totalAttr += attributed[s]
	}
	if !totalUnpriced {
		rep.Totals.APICostMicros = &totalCost
	}
	if hasHistory {
		a := totalAttr
		rep.Totals.AttributedCostMicros = &a
		if !totalUnpriced {
			unattr := totalCost - totalAttr
			rep.Totals.UnattributedCostMicros = &unattr
		}
	}
	if anyAttributed {
		rep.Totals.QuotaPct = &totalPct
	}
	finishPlan(&rep.Totals, opts, false)
	rep.Window = windowLine(rep.RisePct, inRange, hasHistory, opts)
	rep.MinSessionQuotaPct = opts.MinSessionQuotaPct
	return rep
}

// hourlyView credits each attributed rise's priced calls to their local hour,
// split by cost as attribute does. It returns one series point per attributed
// rise and the 24 hour rows. An hour's factor is its API cost over its quota
// share times the weekly plan.
func hourlyView(rises []inRise, opts Options) ([]SeriesPoint, []HourRow) {
	loc := opts.Location
	if loc == nil {
		loc = time.Local
	}
	var hourCost [24]int64
	var hourPct [24]float64
	var hourSeen [24]bool
	var hourSamples [24]int
	var series []SeriesPoint
	for _, r := range rises {
		iv := r.iv
		if !iv.Attributed {
			continue
		}
		var total int64
		for _, c := range iv.CostMicros {
			total += c
		}
		series = append(series, SeriesPoint{
			Start: iv.Start, End: iv.End, APICostMicros: total, QuotaPct: iv.DeltaPct,
			Factor: factorOf(total, iv.DeltaPct, opts.WeeklyPlanMicros),
		})
		var touched [24]bool
		for _, c := range r.calls {
			h := c.At.In(loc).Hour()
			hourCost[h] += *c.CostMicros
			hourPct[h] += iv.DeltaPct * float64(*c.CostMicros) / float64(total)
			hourSeen[h] = true
			if !touched[h] {
				touched[h] = true
				hourSamples[h]++
			}
		}
	}
	hours := make([]HourRow, 24)
	for h := range hours {
		row := HourRow{Hour: h, Samples: hourSamples[h]}
		if hourSeen[h] {
			cost, pct := hourCost[h], hourPct[h]
			row.APICostMicros = &cost
			row.QuotaPct = &pct
			row.Factor = factorOf(cost, pct, opts.WeeklyPlanMicros)
		}
		hours[h] = row
	}
	return series, hours
}

// factorOf is API cost over the plan cost of pct quota points (weekly plan
// price, net). It is nil without a plan price or without quota points.
func factorOf(cost int64, pct float64, weekly *int64) *float64 {
	if weekly == nil || pct <= 0 {
		return nil
	}
	f := float64(cost) / (pct / 100 * float64(*weekly))
	return &f
}

func addTokens(row *SessionRow, c Call) {
	row.Calls++
	row.Input += int64(c.Usage.Input)
	row.Output += int64(c.Usage.Output)
	row.CacheRead += int64(c.Usage.CacheRead)
	row.CacheWrite += int64(c.Usage.CacheWriteTotal)
}

// finishPlan fills the plan net and gross and the subsidy factor. Each needs
// its inputs known; otherwise the field stays nil (n/a).
//
// The factor is attributed API cost over the plan cost of the attributed pp (one
// base, M3). A session under MinSessionQuotaPct has no factor (perSession only).
func finishPlan(row *SessionRow, opts Options, perSession bool) {
	if row.QuotaPct == nil || opts.WeeklyPlanMicros == nil {
		return
	}
	net := int64(math.Round(*row.QuotaPct / 100 * float64(*opts.WeeklyPlanMicros)))
	gross := int64(math.Round(float64(net) * (1 + opts.VATRate)))
	row.PlanNetMicros = &net
	row.PlanGrossMicros = &gross
	if row.AttributedCostMicros == nil || net <= 0 {
		return
	}
	if perSession && *row.QuotaPct < opts.MinSessionQuotaPct {
		return
	}
	f := float64(*row.AttributedCostMicros) / float64(net)
	row.Factor = &f
}

// windowLine sums every local call in range (all sessions, priced) against the
// whole weekly rise. It is nil without weekly history or with an unpriced call.
func windowLine(risePct float64, calls []Call, hasHistory bool, opts Options) *WindowLine {
	if !hasHistory {
		return nil
	}
	var cost int64
	for _, c := range calls {
		if c.CostMicros == nil {
			return nil
		}
		cost += *c.CostMicros
	}
	return &WindowLine{APICostMicros: cost, RisePct: risePct, Factor: factorOf(cost, risePct, opts.WeeklyPlanMicros)}
}

// attribute splits a rise across sessions by the priced API cost of the calls
// assigned to it. A rise with no priced call is not attributed. A quiet rise
// (local cost below min_local_factor of its plan cost, M4 item 5) is not
// attributed either: it gets no share and the caller counts its cost as quiet.
func attribute(iv *Interval, calls []Call, opts Options) {
	var total int64
	costs := map[string]int64{}
	for _, c := range calls {
		costs[c.Session] += *c.CostMicros
		total += *c.CostMicros
	}
	if total == 0 {
		return
	}
	if f := factorOf(total, iv.DeltaPct, opts.WeeklyPlanMicros); f != nil && *f < opts.MinLocalFactor {
		return
	}
	iv.Attributed = true
	iv.CostMicros = costs
	iv.Share = map[string]float64{}
	for s, c := range costs {
		iv.Share[s] = iv.DeltaPct * float64(c) / float64(total)
	}
}

// priceCall prices one call with its own model (ticket 1301). A call without
// the cache-write TTL split, an unknown model or an unrated count is unpriced.
func priceCall(catalog telemetry.PricingCatalog, c Call) *int64 {
	if !c.Usage.CacheWriteSplitKnown {
		return nil
	}
	pricing, ok := telemetry.ClaudeCallPricing(catalog, c.Usage.Model)
	if !ok {
		return nil
	}
	cost, ok := telemetry.RequestCostMicros(pricing, telemetry.RequestTokens{
		Uncached:     int64(c.Usage.Input),
		CacheRead:    int64(c.Usage.CacheRead),
		CacheWrite5m: int64(c.Usage.CacheWrite5m),
		CacheWrite1h: int64(c.Usage.CacheWrite1h),
		Output:       int64(c.Usage.Output),
	})
	if !ok {
		return nil
	}
	return cost
}

// isClaudeWeekly selects the one series the report reads: Claude's all-models
// weekly window, keyed by (agent, group, window) with an empty group. Other
// series (agy groups, Claude session or Fable windows) never mix into it.
func isClaudeWeekly(s usage.QuotaHistoryEntry) bool {
	return s.Agent == usage.ClaudeAgentID && s.Group == "" && s.Window == usage.ClaudeWeeklyWindowName
}

// weeklyGroups orders the weekly samples and groups them into windows: a
// sample joins the previous one's window when their reset_at agree within the
// tolerance. An exact duplicate row is dropped. A row without reset_at is not
// a window member: it is counted and skipped (M4 item 2).
func weeklyGroups(samples []usage.QuotaHistoryEntry, tol time.Duration) ([][]usage.QuotaHistoryEntry, int) {
	var weekly []usage.QuotaHistoryEntry
	skipped := 0
	for _, s := range samples {
		if !isClaudeWeekly(s) {
			continue
		}
		if s.ResetAt == nil {
			skipped++
			continue
		}
		weekly = append(weekly, s)
	}
	sort.SliceStable(weekly, func(i, j int) bool { return weekly[i].Timestamp.Before(weekly[j].Timestamp) })
	var groups [][]usage.QuotaHistoryEntry
	for _, s := range weekly {
		if n := len(groups); n > 0 {
			g := groups[n-1]
			prev := g[len(g)-1]
			if prev.Timestamp.Equal(s.Timestamp) && prev.UsedPercent == s.UsedPercent && usage.SameResetMarker(prev.ResetAt, s.ResetAt, 0) {
				continue
			}
			if usage.SameResetMarker(prev.ResetAt, s.ResetAt, tol) {
				groups[n-1] = append(g, s)
				continue
			}
		}
		groups = append(groups, []usage.QuotaHistoryEntry{s})
	}
	return groups, skipped
}

// spanKind names what a stretch of a window's timeline means for the calls in it.
type spanKind int

const (
	spanBeforeFirst spanKind = iota // before a window's first sample (non-zero baseline)
	spanRise                        // from the previous rise (or the baseline) to a rise
	spanAfterOlder                  // after an older window's last rise, up to the next window
	spanPending                     // after the newest window's last rise: the open tail
)

// span is the half-open stretch [from, to) of the timeline; an open span runs on
// with no end. rise indexes timeline.rises for spanRise and is -1 otherwise.
type span struct {
	from, to time.Time
	open     bool
	kind     spanKind
	rise     int
}

// timeline is every window's rises and spans before any range filter. skipped
// counts Claude weekly rows without reset_at.
type timeline struct {
	rises   []Interval
	spans   []span // sorted by from
	skipped int
}

// buildTimeline measures each window's rises from its running peak (a dip and
// recovery is one rise, not two) and tiles each window's extent with spans: the
// stretch before a non-zero first sample, one stretch per rise, and the tail
// after the last rise, which ends at the next window's start (the newest window's
// tail is open). A reset is a new window, so no span crosses it.
func buildTimeline(samples []usage.QuotaHistoryEntry, tol time.Duration) timeline {
	groups, skipped := weeklyGroups(samples, tol)
	tl := timeline{skipped: skipped}
	for gi, g := range groups {
		ws := windowStart(g)
		// The first sample is the baseline: quota used before it is not measured,
		// unless it is 0%, when the window started with nothing used.
		cur := g[0].Timestamp
		if g[0].UsedPercent == 0 {
			cur = ws
		} else if ws.Before(cur) {
			tl.spans = append(tl.spans, span{from: ws, to: cur, kind: spanBeforeFirst, rise: -1})
		}
		peak := g[0]
		for _, b := range g[1:] {
			if !b.Timestamp.After(peak.Timestamp) || b.UsedPercent <= peak.UsedPercent {
				continue
			}
			tl.rises = append(tl.rises, Interval{Start: cur, End: b.Timestamp, DeltaPct: float64(b.UsedPercent - peak.UsedPercent)})
			tl.spans = append(tl.spans, span{from: cur, to: b.Timestamp, kind: spanRise, rise: len(tl.rises) - 1})
			peak, cur = b, b.Timestamp
		}
		if gi+1 < len(groups) {
			if next := windowStart(groups[gi+1]); next.After(cur) {
				tl.spans = append(tl.spans, span{from: cur, to: next, kind: spanAfterOlder, rise: -1})
			}
		} else {
			tl.spans = append(tl.spans, span{from: cur, open: true, kind: spanPending, rise: -1})
		}
	}
	sort.SliceStable(tl.spans, func(i, j int) bool { return tl.spans[i].from.Before(tl.spans[j].from) })
	return tl
}

// spanAt returns the span that holds t: the last span starting at or before t,
// when t is before its end. The second result is false when no span covers t.
func (tl timeline) spanAt(t time.Time) (span, bool) {
	i := sort.Search(len(tl.spans), func(i int) bool { return tl.spans[i].from.After(t) }) - 1
	if i < 0 {
		return span{}, false
	}
	sp := tl.spans[i]
	if sp.open || t.Before(sp.to) {
		return sp, true
	}
	return span{}, false
}

// windowStart is a window's first instant: its reset_at minus seven days, or
// its first sample when reset_at is unknown. It is never after the first sample.
func windowStart(g []usage.QuotaHistoryEntry) time.Time {
	first := g[0]
	if first.ResetAt != nil {
		if s := first.ResetAt.Add(-weeklyWindowLength); s.Before(first.Timestamp) {
			return s
		}
	}
	return first.Timestamp
}

// WeeklyWindowStart returns the start of the newest weekly window: its reset_at
// minus seven days. It reports false when there is no weekly history.
func WeeklyWindowStart(samples []usage.QuotaHistoryEntry, tol time.Duration) (time.Time, bool) {
	groups, _ := weeklyGroups(samples, tol)
	if len(groups) == 0 {
		return time.Time{}, false
	}
	last := groups[len(groups)-1]
	reset := last[len(last)-1].ResetAt
	if reset == nil {
		return time.Time{}, false
	}
	return reset.Add(-weeklyWindowLength), true
}

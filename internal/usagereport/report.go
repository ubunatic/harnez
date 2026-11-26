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
// of the same window. Share is each session's percentage points of the rise,
// split by its API cost in the interval; an interval with no priced call is
// not attributed and counts as "not explained locally".
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
	// uses this base. PendingCostMicros is the rest: calls with no quota step yet.
	AttributedCostMicros *int64   `json:"attributed_api_cost_micros_usd,omitempty"`
	PendingCostMicros    *int64   `json:"pending_api_cost_micros_usd,omitempty"`
	QuotaPct             *float64 `json:"quota_pct,omitempty"`
	PlanNetMicros        *int64   `json:"plan_net_micros_usd,omitempty"`
	PlanGrossMicros      *int64   `json:"plan_gross_micros_usd,omitempty"`
	Factor               *float64 `json:"subsidy_factor,omitempty"`
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
	// Location is the zone for the hour of day; nil means the local zone.
	Location *time.Location
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

// Build computes the report from calls and quota-history samples. Calls outside
// [Since, Until] are ignored, and so are intervals that are not fully inside it.
// Attribution uses every session's calls; Session only narrows the rows.
func Build(calls []Call, samples []usage.QuotaHistoryEntry, opts Options) Report {
	rep := Report{Since: opts.Since, Until: opts.Until, Sessions: []SessionRow{}}
	var inRange []Call
	for _, c := range calls {
		if c.At.IsZero() || c.At.Before(opts.Since) || c.At.After(opts.Until) {
			continue
		}
		c.CostMicros = priceCall(opts.Catalog, c)
		inRange = append(inRange, c)
	}
	for _, iv := range weeklyIntervals(samples, opts.ResetTolerance) {
		if iv.End.Before(opts.Since) || iv.End.After(opts.Until) {
			continue
		}
		attribute(&iv, inRange)
		rep.Intervals = append(rep.Intervals, iv)
		rep.RisePct += iv.DeltaPct
		if !iv.Attributed {
			rep.UnexplainedPct += iv.DeltaPct
		}
	}
	rep.Unexplained = rep.UnexplainedPct > opts.UnexplainedTolerancePct
	rep.Series, rep.Hours = hourlyView(rep.Intervals, inRange, opts)

	rows := map[string]*SessionRow{}
	costs := map[string]int64{}
	unpriced := map[string]bool{}
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
			unpriced[c.Session] = true
		} else {
			costs[c.Session] += *c.CostMicros
		}
	}
	pct := map[string]float64{}
	attributed := map[string]int64{}
	for _, iv := range rep.Intervals {
		for s, share := range iv.Share {
			pct[s] += share
		}
		for s, c := range iv.CostMicros {
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
		if unpriced[s] {
			row.APICostMicros = nil
		} else {
			cost := costs[s]
			row.APICostMicros = &cost
		}
		if p, ok := pct[s]; ok {
			row.QuotaPct = &p
			totalPct += p
			anyAttributed = true
		}
		if hasHistory {
			a := attributed[s]
			row.AttributedCostMicros = &a
			if !unpriced[s] {
				pend := costs[s] - a
				row.PendingCostMicros = &pend
			}
		}
		finishPlan(row, opts, true)
		rep.Sessions = append(rep.Sessions, *row)
		rep.Totals.Calls += row.Calls
		rep.Totals.Input += row.Input
		rep.Totals.Output += row.Output
		rep.Totals.CacheRead += row.CacheRead
		rep.Totals.CacheWrite += row.CacheWrite
		totalUnpriced = totalUnpriced || unpriced[s]
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
			p := totalCost - totalAttr
			rep.Totals.PendingCostMicros = &p
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

// hourlyView splits each attributed rise across its priced calls by cost, as
// attribute does, and credits each call's share to the local hour of that call.
// It returns one series point per attributed rise and the 24 hour rows. An
// hour's factor is its API cost over its quota share times the weekly plan.
func hourlyView(intervals []Interval, calls []Call, opts Options) ([]SeriesPoint, []HourRow) {
	loc := opts.Location
	if loc == nil {
		loc = time.Local
	}
	var hourCost [24]int64
	var hourPct [24]float64
	var hourSeen [24]bool
	var hourSamples [24]int
	var series []SeriesPoint
	for _, iv := range intervals {
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
		for _, c := range calls {
			if c.CostMicros == nil || !c.At.After(iv.Start) || c.At.After(iv.End) {
				continue
			}
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

// attribute splits the interval's rise across sessions by their priced API cost
// inside (Start, End].
func attribute(iv *Interval, calls []Call) {
	var total int64
	costs := map[string]int64{}
	for _, c := range calls {
		if c.CostMicros == nil || !c.At.After(iv.Start) || c.At.After(iv.End) {
			continue
		}
		costs[c.Session] += *c.CostMicros
		total += *c.CostMicros
	}
	if total == 0 {
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
// tolerance. An exact duplicate row is dropped.
func weeklyGroups(samples []usage.QuotaHistoryEntry, tol time.Duration) [][]usage.QuotaHistoryEntry {
	var weekly []usage.QuotaHistoryEntry
	for _, s := range samples {
		if isClaudeWeekly(s) {
			weekly = append(weekly, s)
		}
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
	return groups
}

// weeklyIntervals returns the rises of each window, measured from the highest
// value seen so far in that window: a dip and recovery (70, 69, 71) is one
// 1 pp rise, not a 2 pp rise from the dip. A reset is a new window, so no
// interval crosses it.
//
// An interval runs from the previous rise (or the window start for the first
// rise) to the sample that rises, so the calls made during a flat stretch fall
// into the rise that ends it (M3). The first sample is the baseline: quota used
// before it is not measured.
func weeklyIntervals(samples []usage.QuotaHistoryEntry, tol time.Duration) []Interval {
	var out []Interval
	for _, g := range weeklyGroups(samples, tol) {
		peak := g[0]
		start := windowStart(g)
		for _, b := range g[1:] {
			if !b.Timestamp.After(peak.Timestamp) || b.UsedPercent <= peak.UsedPercent {
				continue
			}
			out = append(out, Interval{Start: start, End: b.Timestamp, DeltaPct: float64(b.UsedPercent - peak.UsedPercent)})
			peak, start = b, b.Timestamp
		}
	}
	return out
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
	groups := weeklyGroups(samples, tol)
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

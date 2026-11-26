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
	Session         string   `json:"session"`
	Calls           int      `json:"calls"`
	Input           int64    `json:"input_tokens"`
	Output          int64    `json:"output_tokens"`
	CacheRead       int64    `json:"cache_read_tokens"`
	CacheWrite      int64    `json:"cache_write_tokens"`
	APICostMicros   *int64   `json:"api_cost_micros_usd,omitempty"`
	QuotaPct        *float64 `json:"quota_pct,omitempty"`
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
		if iv.Start.Before(opts.Since) || iv.End.After(opts.Until) {
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
	for _, iv := range rep.Intervals {
		for s, share := range iv.Share {
			pct[s] += share
		}
	}
	sort.Strings(order)
	rep.Totals = SessionRow{Session: "total"}
	var totalCost int64
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
		finishPlan(row, opts)
		rep.Sessions = append(rep.Sessions, *row)
		rep.Totals.Calls += row.Calls
		rep.Totals.Input += row.Input
		rep.Totals.Output += row.Output
		rep.Totals.CacheRead += row.CacheRead
		rep.Totals.CacheWrite += row.CacheWrite
		totalUnpriced = totalUnpriced || unpriced[s]
		totalCost += costs[s]
	}
	if !totalUnpriced {
		rep.Totals.APICostMicros = &totalCost
	}
	if anyAttributed {
		rep.Totals.QuotaPct = &totalPct
	}
	finishPlan(&rep.Totals, opts)
	return rep
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
func finishPlan(row *SessionRow, opts Options) {
	if row.QuotaPct == nil || opts.WeeklyPlanMicros == nil {
		return
	}
	net := int64(math.Round(*row.QuotaPct / 100 * float64(*opts.WeeklyPlanMicros)))
	gross := int64(math.Round(float64(net) * (1 + opts.VATRate)))
	row.PlanNetMicros = &net
	row.PlanGrossMicros = &gross
	if row.APICostMicros != nil && net > 0 {
		f := float64(*row.APICostMicros) / float64(net)
		row.Factor = &f
	}
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

// weeklyGroups orders the weekly samples and groups them into windows: a
// sample joins the previous one's window when their reset_at agree within the
// tolerance. An exact duplicate row is dropped.
func weeklyGroups(samples []usage.QuotaHistoryEntry, tol time.Duration) [][]usage.QuotaHistoryEntry {
	var weekly []usage.QuotaHistoryEntry
	for _, s := range samples {
		if usage.IsWeeklyQuotaWindow(s.Window) {
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

// weeklyIntervals returns each rise between consecutive samples of one window.
// A fall is not a rise; a reset is a new window, so no interval crosses it.
func weeklyIntervals(samples []usage.QuotaHistoryEntry, tol time.Duration) []Interval {
	var out []Interval
	for _, g := range weeklyGroups(samples, tol) {
		for i := 1; i < len(g); i++ {
			a, b := g[i-1], g[i]
			if !b.Timestamp.After(a.Timestamp) || b.UsedPercent <= a.UsedPercent {
				continue
			}
			out = append(out, Interval{Start: a.Timestamp, End: b.Timestamp, DeltaPct: float64(b.UsedPercent - a.UsedPercent)})
		}
	}
	return out
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

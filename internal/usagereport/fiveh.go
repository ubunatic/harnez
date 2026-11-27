package usagereport

import (
	"sort"
	"time"

	"ubunatic.com/harnez/internal/usage"
)

// windowShare is the 5-hour window's measured rises in range: their pp, the pp no
// call explains, and per session the attributed pp, API cost and solo pp. ratio is
// the range ratio (5-hour pp per weekly pp), ratioSet false when no window qualifies.
type windowShare struct {
	pct, notExplained float64
	shares            map[string]float64
	costs             map[string]int64
	solo              map[string]float64
	intervals         int
	ratio             float64
	ratioWindows      int
	ratioSet          bool
	models            map[string]*modelAcc
}

// modelAcc is one model's attributed 5-hour rises: its cost and pp, and the same
// for solo rises (only this model had calls), which alone give its factor.
type modelAcc struct {
	cost, soloCost int64
	pp, soloPP     float64
	soloN          int
}

func (w windowShare) ratioOK() bool { return w.ratioSet }

func (w windowShare) ratioPtr() *float64 {
	if !w.ratioSet {
		return nil
	}
	r := w.ratio
	return &r
}

// weeklyPath is the weekly window's used-percent path (ticket 1308 M2 pre-work 1):
// at any instant, the running peak of the weekly window that instant is in.
type weeklyPath struct {
	at    []time.Time
	group []int
	peak  []float64
}

func newWeeklyPath(samples []usage.QuotaHistoryEntry, tol time.Duration) weeklyPath {
	groups, _ := windowGroups(samples, tol, weeklySel)
	var p weeklyPath
	for gi, g := range groups {
		var peak float64
		for _, s := range g {
			peak = max(peak, float64(s.UsedPercent))
			p.at = append(p.at, s.Timestamp)
			p.group = append(p.group, gi)
			p.peak = append(p.peak, peak)
		}
	}
	return p
}

// valueAt returns the weekly peak at t (the last weekly sample at or before t) and
// the weekly window it belongs to. ok is false before the first weekly sample.
func (p weeklyPath) valueAt(t time.Time) (peak float64, group int, ok bool) {
	i := sort.Search(len(p.at), func(i int) bool { return p.at[i].After(t) }) - 1
	if i < 0 {
		return 0, 0, false
	}
	return p.peak[i], p.group[i], true
}

// fiveWindow is one 5-hour window: its first and last sample, the pp of its own
// 5-hour rises, and its weekly rise from the previous window's last sample. The
// windows partition the timeline, so their weekly rises add up (pre-work 1).
type fiveWindow struct {
	first, last time.Time
	pp          float64
	weekly      float64
	weeklyOK    bool
}

// fiveHourWindows lists the 5-hour windows in time order. A window's weekly rise is
// the weekly peak at its last sample minus the peak at the previous boundary; when
// the weekly window changed in between, the rise is the new window's peak alone.
func fiveHourWindows(samples []usage.QuotaHistoryEntry, rises []Interval, opts Options) []fiveWindow {
	groups, _ := windowGroups(samples, opts.ResetTolerance, fiveHourSel)
	path := newWeeklyPath(samples, opts.ResetTolerance)
	wins := make([]fiveWindow, len(groups))
	for k, g := range groups {
		w := fiveWindow{first: g[0].Timestamp, last: g[len(g)-1].Timestamp}
		for _, r := range rises {
			if !r.End.Before(w.first) && !r.End.After(w.last) {
				w.pp += r.DeltaPct
			}
		}
		wins[k] = w
	}
	prevAt := time.Time{}
	if len(wins) > 0 {
		prevAt = wins[0].first
	}
	for k := range wins {
		v, gi, ok := path.valueAt(wins[k].last)
		pv, pg, pok := path.valueAt(prevAt)
		if ok && pok {
			wins[k].weeklyOK = true
			wins[k].weekly = v
			if gi == pg {
				wins[k].weekly = v - pv
			}
		}
		prevAt = wins[k].last
	}
	return wins
}

// fiveHourRatio is Σ 5-hour pp ÷ Σ weekly pp over the windows in range whose weekly
// rise is at least min_ratio_weekly_pct. Capped windows count. Windows are summed,
// never converted one by one.
func fiveHourRatio(wins []fiveWindow, opts Options) (ratio float64, n int, ok bool) {
	var pp, weekly float64
	for _, w := range wins {
		if !w.weeklyOK || w.weekly <= 0 || w.weekly < opts.MinRatioWeeklyPct {
			continue
		}
		if w.last.Before(opts.Since) || w.last.After(opts.Until) {
			continue
		}
		pp += w.pp
		weekly += w.weekly
		n++
	}
	if n == 0 || weekly <= 0 || pp <= 0 {
		return 0, n, false
	}
	return pp / weekly, n, true
}

// fiveHourShare measures the 5-hour window's rises through the same timeline as
// the weekly report and splits each attributed rise across sessions by cost. The
// quiet check uses weekly-equivalent pp (5-hour pp ÷ ratio, M2). Without a ratio it
// is off. A rise is kept when it ends inside the range, as weekly rises are.
func fiveHourShare(samples []usage.QuotaHistoryEntry, inRange []Call, keep func(Interval) bool, opts Options) windowShare {
	tl := buildTimeline(samples, opts.ResetTolerance, fiveHourSel)
	riseCalls := make([][]Call, len(tl.rises))
	for _, c := range inRange {
		if c.CostMicros == nil {
			continue
		}
		if sp, ok := tl.spanAt(c.At); ok && sp.kind == spanRise {
			riseCalls[sp.rise] = append(riseCalls[sp.rise], c)
		}
	}
	ratio, n, ok := fiveHourRatio(fiveHourWindows(samples, tl.rises, opts), opts)
	out := windowShare{
		shares: map[string]float64{}, costs: map[string]int64{}, solo: map[string]float64{},
		models: map[string]*modelAcc{}, ratio: ratio, ratioWindows: n, ratioSet: ok,
	}
	local := opts
	if !ok {
		local.MinLocalFactor = 0
	}
	for r, iv := range tl.rises {
		if !keep(iv) {
			continue
		}
		out.intervals++
		out.pct += iv.DeltaPct
		quietPct := iv.DeltaPct
		if ok {
			quietPct = iv.DeltaPct / ratio
		}
		attributeWith(&iv, riseCalls[r], local, quietPct)
		if !iv.Attributed {
			out.notExplained += iv.DeltaPct
			continue
		}
		addFiveRise(&out, iv, riseCalls[r])
	}
	return out
}

// addFiveRise adds one attributed 5-hour rise to the sessions and models it touches.
func addFiveRise(out *windowShare, iv Interval, calls []Call) {
	for s, sh := range iv.Share {
		out.shares[s] += sh
		if len(iv.CostMicros) == 1 {
			out.solo[s] += sh
		}
	}
	for s, c := range iv.CostMicros {
		out.costs[s] += c
	}
	models := map[string]int64{}
	for _, c := range calls {
		models[c.Usage.Model] += *c.CostMicros
	}
	for m, c := range models {
		acc := out.models[m]
		if acc == nil {
			acc = &modelAcc{}
			out.models[m] = acc
		}
		acc.cost += c
		acc.pp += iv.DeltaPct * float64(c) / float64(sumCosts(models))
		if len(models) == 1 {
			acc.soloCost += c
			acc.soloPP += iv.DeltaPct
			acc.soloN++
		}
	}
}

func sumCosts(m map[string]int64) int64 {
	var t int64
	for _, c := range m {
		t += c
	}
	return t
}

// soloShares sums each session's share from the attributed weekly rises where it
// was the only session with calls. basisOf reads it as "measured" when it is at
// least half the session's share.
func soloShares(rises []inRise) map[string]float64 {
	solo := map[string]float64{}
	for _, r := range rises {
		if !r.iv.Attributed || len(r.iv.CostMicros) != 1 {
			continue
		}
		for s, sh := range r.iv.Share {
			solo[s] += sh
		}
	}
	return solo
}

// basisOf names a factor's basis: "measured" when at least half of its share (total
// pp) comes from solo rises (solo pp), else "split" (M2).
func basisOf(solo, total float64) string {
	if 2*solo >= total {
		return "measured"
	}
	return "split"
}

// finishFiveHour fills a session's weekly-equivalent 5-hour pp and its 5-hour
// factor. The factor needs the ratio, the plan price, the session's 5-hour share at
// least min_session_five_hour_pct and priced calls. Otherwise it stays n/a.
func finishFiveHour(row *SessionRow, five windowShare, opts Options) {
	if row.FiveHourQuotaPct == nil || !five.ratioOK() {
		return
	}
	pp := *row.FiveHourQuotaPct
	eq := pp / five.ratio
	row.FiveHourWeeklyEqPct = &eq
	if opts.WeeklyPlanMicros == nil || row.UnpricedCalls > 0 || pp < opts.MinSessionFiveHourPct {
		return
	}
	cost, ok := five.costs[row.Session]
	if !ok {
		return
	}
	row.FiveHourFactor = factorOf(cost, eq, opts.WeeklyPlanMicros)
	if row.FiveHourFactor != nil {
		row.FiveHourFactorBasis = basisOf(five.solo[row.Session], pp)
	}
}

// ModelRow is one model's rollup over the range (M2). Its 5-hour numbers come from
// attributed 5-hour rises; its factor from solo rises of that model only.
type ModelRow struct {
	Model                string   `json:"model"`
	Calls                int      `json:"calls"`
	APICostMicros        *int64   `json:"api_cost_micros_usd,omitempty"`
	AttributedCostMicros int64    `json:"attributed_api_cost_micros_usd"`
	FiveHourQuotaPct     float64  `json:"five_hour_quota_pct"`
	FiveHourWeeklyEqPct  *float64 `json:"five_hour_weekly_eq_pct,omitempty"`
	PlanNetMicros        *int64   `json:"plan_net_micros_usd,omitempty"`
	SoloRises            int      `json:"solo_rises"`
	Factor               *float64 `json:"subsidy_factor,omitempty"`
}

// modelRows rolls the range up per model, sorted by name. The weekly-equivalent
// pp, plan and factor are nil without a ratio; the factor is also nil without solo
// rises of that model.
func modelRows(inRange []Call, five windowShare, opts Options) []ModelRow {
	byModel := map[string]*ModelRow{}
	costs := map[string]int64{}
	unpriced := map[string]bool{}
	for _, c := range inRange {
		m := c.Usage.Model
		row, ok := byModel[m]
		if !ok {
			row = &ModelRow{Model: m}
			byModel[m] = row
		}
		row.Calls++
		if c.CostMicros == nil {
			unpriced[m] = true
			continue
		}
		costs[m] += *c.CostMicros
	}
	var out []ModelRow
	for m, row := range byModel {
		if !unpriced[m] {
			cost := costs[m]
			row.APICostMicros = &cost
		}
		if acc := five.models[m]; acc != nil {
			row.AttributedCostMicros = acc.cost
			row.FiveHourQuotaPct = acc.pp
			row.SoloRises = acc.soloN
			if five.ratioOK() {
				eq := acc.pp / five.ratio
				row.FiveHourWeeklyEqPct = &eq
				if opts.WeeklyPlanMicros != nil {
					net := int64(float64(*opts.WeeklyPlanMicros) * eq / 100)
					row.PlanNetMicros = &net
				}
				if acc.soloPP > 0 {
					row.Factor = factorOf(acc.soloCost, acc.soloPP/five.ratio, opts.WeeklyPlanMicros)
				}
			}
		}
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	return out
}

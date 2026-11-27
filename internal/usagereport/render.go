package usagereport

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"text/tabwriter"
)

// RenderJSON writes the report as indented JSON.
func RenderJSON(w io.Writer, rep Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

// RenderText writes the report as an aligned table. Money is micro-USD shown
// in dollars; a nil field prints "n/a", never 0.
func RenderText(w io.Writer, rep Report, unexplainedTolerancePct float64) error {
	fmt.Fprintf(w, "Claude usage %s .. %s (UTC). Plan prices are net USD; gross adds VAT.\n",
		rep.Since.UTC().Format("2006-01-02 15:04"), rep.Until.UTC().Format("2006-01-02 15:04"))
	if len(rep.Sessions) == 0 {
		fmt.Fprintln(w, "no Claude calls in range")
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "SESSION\tCALLS\tIN\tOUT\tCACHE-READ\tCACHE-WRITE\tAPI $\tATTRIB $\tUNATTRIB $\tQUOTA %\tQUOTA 5H %\t5H W-EQ %\tPLAN NET $\tPLAN GROSS $\tFACTOR\t5H FACTOR\t")
	for _, r := range rep.Sessions {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t\n",
			r.Session, r.Calls, r.Input, r.Output, r.CacheRead, r.CacheWrite,
			money(r.APICostMicros), money(r.AttributedCostMicros), money(r.UnattributedCostMicros),
			percent(r.QuotaPct), percent(r.FiveHourQuotaPct), percent(r.FiveHourWeeklyEqPct),
			money(r.PlanNetMicros), money(r.PlanGrossMicros),
			sessionFactor(r, rep), fiveHourFactor(r, rep))
	}
	t := rep.Totals
	fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t\n",
		t.Session, t.Calls, t.Input, t.Output, t.CacheRead, t.CacheWrite,
		money(t.APICostMicros), money(t.AttributedCostMicros), money(t.UnattributedCostMicros),
		percent(t.QuotaPct), percent(t.FiveHourQuotaPct), percent(t.FiveHourWeeklyEqPct),
		money(t.PlanNetMicros), money(t.PlanGrossMicros), factor(t.Factor), factor(t.FiveHourFactor))
	if err := tw.Flush(); err != nil {
		return err
	}
	if rep.FiveHourIntervals > 0 {
		fmt.Fprintf(w, "5-hour window: %d rise(s), %.2f pp in range, %.2f pp not explained locally; %s\n",
			rep.FiveHourIntervals, rep.FiveHourRisePct, rep.FiveHourNotExplainedPct, fiveHourRatioLine(rep))
	}
	if rep.Window != nil {
		fmt.Fprintf(w, "window (all local calls / weekly rise %.2f pp): API $%s, factor %s; an upper bound on local quota, other machines add to the rise\n",
			rep.Window.RisePct, money(&rep.Window.APICostMicros), factor(rep.Window.Factor))
	}
	if b := rep.Buckets; b.BeforeFirst+b.AfterOlder+b.Pending+b.Quiet > 0 {
		fmt.Fprintf(w, "not in a quota rise: before first quota sample $%s, after an older window $%s, pending (no quota step yet) $%s, quiet rises (below min_local_factor) $%s; no share, not in the factor\n",
			money(&b.BeforeFirst), money(&b.AfterOlder), money(&b.Pending), money(&b.Quiet))
	}
	if rep.Session != "" {
		fmt.Fprintf(w, "--session %s: the window, hours and series cover all sessions\n", rep.Session)
	}
	if rep.SkippedSamples > 0 {
		fmt.Fprintf(w, "skipped samples: %d Claude weekly rows without reset_at (they never join or split a window)\n", rep.SkippedSamples)
	}
	if rep.UnpricedCalls > 0 {
		fmt.Fprintf(w, "unpriced calls: %d (no price); their sessions show n/a for API $, ATTRIB $, UNATTRIB $ and FACTOR\n", rep.UnpricedCalls)
	}
	fmt.Fprintln(w, "legend: ATTRIB $ = API cost of calls in a quota rise (the factor's base); UNATTRIB $ = API $ - ATTRIB $ (quiet, before first sample, after an older window, pending); FACTOR = ATTRIB $ / plan net for the same quota %")
	if rep.FiveHourIntervals > 0 {
		fmt.Fprintln(w, "legend: 5H W-EQ % = 5-hour pp / range ratio (weekly-equivalent pp); 5H FACTOR = API $ / plan net for the W-EQ %; basis measured = most of the share came from calls alone in their rise, split = shared with other sessions or models")
	}
	if len(rep.Intervals) == 0 {
		fmt.Fprintln(w, "quota: no weekly history in range; quota, plan and factor are n/a")
	}
	if err := renderModels(w, rep); err != nil {
		return err
	}
	if err := renderHours(w, rep); err != nil {
		return err
	}
	if rep.Unexplained {
		fmt.Fprintf(w, "not explained locally: %.1f pp of the weekly rise had no priced local call (tolerance %.1f pp)\n",
			rep.UnexplainedPct, unexplainedTolerancePct)
	}
	return nil
}

// renderHours writes the factor by local hour (24 rows, n/a where no data) and
// the factor over time as a sparkline of the attributed rises.
func renderHours(w io.Writer, rep Report) error {
	if len(rep.Series) == 0 {
		fmt.Fprintln(w, "factor by local hour: no attributed quota rises in range; factor is n/a")
		return nil
	}
	fmt.Fprintln(w, "factor by local hour (samples = attributed rises touching the hour):")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "HOUR\tSAMPLES\tAPI $\tQUOTA %\tFACTOR\t")
	for _, h := range rep.Hours {
		fmt.Fprintf(tw, "%02d:00\t%d\t%s\t%s\t%s\t\n", h.Hour, h.Samples, money(h.APICostMicros), percent(h.QuotaPct), factor(h.Factor))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	var vals []float64
	for _, p := range rep.Series {
		if p.Factor != nil {
			vals = append(vals, *p.Factor)
		}
	}
	if len(vals) == 0 {
		fmt.Fprintln(w, "factor over time: n/a (no plan price)")
		return nil
	}
	lo, hi := vals[0], vals[0]
	for _, v := range vals {
		lo, hi = min(lo, v), max(hi, v)
	}
	fmt.Fprintf(w, "factor over time (%d rises, %.2fx to %.2fx): %s\n", len(vals), lo, hi, sparkline(vals))
	return nil
}

// sparkline maps values onto eight block glyphs between their min and max. A
// flat series gets the middle glyph.
func sparkline(vals []float64) string {
	if len(vals) == 0 {
		return ""
	}
	glyphs := []rune("▁▂▃▄▅▆▇█")
	lo, hi := vals[0], vals[0]
	for _, v := range vals {
		lo, hi = min(lo, v), max(hi, v)
	}
	var b strings.Builder
	for _, v := range vals {
		i := 3
		if hi > lo {
			i = int(math.Round((v - lo) / (hi - lo) * 7))
		}
		b.WriteRune(glyphs[i])
	}
	return b.String()
}

func money(v *int64) string {
	if v == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.4f", float64(*v)/1e6)
}

func percent(v *float64) string {
	if v == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.2f", *v)
}

// sessionFactor prints a session's factor. Without a plan price it is a plain
// "n/a". With a plan price, "n/a (< N pp)" marks a session whose attributed quota
// is below the threshold (usage_report.min_session_quota_pct).
func sessionFactor(r SessionRow, rep Report) string {
	if r.Factor == nil && rep.PlanPriced && r.QuotaPct != nil && *r.QuotaPct < rep.MinSessionQuotaPct {
		return fmt.Sprintf("n/a (< %g pp)", rep.MinSessionQuotaPct)
	}
	return factor(r.Factor)
}

// fiveHourRatioLine words the range ratio, or says why there is none.
func fiveHourRatioLine(rep Report) string {
	if rep.FiveHourRatio == nil {
		return fmt.Sprintf("no range ratio (no 5-hour window with a weekly rise of at least %g pp), so no weekly-equivalent pp, 5-hour factor or quiet check", rep.MinRatioWeeklyPct)
	}
	eq := 0.0
	if rep.Totals.FiveHourWeeklyEqPct != nil {
		eq = *rep.Totals.FiveHourWeeklyEqPct
	}
	attrib := 0.0
	if rep.Totals.QuotaPct != nil {
		attrib = *rep.Totals.QuotaPct
	}
	return fmt.Sprintf("range ratio %.3f 5-hour pp per weekly pp from %d window(s) with a weekly rise of at least %g pp; weekly-equivalent %.2f pp vs %.2f pp weekly attributed (gap %.2f pp)",
		*rep.FiveHourRatio, rep.FiveHourRatioWindows, rep.MinRatioWeeklyPct, eq, attrib, eq-attrib)
}

// fiveHourFactor prints a session's 5-hour factor with its basis. "n/a" stays plain.
func fiveHourFactor(r SessionRow, rep Report) string {
	if r.FiveHourFactor == nil {
		if r.FiveHourWeeklyEqPct != nil && r.FiveHourQuotaPct != nil && *r.FiveHourQuotaPct < rep.MinSessionFiveHourPct {
			return fmt.Sprintf("n/a (< %g pp)", rep.MinSessionFiveHourPct)
		}
		return "n/a"
	}
	return fmt.Sprintf("%.2fx %s", *r.FiveHourFactor, r.FiveHourFactorBasis)
}

// renderModels writes the per-model rollup of the range (M2).
func renderModels(w io.Writer, rep Report) error {
	if len(rep.Models) == 0 {
		return nil
	}
	fmt.Fprintln(w, "models in range:")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "MODEL\tCALLS\tAPI $\tATTRIB $\tQUOTA 5H %\t5H W-EQ %\tPLAN NET $\tSOLO RISES\tFACTOR\t")
	for _, m := range rep.Models {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%.2f\t%s\t%s\t%d\t%s\t\n",
			m.Model, m.Calls, money(m.APICostMicros), money(&m.AttributedCostMicros), m.FiveHourQuotaPct,
			percent(m.FiveHourWeeklyEqPct), money(m.PlanNetMicros), m.SoloRises, factor(m.Factor))
	}
	return tw.Flush()
}

func factor(v *float64) string {
	if v == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.2fx", *v)
}

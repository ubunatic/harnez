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
	fmt.Fprintln(tw, "SESSION\tCALLS\tIN\tOUT\tCACHE-READ\tCACHE-WRITE\tAPI $\tQUOTA %\tPLAN NET $\tPLAN GROSS $\tFACTOR\t")
	for _, r := range append(append([]SessionRow{}, rep.Sessions...), rep.Totals) {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%s\t%s\t%s\t%s\t%s\t\n",
			r.Session, r.Calls, r.Input, r.Output, r.CacheRead, r.CacheWrite,
			money(r.APICostMicros), percent(r.QuotaPct), money(r.PlanNetMicros), money(r.PlanGrossMicros), factor(r.Factor))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(rep.Intervals) == 0 {
		fmt.Fprintln(w, "quota: no weekly history in range; quota, plan and factor are n/a")
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

func factor(v *float64) string {
	if v == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.2fx", *v)
}

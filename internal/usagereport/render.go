package usagereport

import (
	"encoding/json"
	"fmt"
	"io"
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
	if rep.Unexplained {
		fmt.Fprintf(w, "not explained locally: %.1f pp of the weekly rise had no priced local call (tolerance %.1f pp)\n",
			rep.UnexplainedPct, unexplainedTolerancePct)
	}
	return nil
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

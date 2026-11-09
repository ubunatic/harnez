package assess

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// RenderJSON serializes the AssessmentReport into formatted JSON.
func RenderJSON(report *AssessmentReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RenderText renders a concise human-readable terminal report.
func RenderText(report *AssessmentReport) string {
	var b strings.Builder

	targetName := report.Target
	base := filepath.Base(targetName)

	if report.IsFile {
		fmt.Fprintf(&b, "── File Assessment: %s ──────────────────────────────\n", base)
		if len(report.Files) > 0 {
			f := report.Files[0]
			fmt.Fprintf(&b, "Language:     %s (%s)\n", f.Language, f.Category)
			fmt.Fprintf(&b, "Size:         %d lines, ~%d tokens (%d bytes)\n", f.Lines, f.Tokens, f.Bytes)
		}
		if len(report.Warnings) > 0 {
			b.WriteString("Warnings:\n")
			for _, w := range report.Warnings {
				fmt.Fprintf(&b, "  • %s\n", w)
			}
		} else {
			b.WriteString("Status:       ✓ Clean (no warnings)\n")
		}
		return b.String()
	}

	// Directory / Repo rendering
	fmt.Fprintf(&b, "── Repository Assessment: %s ──────────────────────────────\n", base)
	fmt.Fprintf(&b, "%-16s %7s %15s %15s    %-20s\n", "Category", "Files", "Lines (code)", "Tokens (est)", "Health")

	for _, c := range report.Categories {
		linesStr := formatNumber(c.Lines)
		tokensStr := formatNumber(c.Tokens)
		fmt.Fprintf(&b, "%-16s %7d %15s %15s    %-20s\n", c.Name, c.FilesCount, linesStr, tokensStr, c.Health)
	}

	b.WriteString("────────────────────────────────────────────────────────────────────────\n")
	if report.RAMP != nil {
		fmt.Fprintf(&b, "RAMP Level:   %s (%s) [Baseline: %s, Projected: %s]\n",
			report.RAMP.BaselineLevel, LevelDescription(report.RAMP.BaselineLevel),
			report.RAMP.BaselineLevel, report.RAMP.ProjectedLevel)
	}
	if report.FeasibilityDesc != "" {
		fmt.Fprintf(&b, "Feasibility:  %s (%s)\n", report.Feasibility, report.FeasibilityDesc)
	} else {
		fmt.Fprintf(&b, "Feasibility:  %s\n", report.Feasibility)
	}

	b.WriteString("Warnings:\n")
	if len(report.Warnings) == 0 {
		b.WriteString("  • none\n")
	} else {
		for _, w := range report.Warnings {
			fmt.Fprintf(&b, "  • %s\n", w)
		}
	}

	return b.String()
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	in := fmt.Sprintf("%d", n)
	var out []byte
	l := len(in)
	for i, c := range []byte(in) {
		if i > 0 && (l-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

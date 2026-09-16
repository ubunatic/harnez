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
		b.WriteString(fmt.Sprintf("── File Assessment: %s ──────────────────────────────\n", base))
		if len(report.Files) > 0 {
			f := report.Files[0]
			b.WriteString(fmt.Sprintf("Language:     %s (%s)\n", f.Language, f.Category))
			b.WriteString(fmt.Sprintf("Size:         %d lines, ~%d tokens (%d bytes)\n", f.Lines, f.Tokens, f.Bytes))
		}
		if len(report.Warnings) > 0 {
			b.WriteString("Warnings:\n")
			for _, w := range report.Warnings {
				b.WriteString(fmt.Sprintf("  • %s\n", w))
			}
		} else {
			b.WriteString("Status:       ✓ Clean (no warnings)\n")
		}
		return b.String()
	}

	// Directory / Repo rendering
	b.WriteString(fmt.Sprintf("── Repository Assessment: %s ──────────────────────────────\n", base))
	b.WriteString(fmt.Sprintf("%-16s %7s %15s %15s    %-20s\n", "Category", "Files", "Lines (code)", "Tokens (est)", "Health"))

	for _, c := range report.Categories {
		linesStr := formatNumber(c.Lines)
		tokensStr := formatNumber(c.Tokens)
		b.WriteString(fmt.Sprintf("%-16s %7d %15s %15s    %-20s\n", c.Name, c.FilesCount, linesStr, tokensStr, c.Health))
	}

	b.WriteString("────────────────────────────────────────────────────────────────────────\n")
	if report.RAMP != nil {
		b.WriteString(fmt.Sprintf("RAMP Level:   %s (%s) [Baseline: %s, Projected: %s]\n",
			report.RAMP.BaselineLevel, LevelDescription(report.RAMP.BaselineLevel),
			report.RAMP.BaselineLevel, report.RAMP.ProjectedLevel))
	}
	if report.FeasibilityDesc != "" {
		b.WriteString(fmt.Sprintf("Feasibility:  %s (%s)\n", report.Feasibility, report.FeasibilityDesc))
	} else {
		b.WriteString(fmt.Sprintf("Feasibility:  %s\n", report.Feasibility))
	}

	b.WriteString("Warnings:\n")
	if len(report.Warnings) == 0 {
		b.WriteString("  • none\n")
	} else {
		for _, w := range report.Warnings {
			b.WriteString(fmt.Sprintf("  • %s\n", w))
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

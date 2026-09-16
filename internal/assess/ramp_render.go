package assess

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RenderRAMPJSON serializes the RAMPProfile to indented JSON.
func RenderRAMPJSON(profile *RAMPProfile) (string, error) {
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// RenderRAMPText renders an inspectable human-readable RAMP maturity report.
func RenderRAMPText(profile *RAMPProfile) string {
	var b strings.Builder

	b.WriteString("── RAMP Repository AI Maturity Profile ───────────────────────────────\n")
	b.WriteString(fmt.Sprintf("Baseline Level:    %s (%s)\n", profile.BaselineLevel, LevelDescription(profile.BaselineLevel)))
	b.WriteString(fmt.Sprintf("Projected Level:   %s (%s)\n", profile.ProjectedLevel, LevelDescription(profile.ProjectedLevel)))
	b.WriteString(fmt.Sprintf("Rule Set Version:  %s\n", profile.RuleSetVersion))
	b.WriteString("Classification:    RAMP-informed estimate (offline, read-only)\n")

	if profile.GitNotice != "" {
		b.WriteString(fmt.Sprintf("Notice:            ℹ️  %s\n", profile.GitNotice))
	}

	b.WriteString("\nEvidence Artifact Inventory:\n")
	if len(profile.Artifacts) == 0 {
		b.WriteString("  • No AI practice artifacts detected (L1 Unconfigured)\n")
	} else {
		for _, a := range profile.Artifacts {
			statusTag := string(a.Status)
			if a.Confidence == ConfidenceUnknown {
				statusTag += ", unknown"
			} else {
				statusTag += ", " + string(a.Confidence) + " confidence"
			}
			b.WriteString(fmt.Sprintf("  • %-32s [%s, %s] (%s)\n", a.Path, a.Level, a.Category, statusTag))
			b.WriteString(fmt.Sprintf("    Rule: %s — %s\n", a.RuleID, a.Reason))
		}
	}

	b.WriteString("\nCoherence Status:\n")
	if len(profile.CoherenceAlerts) == 0 {
		b.WriteString("  ✓ Artifact levels are coherent\n")
	} else {
		for _, alert := range profile.CoherenceAlerts {
			b.WriteString(fmt.Sprintf("  ⚠️  %s\n", alert))
		}
	}

	return b.String()
}

// RenderRAMPInitText renders a concise RAMP summary specifically for harnez init preview.
func RenderRAMPInitText(profile *RAMPProfile) string {
	var b strings.Builder

	b.WriteString("RAMP Maturity Profile:\n")
	b.WriteString(fmt.Sprintf("  Baseline:  %s (%s)\n", profile.BaselineLevel, LevelDescription(profile.BaselineLevel)))
	b.WriteString(fmt.Sprintf("  Projected: %s (%s)\n", profile.ProjectedLevel, LevelDescription(profile.ProjectedLevel)))

	if profile.GitNotice != "" {
		b.WriteString(fmt.Sprintf("  Notice:    %s\n", profile.GitNotice))
	}

	var projectedArts []RAMPArtifact
	for _, a := range profile.Artifacts {
		if a.Status == StatusProjected || a.Status == StatusUncommitted {
			projectedArts = append(projectedArts, a)
		}
	}

	if len(projectedArts) > 0 {
		b.WriteString(fmt.Sprintf("  Projected Evidence (%d):\n", len(projectedArts)))
		for _, a := range projectedArts {
			b.WriteString(fmt.Sprintf("    + %s [%s, %s] (%s)\n", a.Path, a.Level, a.Category, a.Status))
		}
		b.WriteString("  Note: Working-tree files remain projected until committed in Git.\n")
	}

	if len(profile.CoherenceAlerts) > 0 {
		b.WriteString("  Coherence:\n")
		for _, alert := range profile.CoherenceAlerts {
			b.WriteString(fmt.Sprintf("    ⚠️  %s\n", alert))
		}
	}

	return b.String()
}

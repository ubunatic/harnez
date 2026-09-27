package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"ubunatic.com/harnez/internal/usage"
)

type usageTimelineJSON struct {
	Snapshots   []usage.HistoryEntry      `json:"snapshots"`
	QuotaWindow []usage.QuotaHistoryEntry `json:"quota_windows"`
}

func renderUsageTimeline(dir string, jsonOutput bool) (string, error) {
	snapshots, err := usage.ReadHistory(dir)
	if err != nil {
		return "", err
	}
	quota, err := usage.ReadQuotaHistory(dir)
	if err != nil {
		return "", err
	}
	if jsonOutput {
		out, err := json.MarshalIndent(usageTimelineJSON{Snapshots: snapshots, QuotaWindow: quota}, "", "  ")
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	var out strings.Builder
	out.WriteString(usage.RenderTimelineText(snapshots))
	if len(quota) > 0 {
		out.WriteString("\nQuota Window History\n")
		for _, entry := range quota {
			fmt.Fprintf(&out, "%s  %-10s %-18s %3d%% used\n", entry.Timestamp.Format("2006-01-02 15:04:05Z"), entry.Agent, entry.Window, entry.UsedPercent)
		}
	}
	return out.String(), nil
}

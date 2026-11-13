package usage

import (
	"fmt"
	"io/fs"
	"math"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func (s usageSpec) quotaNoticePolicy() (float64, time.Duration, error) {
	age, err := time.ParseDuration(s.AgentQuotaMaxReadingAge)
	if err != nil || age <= 0 || s.AgentQuotaWarnPercent <= 0 || s.AgentQuotaWarnPercent > 100 || math.IsNaN(s.AgentQuotaWarnPercent) {
		return 0, 0, fmt.Errorf("%s: agent quota warning requires a percent in (0,100] and positive max reading age", usageSpecPath)
	}
	return s.AgentQuotaWarnPercent, age, nil
}

// QuotaNotice formats cached quota evidence without fetching or refusing a turn.
// Each window uses its own ObservedAt, never the capture time or newest sibling.
// Both high windows and stale windows are joined into a single stderr-ready line.
func QuotaNotice(provider string, state ProviderQuotaAvailability, now time.Time) string {
	data, err := fs.ReadFile(harnez.DefaultFS, usageSpecPath)
	if err != nil {
		return ""
	}
	var spec usageSpec
	if yaml.Unmarshal(data, &spec) != nil {
		return ""
	}
	threshold, maxAge, err := spec.quotaNoticePolicy()
	if err != nil {
		return ""
	}
	var notices []string
	for _, window := range state.Windows {
		// Token counters are stored alongside quota windows but are not limits.
		if window.Pool == "tokens" || window.ObservedAt.IsZero() || window.ObservedAt.After(now) {
			continue
		}
		label := window.Key
		if label == "five_hour" {
			label = "5h"
		}
		if label == "" {
			continue
		}
		age := now.Sub(window.ObservedAt)
		read := window.ObservedAt.In(now.Location()).Format("15:04:05")
		if age > maxAge || window.Freshness != "fresh" {
			notices = append(notices, fmt.Sprintf("%s unknown, last reading %s old (read %s)", label, quotaNoticeAge(age), read))
			continue
		}
		percent := window.UsedFraction * 100
		if math.IsNaN(percent) || math.IsInf(percent, 0) || percent < threshold {
			continue
		}
		reset := ""
		if window.ResetAt != nil {
			reset = ", resets " + window.ResetAt.In(now.Location()).Format("15:04")
		}
		notices = append(notices, fmt.Sprintf("%s %.0f%% (read %s, %s ago%s)", label, percent, read, quotaNoticeAge(age), reset))
	}
	if len(notices) == 0 {
		return ""
	}
	return "quota: " + provider + " " + strings.Join(notices, "; ")
}

func quotaNoticeAge(age time.Duration) string {
	if age < time.Minute {
		return fmt.Sprintf("%ds", int64(age/time.Second))
	}
	if age < time.Hour {
		return fmt.Sprintf("%dm", int64(age/time.Minute))
	}
	return fmt.Sprintf("%dh%dm", int64(age/time.Hour), int64(age%time.Hour/time.Minute))
}

// QuotaRemainingSummary formats the remaining 5h/weekly quota percentages for
// state as known to `harnez agent models`, returning "unknown" when stale or missing.
func QuotaRemainingSummary(state ProviderQuotaAvailability, now time.Time) string {
	if state.State != "available" && state.State != "exhausted" {
		return "unknown"
	}
	var maxAge time.Duration
	if data, err := fs.ReadFile(harnez.DefaultFS, usageSpecPath); err == nil {
		var spec usageSpec
		if yaml.Unmarshal(data, &spec) == nil {
			_, maxAge, _ = spec.quotaNoticePolicy()
		}
	}
	type windowSummary struct {
		label string
		text  string
		fresh bool
	}
	var items []windowSummary
	anyFresh := false
	for _, window := range state.Windows {
		if window.Pool == "tokens" || window.Key == "" {
			continue
		}
		label := window.Key
		if label == "five_hour" {
			label = "5h"
		}
		staleWindow := window.Freshness != "fresh"
		if !window.ObservedAt.IsZero() && (window.ObservedAt.After(now) || (maxAge > 0 && now.Sub(window.ObservedAt) > maxAge)) {
			staleWindow = true
		}
		if staleWindow || math.IsNaN(window.UsedFraction) || math.IsInf(window.UsedFraction, 0) {
			items = append(items, windowSummary{label: label, text: label + " unknown"})
			continue
		}
		anyFresh = true
		rem := math.Round((1 - window.UsedFraction) * 100)
		if rem < 0 {
			rem = 0
		}
		if rem > 100 {
			rem = 100
		}
		items = append(items, windowSummary{label: label, text: fmt.Sprintf("%s %.0f%%", label, rem), fresh: true})
	}
	if !anyFresh {
		return "unknown"
	}
	rank := func(label string) int {
		switch label {
		case "5h":
			return 0
		case "weekly":
			return 1
		default:
			return 2
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		ri, rj := rank(items[i].label), rank(items[j].label)
		if ri != rj {
			return ri < rj
		}
		return items[i].label < items[j].label
	})
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = item.text
	}
	return strings.Join(parts, ", ")
}

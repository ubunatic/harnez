package usage

import (
	"fmt"
	"io/fs"
	"math"
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

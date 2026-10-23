package usage

import (
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

type usageDebugLabels struct {
	PassiveDropped string `yaml:"passive_dropped"`
	LastFetch      string `yaml:"last_fetch"`
	StaleReason    string `yaml:"stale_reason"`
	FetchError     string `yaml:"fetch_error"`
	Cause          string `yaml:"cause"`
	None           string `yaml:"none"`
	Unknown        string `yaml:"unknown"`
}

var debugLabelsOnce = sync.OnceValues(func() (usageDebugLabels, error) {
	var spec struct {
		Labels usageDebugLabels `yaml:"debug_labels"`
	}
	data, err := fs.ReadFile(harnez.DefaultFS, usageSpecPath)
	if err != nil {
		return spec.Labels, err
	}
	err = yaml.Unmarshal(data, &spec)
	return spec.Labels, err
})

// debugText keeps provider messages on one line and strips terminal controls.
func debugText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ' '
		}
		return r
	}, s)
}

func buildUsageDebugBox(summary UsageSummary, width int, now time.Time) wbox {
	labels, err := debugLabelsOnce()
	if err != nil {
		return wbox{title: mustWatchActions().spec.Actions["toggle_debug_box"].Title, width: width, lines: []string{debugText(err.Error())}}
	}
	value := func(s string) string {
		if s == "" {
			return labels.None
		}
		return debugText(s)
	}
	var lines []string
	for _, agent := range sortedUsageAgents(summary.Agents) {
		at := labels.Unknown
		if !agent.LastRefreshed.IsZero() {
			at = agent.LastRefreshed.Local().Format(time.RFC3339) + " (" + usageSnapshotAgeLabel(agent.LastRefreshed, now) + ")"
		}
		reason := agent.StaleReasonText()
		cause := agent.QuotaFetchCause
		if agent.QuotaFetchError != "" && cause == "" {
			cause = labels.Unknown
		}
		lines = append(lines,
			fmt.Sprintf("%s  %s: %s", debugText(agent.AgentID), labels.LastFetch, at),
			fmt.Sprintf("  %s: %s", labels.StaleReason, value(reason)),
			fmt.Sprintf("  %s: %s; %s: %s", labels.FetchError, value(agent.QuotaFetchError), labels.Cause, value(cause)),
		)
		count := labels.Unknown
		if n, ok := summary.PassiveDropped[agent.AgentID]; ok {
			count = fmt.Sprint(n)
		}
		lines = append(lines, fmt.Sprintf("  %s: %s", labels.PassiveDropped, count))
	}
	action := mustWatchActions().spec.Actions["toggle_debug_box"]
	return wbox{title: "[" + action.Symbol + "] " + action.Title, lines: lines, width: width}
}

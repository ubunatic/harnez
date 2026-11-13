package usage

import (
	"strings"
	"testing"
	"time"
)

const fableUsageSample = `Current session: 98% used · resets Nov 13, 11:29pm (Europe/Berlin)
Current week (all models): 97% used · resets Nov 14, 5:59pm (Europe/Berlin)
Current week (Fable): 96% used · resets Nov 14, 5:59pm (Europe/Berlin)
`

func TestParseClaudeUsageOutputFable(t *testing.T) {
	p := parseClaudeUsageOutput([]byte(fableUsageSample))
	if p.Session == nil || p.Session.UsedPercent != 98 {
		t.Fatalf("session = %+v", p.Session)
	}
	if p.Weekly == nil || p.Weekly.UsedPercent != 97 {
		t.Fatalf("weekly must stay the all-models window: %+v", p.Weekly)
	}
	if p.Fable == nil || p.Fable.UsedPercent != 96 {
		t.Fatalf("fable = %+v", p.Fable)
	}
	if q := parseClaudeUsageOutput([]byte("Current week (Opus): 50% used\n")); q.Weekly != nil || q.Fable != nil {
		t.Fatalf("other per-model limit leaked: %+v", q)
	}
}

func fableAgent(withFable bool) AgentUsage {
	now := time.Now()
	reset := now.Add(20 * time.Hour)
	a := AgentUsage{
		AgentID: "claude", Name: "Claude", Authenticated: true, LastRefreshed: now,
		Session: &QuotaWindow{Name: "Session (5-hour)", UsedPercent: 98, RemainingPercent: 2, ResetAt: &reset, DurationLeft: time.Hour},
		Weekly:  &QuotaWindow{Name: "Weekly (7-day)", UsedPercent: 97, RemainingPercent: 3, ResetAt: &reset, DurationLeft: 20 * time.Hour},
	}
	if withFable {
		applyClaudeFable(&a, &QuotaWindow{Name: claudeFableWindowName, UsedPercent: 96, RemainingPercent: 4, ResetAt: &reset, DurationLeft: 20 * time.Hour})
	}
	return a
}

func TestUsageTableFableLine(t *testing.T) {
	var labels []string
	for _, w := range usageTableWindows(fableAgent(true), time.Now()) {
		labels = append(labels, w.label)
	}
	if got := strings.Join(labels, ","); got != "weekly,5h,fable weekly" {
		t.Fatalf("labels = %s", got)
	}
	if n := len(usageTableWindows(fableAgent(false), time.Now())); n != 2 {
		t.Fatalf("no fable window must add no line, got %d windows", n)
	}
}

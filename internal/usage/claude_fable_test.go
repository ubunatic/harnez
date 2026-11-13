package usage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
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
		AgentID: "claude", Name: "Claude", Installed: true, Authenticated: true, LastRefreshed: now,
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

type fixtureTransport string

func (f fixtureTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(f)))}, nil
}

// Shape of the live api.anthropic.com/api/oauth/usage response (2026-11-13).
const fableOAuthFixture = `{
 "five_hour": {"utilization": 0.0, "resets_at": null},
 "seven_day": {"utilization": 98.0, "resets_at": "2026-11-14T17:00:00.023747+00:00"},
 "limits": [
  {"kind": "session", "group": "session", "percent": 0, "resets_at": null, "scope": null, "is_active": false},
  {"kind": "weekly_all", "group": "weekly", "percent": 98, "resets_at": "2026-11-14T17:00:00.023747+00:00", "scope": null, "is_active": true},
  {"kind": "weekly_scoped", "group": "weekly", "percent": 96, "resets_at": "2026-11-14T17:00:00.024103+00:00",
   "scope": {"model": {"id": null, "display_name": "Fable"}, "surface": null}, "is_active": false},
  {"kind": "weekly_scoped", "group": "weekly", "percent": 10, "resets_at": "2026-11-14T17:00:00.024103+00:00",
   "scope": {"model": {"id": null, "display_name": "Opus"}, "surface": null}, "is_active": false}
 ]}`

func TestFetchClaudeQuotaFableFromOAuthLimits(t *testing.T) {
	client := &http.Client{Transport: fixtureTransport(fableOAuthFixture)}
	got, _, err := fetchClaudeQuota(context.Background(), client, ClaudeCredentials{})
	if err != nil {
		t.Fatal(err)
	}
	if got.payload.Weekly == nil || got.payload.Weekly.UsedPercent != 98 {
		t.Fatalf("weekly = %+v", got.payload.Weekly)
	}
	if got.payload.Fable == nil || got.payload.Fable.UsedPercent != 96 || got.payload.Fable.ResetAt == nil {
		t.Fatalf("fable = %+v, want 96%% with a reset time", got.payload.Fable)
	}

	none := &http.Client{Transport: fixtureTransport(`{"seven_day": {"utilization": 5}, "limits": []}`)}
	got, _, err = fetchClaudeQuota(context.Background(), none, ClaudeCredentials{})
	if err != nil || got.payload.Fable != nil {
		t.Fatalf("no Fable limit must give no Fable window: %+v, %v", got.payload.Fable, err)
	}
}

// The Fable window must survive the SQLite store round trip without colliding
// with (or replacing) the all-models weekly window, whatever the row order.
func TestFableWindowStoreRoundTrip(t *testing.T) {
	isolateUsageTestStorage(t)
	now := time.Now().UTC().Truncate(time.Second)
	seedProviderSnapshot(t, "claude", now, fableAgent(true))
	_, got, err := ProviderSnapshotFromStore(context.Background(), "claude")
	if err != nil {
		t.Fatal(err)
	}
	if got.Weekly == nil || got.Weekly.UsedPercent != 97 {
		t.Fatalf("weekly = %+v, want the all-models 97%%", got.Weekly)
	}
	if got.Session == nil || got.Session.UsedPercent != 98 {
		t.Fatalf("session = %+v, want 98%%", got.Session)
	}
	fw, ok := got.ExtraWindows[ClaudeFableWindowKey]
	if !ok || fw.UsedPercent != 96 {
		t.Fatalf("fable window = %+v (ok=%v), want 96%%", fw, ok)
	}
}

// Offline guards: no network, no real home, no collector.

func TestFableWindowInUsageJSON(t *testing.T) {
	data, err := json.Marshal(fableAgent(true))
	if err != nil {
		t.Fatal(err)
	}
	var back struct {
		Weekly       *QuotaWindow           `json:"weekly"`
		ExtraWindows map[string]QuotaWindow `json:"extra_windows"`
	}
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.Weekly == nil || back.Weekly.UsedPercent != 97 {
		t.Fatalf("weekly must stay the all-models window: %s", data)
	}
	if fw, ok := back.ExtraWindows["fable"]; !ok || fw.UsedPercent != 96 || fw.Name != "Fable" {
		t.Fatalf(`extra_windows["fable"] = %+v (ok=%v) in %s`, fw, ok, data)
	}
	data, _ = json.Marshal(fableAgent(false))
	if strings.Contains(string(data), "fable") {
		t.Fatalf("no Fable limit must emit no fable key: %s", data)
	}
}

func TestCompactUsageViewHasSeparateFableLine(t *testing.T) {
	now := time.Now()
	lines := allUsageLinesAt(UsageSummary{Agents: []AgentUsage{fableAgent(true)}}, 100, false, now, time.Minute)
	var plain []string
	for _, l := range lines {
		plain = append(plain, stripANSI(l))
	}
	fable := -1
	for i, l := range plain {
		if strings.Contains(strings.ToLower(l), "fable") {
			fable = i
		}
	}
	if len(plain) != 2 || fable != 1 || !strings.Contains(plain[fable], "96%") || !strings.Contains(plain[0], "97%") {
		t.Fatalf("want agent row (97%%) then separate fable row (96%%), got %q", plain)
	}

	lines = allUsageLinesAt(UsageSummary{Agents: []AgentUsage{fableAgent(false)}}, 100, false, now, time.Minute)
	if len(lines) != 1 {
		t.Fatalf("no Fable limit must add no row, got %q", lines)
	}
}

func TestDetailedUsageViewShowsFableLimit(t *testing.T) {
	out := stripANSI(RenderText(UsageSummary{Agents: []AgentUsage{fableAgent(true)}}))
	if !strings.Contains(out, "Fable Weekly Limit:") {
		t.Fatalf("detailed view lacks the Fable limit:\n%s", out)
	}
}

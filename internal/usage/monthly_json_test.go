package usage

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAgentUsageJSONMonthlyWindow(t *testing.T) {
	u := AgentUsage{AgentID: "codex", Weekly: &QuotaWindow{Name: "Monthly", UsedPercent: 40, RemainingPercent: 60}}
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, `"weekly"`) || !strings.Contains(s, `"monthly":{"name":"Monthly"`) {
		t.Fatalf("monthly window must be reported as monthly: %s", s)
	}
	var back AgentUsage
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.Weekly == nil || back.Weekly.Name != "Monthly" || back.AgentID != "codex" {
		t.Fatalf("round trip lost the window: %+v", back)
	}
}

func TestAgentUsageJSONWeeklyUnchanged(t *testing.T) {
	data, err := json.Marshal(AgentUsage{AgentID: "claude", Weekly: &QuotaWindow{Name: "Weekly"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"weekly":{"name":"Weekly"`) || strings.Contains(string(data), `"monthly"`) {
		t.Fatalf("weekly window must stay weekly: %s", data)
	}
}

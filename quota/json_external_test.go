package quota_test

import (
	"encoding/json"
	"os"
	"testing"

	"ubunatic.com/harnez/quota"
)

// TestUsageJSONDecodeAndRoundTrip decodes a usage --json sample into the
// public quota types from outside the module's internal packages, checks the
// values, and re-encodes it to the same bytes. The sample is RenderJSON output
// (testdata/usage.json, a copy of internal/usage's golden), so a wire-format
// change breaks this test without any internal/usage dependency.
func TestUsageJSONDecodeAndRoundTrip(t *testing.T) {
	raw, err := os.ReadFile("testdata/usage.json")
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}

	var summary quota.UsageSummary
	if err := json.Unmarshal(raw, &summary); err != nil {
		t.Fatalf("decode UsageSummary: %v", err)
	}

	if len(summary.Agents) != 2 {
		t.Fatalf("agents = %d, want 2", len(summary.Agents))
	}
	claude := summary.Agents[0]
	if claude.AgentID != "claude" || claude.PlanTier != "max" {
		t.Errorf("claude agent = %q/%q, want claude/max", claude.AgentID, claude.PlanTier)
	}
	// The "monthly" key decodes into the long-window slot (Weekly).
	if claude.Weekly == nil || claude.Weekly.Name != "Monthly" || claude.Weekly.UsedPercent != 10 {
		t.Errorf("weekly window = %+v, want Monthly at 10%% used", claude.Weekly)
	}
	if claude.Tokens == nil || claude.Tokens.TotalTokens != 100 {
		t.Errorf("tokens = %+v, want total 100", claude.Tokens)
	}
	if !claude.Stale || claude.StaleReason != "fetch failed: timeout" {
		t.Errorf("stale = %v reason %q, want true and %q", claude.Stale, claude.StaleReason, "fetch failed: timeout")
	}

	if summary.Load == nil {
		t.Fatal("load snapshot missing")
	}
	if summary.Load.CPU.NumCPU != 8 || len(summary.Load.CPU.PerCorePercent) != 2 {
		t.Errorf("cpu = %+v, want 8 cores and 2 per-core samples", summary.Load.CPU)
	}
	if len(summary.Load.GPUs) != 1 || summary.Load.GPUs[0].PowerLabel != "PPT" {
		t.Fatalf("gpus = %+v, want one GPU labelled PPT", summary.Load.GPUs)
	}
	// GPUClass encodes as its integer value; 2 is the iGPU class.
	if int(summary.Load.GPUs[0].Class) != 2 {
		t.Errorf("gpu class = %d, want 2 (iGPU)", summary.Load.GPUs[0].Class)
	}

	again, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if string(again) != string(raw) {
		t.Fatalf("re-encoded JSON differs from the sample; first bytes:\n%s", again[:min(len(again), 400)])
	}
}

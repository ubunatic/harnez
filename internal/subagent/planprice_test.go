package subagent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlanPriceDefaultsAndOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	gotCodex, err := PlanPriceFor("codex")
	if err != nil || gotCodex == nil || *gotCodex != 20_000_000 {
		t.Fatalf("codex plan price = %v, err=%v; want 20000000", gotCodex, err)
	}
	gotClaude, err := PlanPriceFor("claude")
	if err != nil || gotClaude == nil || *gotClaude != 100_000_000 {
		t.Fatalf("claude plan price = %v, err=%v; want 100000000", gotClaude, err)
	}
	gotAgy, err := PlanPriceFor("agy")
	if err != nil || gotAgy == nil || *gotAgy != 20_000_000 {
		t.Fatalf("agy plan price = %v, err=%v; want 20000000", gotAgy, err)
	}
	gotPi, err := PlanPriceFor("pi")
	if err != nil || gotPi != nil {
		t.Fatalf("pi plan price = %v, err=%v; want nil (unknown)", gotPi, err)
	}

	cfgDir := filepath.Join(home, ".harnez")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfg := "agent:\n  plan_prices:\n    codex:\n      plan: Pro\n      monthly_usd_micros: 200000000\n    claude: null\n    pi:\n      monthly_usd_micros: 15000000\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	gotCodex, err = PlanPriceFor("codex")
	if err != nil || gotCodex == nil || *gotCodex != 200_000_000 {
		t.Fatalf("overridden codex = %v, err=%v; want 200000000", gotCodex, err)
	}
	gotClaude, err = PlanPriceFor("claude")
	if err != nil || gotClaude != nil {
		t.Fatalf("nulled claude = %v, err=%v; want nil", gotClaude, err)
	}
	gotPi, err = PlanPriceFor("pi")
	if err != nil || gotPi == nil || *gotPi != 15_000_000 {
		t.Fatalf("overridden pi = %v, err=%v; want 15000000", gotPi, err)
	}
}

func TestTurnRecordCacheWriteAndRecomputeSessionCostTotals(t *testing.T) {
	cwRec := NewTurnRecord(1, &TurnResult{InputTokens: 10, OutputTokens: 4, CachedTokens: 5, CacheWriteTokens: 2, CacheWriteTokensKnown: true})
	if cwRec.CacheWriteTokens == nil || *cwRec.CacheWriteTokens != 2 {
		t.Fatalf("CacheWriteTokens = %v, want 2", cwRec.CacheWriteTokens)
	}
	unknownCW := NewTurnRecord(1, &TurnResult{InputTokens: 10, OutputTokens: 4, CachedTokens: 3})
	if unknownCW.CacheWriteTokens != nil {
		t.Fatalf("CacheWriteTokens = %v, want nil when not reported", unknownCW.CacheWriteTokens)
	}

	c1, c2 := int64(100), int64(250)
	s1 := int64(400)
	sess := &Session{TurnRecords: []TurnRecord{
		{Turn: 1, CostMicros: &c1, SubscriptionPriceShareMicros: &s1},
		{Turn: 2, CostMicros: &c2},
	}}
	RecomputeSessionCostTotals(sess)
	if sess.TotalCostMicros == nil || *sess.TotalCostMicros != 350 {
		t.Fatalf("TotalCostMicros = %v, want 350", sess.TotalCostMicros)
	}
	if sess.TotalSubscriptionShareMicros != nil {
		t.Fatalf("TotalSubscriptionShareMicros = %v, want nil when turn 2 share unknown", sess.TotalSubscriptionShareMicros)
	}
}

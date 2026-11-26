package subagent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlanPriceClaudeProIsEURConvertedOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got, err := PlanPriceFor("claude")
	// EUR 18 at the spec's ECB rate 1.0935 USD per EUR is 19.683 USD.
	if err != nil || got == nil || *got != 19_683_000 {
		t.Fatalf("claude Pro price = %v, err=%v; want 19683000 USD micros", got, err)
	}
}

func TestPlanPriceOldUSDConfigStaysValid(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".harnez"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "agent:\n  plan_prices:\n    claude:\n      plan: Max 5x\n      monthly_usd_micros: 100000000\n"
	if err := os.WriteFile(filepath.Join(home, ".harnez", "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := PlanPriceFor("claude")
	if err != nil || got == nil || *got != 100_000_000 {
		t.Fatalf("legacy monthly_usd_micros override = %v, err=%v; want 100000000", got, err)
	}
}

func TestPlanMicrosFromEURNeedsFXRate(t *testing.T) {
	eur := int64(18_000_000)
	if got := usdMicrosFromEUR(&eur, 0); got != nil {
		t.Fatalf("missing FX rate gave %v; want nil (unknown, never 0)", *got)
	}
	if got := usdMicrosFromEUR(nil, 1.0935); got != nil {
		t.Fatalf("missing EUR amount gave %v; want nil", *got)
	}
	if got := usdMicrosFromEUR(&eur, 1.0935); got == nil || *got != 19_683_000 {
		t.Fatalf("usdMicrosFromEUR = %v; want 19683000", got)
	}
}

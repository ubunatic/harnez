package usage

import (
	"testing"
	"time"

	"ubunatic.com/harnez/internal/modelblock"
)

func isolateBlockState(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
}

func TestActiveModelBlockMakesModelExhausted(t *testing.T) {
	isolateBlockState(t)
	now := time.Now()
	err := modelblock.Add(modelblock.Path(), modelblock.Block{Provider: "agy", Model: "claude-sonnet-5-5", Until: now.Add(2 * time.Hour), Reason: "Individual quota reached"}, now)
	if err != nil {
		t.Fatal(err)
	}
	got := CachedProviderQuotaAvailabilities("agy", []string{"claude-sonnet-5-5", "other"}, now)
	blocked := got["claude-sonnet-5-5"]
	if blocked.State != "exhausted" || !blocked.Exhausted || blocked.BlockReason != "Individual quota reached" {
		t.Fatalf("blocked model = %+v", blocked)
	}
	if blocked.ResetIn < 119*time.Minute || blocked.ResetIn > 2*time.Hour {
		t.Fatalf("ResetIn = %v, want about 2h", blocked.ResetIn)
	}
	if got["other"].State == "exhausted" {
		t.Fatal("block leaked to another model")
	}
	if CachedProviderQuotaAvailabilities("codex", []string{"claude-sonnet-5-5"}, now)["claude-sonnet-5-5"].State == "exhausted" {
		t.Fatal("block leaked to another provider")
	}
}

func TestExpiredModelBlockIsIgnored(t *testing.T) {
	isolateBlockState(t)
	now := time.Now()
	if err := modelblock.Add(modelblock.Path(), modelblock.Block{Provider: "agy", Model: "m", Until: now.Add(time.Minute)}, now); err != nil {
		t.Fatal(err)
	}
	got := CachedProviderQuotaAvailabilities("agy", []string{"m"}, now.Add(2*time.Minute))["m"]
	if got.State == "exhausted" || !got.BlockedUntil.IsZero() {
		t.Fatalf("expired block still applied: %+v", got)
	}
}

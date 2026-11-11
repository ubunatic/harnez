package subagent

import "testing"

func TestCheckCost(t *testing.T) {
	spec := CostCheckSpec{CachedToNewRatio: 20, ContextUsageFraction: 0.75, MinTurns: 3}
	base := CostCheckInput{Role: "developer", Model: "codex:gpt-6.1-terra", CompactLimit: 200_000, FreshStart: 25_000}

	tests := []struct {
		name string
		edit func(*CostCheckInput)
		want string // "" means no check line
	}{
		{"below min turns", func(in *CostCheckInput) {
			in.Turn, in.CachedTokens, in.NewTokens, in.ContextUsage = 2, 7_100_000, 175_000, 190_000
		}, ""},
		{"ticket example over ratio", func(in *CostCheckInput) {
			in.Turn, in.CachedTokens, in.NewTokens, in.ContextUsage = 17, 7_100_000, 175_000, 110_000
		}, "[check: turn re-read 7.1M cached for 175.0k new (40x); context usage 110.0k; fresh start ~25.0k; consider: harnez agent start --role developer --model codex:gpt-6.1-terra -f <ticket>]"},
		{"under both thresholds", func(in *CostCheckInput) {
			in.Turn, in.CachedTokens, in.NewTokens, in.ContextUsage = 4, 19_999, 1_000, 149_999
		}, ""},
		{"ratio exactly at threshold", func(in *CostCheckInput) {
			in.Turn, in.CachedTokens, in.NewTokens, in.ContextUsage = 3, 20_000, 1_000, 0
		}, "[check: turn re-read 20.0k cached for 1000 new (20x); context usage unknown; fresh start ~25.0k; consider: harnez agent start --role developer --model codex:gpt-6.1-terra -f <ticket>]"},
		{"min turns zero keeps the checks on", func(in *CostCheckInput) {
			in.Turn, in.CachedTokens, in.NewTokens = 1, 20_000, 1_000
		}, "minturns0"},
		{"context at 0.75 of compaction limit", func(in *CostCheckInput) {
			in.Turn, in.CachedTokens, in.NewTokens, in.ContextUsage = 3, 10_000, 1_000, 150_000
		}, "[check: turn re-read 10.0k cached for 1000 new (10x); context usage 150.0k; fresh start ~25.0k; consider: harnez agent start --role developer --model codex:gpt-6.1-terra -f <ticket>]"},
		{"unknown new tokens never trigger the ratio", func(in *CostCheckInput) {
			in.Turn, in.CachedTokens, in.NewTokens, in.ContextUsage = 5, 5_000_000, 0, 0
		}, ""},
		{"agy-like unknown usage with context over the limit", func(in *CostCheckInput) {
			in.Turn, in.ContextUsage, in.FreshStart = 5, 160_000, 0
		}, "[check: turn re-read unknown; context usage 160.0k; fresh start ~unknown; consider: harnez agent start --role developer --model codex:gpt-6.1-terra -f <ticket>]"},
		{"zero thresholds disable the check", func(in *CostCheckInput) {
			in.Turn, in.CachedTokens, in.NewTokens, in.ContextUsage = 9, 9_000_000, 1_000, 199_000
		}, "disabled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in, s := base, spec
			tt.edit(&in)
			want := tt.want
			switch want {
			case "disabled":
				s, want = CostCheckSpec{}, ""
			case "minturns0":
				s.MinTurns = 0
				want = "[check: turn re-read 20.0k cached for 1000 new (20x); context usage unknown; fresh start ~25.0k; consider: harnez agent start --role developer --model codex:gpt-6.1-terra -f <ticket>]"
			}
			got, fired := CheckCost(s, in)
			if fired != (want != "") || got != want {
				t.Errorf("CheckCost() = %q, %v; want %q", got, fired, want)
			}
		})
	}
}

func TestCostCheckSpecValuesLoad(t *testing.T) {
	spec, err := loadAgentSpec()
	if err != nil {
		t.Fatal(err)
	}
	if spec.CostCheck.CachedToNewRatio <= 0 || spec.CostCheck.ContextUsageFraction <= 0 || spec.CostCheck.MinTurns <= 0 {
		t.Fatalf("spec/agent.yaml cost_check not loaded: %+v", spec.CostCheck)
	}
}

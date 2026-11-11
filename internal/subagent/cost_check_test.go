package subagent

import "testing"

func TestCheckCost(t *testing.T) {
	spec := CostCheckSpec{
		CachedToNewRatio:     20.0,
		ContextUsageFraction: 0.75,
		MinTurns:             3,
	}
	model := Model{Name: "test-model"}
	compactLimit := 100000

	tests := []struct {
		name         string
		turn         int
		cachedTokens int
		newTokens    int
		contextUsage int
		wantFound    bool
		wantString   string
	}{
		{
			name:         "below min turns",
			turn:         2,
			cachedTokens: 100000,
			newTokens:    1000,
			contextUsage: 80000,
			wantFound:    false,
		},
		{
			name:         "over ratio",
			turn:         3,
			cachedTokens: 21000,
			newTokens:    1000,
			contextUsage: 10000,
			wantFound:    true,
			wantString:   "[check: turn re-read 21.0k cached for 1000 new (21x); context usage 10.0k; fresh start ~1000; consider: harnez agent start --role <role> --model test-model -f <ticket>]",
		},
		{
			name:         "under ratio",
			turn:         4,
			cachedTokens: 19000,
			newTokens:    1000,
			contextUsage: 50000,
			wantFound:    false,
		},
		{
			name:         "over context fraction",
			turn:         3,
			cachedTokens: 10000,
			newTokens:    1000,
			contextUsage: 75000, // 0.75 * 100000
			wantFound:    true,
			wantString:   "[check: turn re-read 10.0k cached for 1000 new (10x); context usage 75.0k; fresh start ~1000; consider: harnez agent start --role <role> --model test-model -f <ticket>]",
		},
		{
			name:         "zero new tokens",
			turn:         3,
			cachedTokens: 20,
			newTokens:    0,
			contextUsage: 100,
			wantFound:    true,
			wantString:   "[check: turn re-read 20 cached for 1 new (20x); context usage 100; fresh start ~1; consider: harnez agent start --role <role> --model test-model -f <ticket>]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotString, gotFound := CheckCost(spec, model, tt.turn, tt.cachedTokens, tt.newTokens, tt.contextUsage, compactLimit)
			if gotFound != tt.wantFound {
				t.Errorf("CheckCost() gotFound = %v, want %v", gotFound, tt.wantFound)
			}
			if gotFound && gotString != tt.wantString {
				t.Errorf("CheckCost() gotString = %q, want %q", gotString, tt.wantString)
			}
		})
	}
}

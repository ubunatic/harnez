package subagent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompactThresholdDefaultGlobalAndOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	m := Model{Provider: "codex", Name: "gpt", Tier: "high"}
	got, err := CompactThreshold(m)
	if err != nil || got != 200000 {
		t.Fatalf("default threshold = %d, %v; want spec/agent.yaml compact_threshold_tokens", got, err)
	}
	config := "agent:\n  compact_threshold_tokens: 250000\n  compact_thresholds:\n    codex:gpt:high: 175000\n"
	if err := os.MkdirAll(filepath.Join(home, ".harnez"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".harnez", "config.yaml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = CompactThreshold(m)
	if err != nil || got != 175000 {
		t.Fatalf("override threshold = %d, %v", got, err)
	}
	got, err = CompactThreshold(Model{Provider: "claude", Name: "sonnet"})
	if err != nil || got != 250000 {
		t.Fatalf("global threshold = %d, %v", got, err)
	}
}

func TestCompactThresholdHaikuSpecAndUserPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, tc := range []struct {
		name string
		want int
	}{
		{"haiku", 80000}, {"claude:haiku:low", 80000}, {"claude:haiku:med", 80000},
		{"claude:haiku:high", 80000}, {"claude:sonnet:low", 200000},
		{"codex:luna:med", 100000}, {"codex:sol:low", 100000}, {"codex:terra:high", 100000},
	} {
		model, err := ResolveModel(tc.name)
		if err != nil {
			t.Fatal(err)
		}
		got, err := CompactThreshold(model)
		if err != nil || got != tc.want {
			t.Errorf("%s: threshold = %d, %v; want %d", tc.name, got, err, tc.want)
		}
	}
	if err := os.MkdirAll(filepath.Join(home, ".harnez"), 0700); err != nil {
		t.Fatal(err)
	}
	model, err := ResolveModel("haiku")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		config string
		want   int
	}{
		{"agent:\n  compact_threshold_tokens: 150000\n", 80000},
		{"agent:\n  compact_threshold_tokens: 150000\n  compact_thresholds:\n    claude:haiku:low: 120000\n", 120000},
		{"agent:\n  compact_thresholds:\n    claude:haiku:low: 60000\n", 60000},
	} {
		if err := os.WriteFile(filepath.Join(home, ".harnez", "config.yaml"), []byte(tc.config), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := CompactThreshold(model)
		if err != nil || got != tc.want {
			t.Errorf("config %q: threshold = %d, %v; want %d", tc.config, got, err, tc.want)
		}
	}
}

func TestAutoCompactModesFromSpec(t *testing.T) {
	for provider, want := range map[string]string{"agy": "disabled", "claude": "manual", "codex": "native", "other": "manual"} {
		got, err := AutoCompactMode(provider)
		if err != nil || got != want {
			t.Errorf("%s mode = %q, %v; want %q", provider, got, err, want)
		}
	}
}

func TestVerifyCompactionRequiresAckAndTokenDrop(t *testing.T) {
	cases := []struct {
		name    string
		result  *TurnResult
		wantErr string
	}{
		{name: "valid with cached input", result: &TurnResult{Response: "Compaction complete", InputTokens: 500, CachedTokens: 490, ContextTokens: 500}},
		{name: "no acknowledgement", result: &TurnResult{InputTokens: 500}, wantErr: "acknowledgement"},
		{name: "no token drop despite cache", result: &TurnResult{Response: "Compaction complete", InputTokens: 10, CachedTokens: 900, ContextTokens: 1200}, wantErr: "context drop"},
		{name: "no token measurement", result: &TurnResult{Response: "Compaction complete"}, wantErr: "context drop"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyCompaction(1200, tc.result)
			if tc.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestEnsureContextUnderThreshold(t *testing.T) {
	called := false
	compact := func() (*TurnResult, error) {
		called = true
		return &TurnResult{Response: "Compaction complete", ContextTokens: 100}, nil
	}
	compacted, err := EnsureContextUnderThreshold(-1, 200, compact)
	if err != nil || compacted || called {
		t.Fatalf("unknown context: compacted=%v called=%v err=%v", compacted, called, err)
	}
	compacted, err = EnsureContextUnderThreshold(199, 200, compact)
	if err != nil || compacted || called {
		t.Fatalf("under threshold: compacted=%v called=%v err=%v", compacted, called, err)
	}
	compacted, err = EnsureContextUnderThreshold(200, 200, compact)
	if err != nil || !compacted || !called {
		t.Fatalf("at threshold: compacted=%v called=%v err=%v", compacted, called, err)
	}
}

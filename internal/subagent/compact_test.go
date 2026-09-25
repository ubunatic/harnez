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
	if err != nil || got != DefaultCompactThresholdTokens {
		t.Fatalf("default threshold = %d, %v", got, err)
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

func TestVerifyCompactionRequiresAckAndTokenDrop(t *testing.T) {
	cases := []struct {
		name    string
		result  *TurnResult
		wantErr string
	}{
		{name: "valid", result: &TurnResult{Response: "Compaction complete", InputTokens: 500}},
		{name: "no acknowledgement", result: &TurnResult{InputTokens: 500}, wantErr: "acknowledgement"},
		{name: "no token drop", result: &TurnResult{Response: "Compaction complete", InputTokens: 1000}, wantErr: "context drop"},
		{name: "no token measurement", result: &TurnResult{Response: "Compaction complete"}, wantErr: "context drop"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyCompaction(1000, tc.result)
			if tc.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

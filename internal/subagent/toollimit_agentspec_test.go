package subagent

import (
	"strings"
	"testing"
)

// Issue 1033 M1: the embedded spec carries the tool-call limits, and the
// lookup follows the compact_thresholds key order with role, then "default".
func TestToolCallLimitFromEmbeddedSpec(t *testing.T) {
	lm := Model{Provider: "lmcoder", Name: "coder", Tier: "low"}
	for role, want := range map[string]int{"reviewer": 15, "advisor": 15, "developer": 40} {
		got, err := ToolCallLimit(lm, role)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("ToolCallLimit(lmcoder:coder, %s) = %d, want %d", role, got, want)
		}
	}
	got, err := ToolCallLimit(Model{Provider: "claude", Name: "sonnet", Tier: "high"}, "developer")
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Errorf("paid model limit = %d, want 0 (off)", got)
	}
}

func TestToolCallLimitsLookupOrder(t *testing.T) {
	s := ToolCallLimitsSpec{
		Default: 5,
		Models: map[string]map[string]int{
			"p:full:low": {"reviewer": 1},
			"p:full":     {"reviewer": 2, "default": 3},
			"p":          {"default": 4},
		},
		NativeBackstopCalls: 1,
		SynthesisPrompt:     "x",
	}
	full := Model{Provider: "p", Name: "full", Tier: "low"}
	if n := s.Limit(full, "reviewer"); n != 1 {
		t.Errorf("spec key: %d, want 1", n)
	}
	// The first matching model entry wins even without the role: its absent
	// "default" falls back to the section default, not to a broader key.
	if n := s.Limit(full, "developer"); n != 5 {
		t.Errorf("spec key without role: %d, want 5", n)
	}
	if n := s.Limit(Model{Provider: "p", Name: "full", Tier: "high"}, "developer"); n != 3 {
		t.Errorf("provider:name default: %d, want 3", n)
	}
	if n := s.Limit(Model{Provider: "p", Name: "other", Tier: "low"}, "reviewer"); n != 4 {
		t.Errorf("provider default: %d, want 4", n)
	}
	if n := s.Limit(Model{Provider: "q", Name: "x", Tier: "low"}, "reviewer"); n != 5 {
		t.Errorf("section default: %d, want 5", n)
	}
}

func TestToolCallLimitsValidate(t *testing.T) {
	if err := (ToolCallLimitsSpec{}).validate(); err != nil {
		t.Fatalf("absent section must be valid: %v", err)
	}
	ok := ToolCallLimitsSpec{NativeBackstopCalls: 1, SynthesisPrompt: "x"}
	for name, tc := range map[string]struct {
		mut  func(*ToolCallLimitsSpec)
		want string
	}{
		"negative default": {func(s *ToolCallLimitsSpec) { s.Default = -1 }, "default"},
		"no backstop":      {func(s *ToolCallLimitsSpec) { s.NativeBackstopCalls = 0 }, "native_backstop_calls"},
		"blank prompt":     {func(s *ToolCallLimitsSpec) { s.SynthesisPrompt = " " }, "synthesis_prompt"},
		"negative role":    {func(s *ToolCallLimitsSpec) { s.Models = map[string]map[string]int{"m": {"r": -1}} }, "models.m.r"},
		"empty model":      {func(s *ToolCallLimitsSpec) { s.Models = map[string]map[string]int{"m": {}} }, "models.m"},
	} {
		s := ok
		tc.mut(&s)
		err := s.validate()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want mention of %q", name, err, tc.want)
		}
	}
}

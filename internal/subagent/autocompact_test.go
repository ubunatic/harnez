package subagent

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseAutocompact(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{in: "auto", want: 0},
		{in: "1", want: 1},
		{in: "120000", want: 120000},
		{in: "1k", want: 1000},
		{in: "120k", want: 120000},
	} {
		got, err := ParseAutocompact(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseAutocompact(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}
	for _, in := range []string{"", "k", "0", "0k", "-5", "1.5k", "120K", "abc", "12k3", "99999999999999999999k"} {
		if got, err := ParseAutocompact(in); err == nil {
			t.Errorf("ParseAutocompact(%q) = %d, want an error", in, got)
		} else if !strings.Contains(err.Error(), "--autocompact") {
			t.Errorf("ParseAutocompact(%q) error %q does not name the flag", in, err)
		}
	}
}

func TestCompactThresholdPerRunOverrideWins(t *testing.T) {
	got, err := CompactThreshold(Model{Provider: "claude", Name: "haiku", Tier: "low", CompactTokens: 120000})
	if err != nil || got != 120000 {
		t.Fatalf("CompactThreshold with CompactTokens 120000 = %d, %v; want 120000", got, err)
	}
}

func TestClaudeChildEnvAutocompactReplacesCallerWindow(t *testing.T) {
	model := Model{Provider: "claude", Name: "haiku", Tier: "low", CompactTokens: 120000}
	environ := []string{"PATH=/bin", "CLAUDE_CODE_AUTO_COMPACT_WINDOW=50000", "HOME=/tmp/h", "CLAUDE_CODE_AUTO_COMPACT_WINDOW=60000"}
	var warn bytes.Buffer
	out := claudeChildEnv(model, environ, &warn)
	var windows []string
	for _, kv := range out {
		if strings.HasPrefix(kv, claudeCompactWindowEnv+"=") {
			windows = append(windows, kv)
		}
	}
	if len(windows) != 1 || windows[0] != claudeCompactWindowEnv+"=120000" {
		t.Fatalf("window entries = %q, want exactly [%s=120000]", windows, claudeCompactWindowEnv)
	}
	if !strings.Contains(out[len(out)-1], "CLAUDE_CODE_AUTO_COMPACT_WINDOW=120000") {
		t.Fatalf("harnez value must be last so it is the one Claude sees, got %q", out)
	}
	if !strings.Contains(warn.String(), "replaced by --autocompact 120000") {
		t.Fatalf("warning = %q, want a replaced-by note", warn.String())
	}
}

func TestClaudeChildEnvAutocompactSameCallerValueIsSilent(t *testing.T) {
	model := Model{Provider: "claude", Name: "haiku", Tier: "low", CompactTokens: 120000}
	var warn bytes.Buffer
	out := claudeChildEnv(model, []string{"CLAUDE_CODE_AUTO_COMPACT_WINDOW=120000"}, &warn)
	if len(out) != 1 || out[0] != "CLAUDE_CODE_AUTO_COMPACT_WINDOW=120000" {
		t.Fatalf("env = %q, want one window entry at 120000", out)
	}
	if warn.Len() != 0 {
		t.Fatalf("warning = %q, want none for an equal caller value", warn.String())
	}
}

func TestSessionCompactTokensJSON(t *testing.T) {
	data, err := json.Marshal(&Session{ID: "s", CompactTokens: 120000})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"compact_tokens":120000`) {
		t.Fatalf("session JSON %s lacks compact_tokens", data)
	}
	var back Session
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.CompactTokens != 120000 {
		t.Fatalf("round trip CompactTokens = %d, want 120000", back.CompactTokens)
	}
	data, err = json.Marshal(&Session{ID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "compact_tokens") {
		t.Fatalf("zero CompactTokens must be omitted, got %s", data)
	}
}

package hookdispatch

import (
	"slices"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez"
)

// Issue 1303 M1e: the collector's fill schedule for Claude's compact boundary.
func TestLoadClaudeCompactFillRetryReadsEmbeddedSpec(t *testing.T) {
	got, err := LoadClaudeCompactFillRetry()
	if err != nil {
		t.Fatalf("LoadClaudeCompactFillRetry: %v", err)
	}
	want := []time.Duration{2 * time.Second, 10 * time.Second, 60 * time.Second}
	if !slices.Equal(got, want) {
		t.Errorf("LoadClaudeCompactFillRetry = %v, want %v", got, want)
	}
}

func TestParseClaudeCompactFillRetryRejectsBadValues(t *testing.T) {
	raw, err := harnez.DefaultFS.ReadFile("spec/hooks.yaml")
	if err != nil {
		t.Fatal(err)
	}
	base := string(raw)
	cases := map[string]string{
		"empty schedule":  strings.Replace(base, "fill_retry: [2s, 10s, 60s]", "fill_retry: []", 1),
		"zero delay":      strings.Replace(base, "fill_retry: [2s, 10s, 60s]", "fill_retry: [0s, 10s, 60s]", 1),
		"malformed delay": strings.Replace(base, "fill_retry: [2s, 10s, 60s]", "fill_retry: [2s, ten, 60s]", 1),
		"old wait key":    strings.Replace(base, "fill_retry: [2s, 10s, 60s]", "boundary_wait: 3s", 1),
		"missing section": strings.Replace(base, "claude_compact:", "claude_compact_removed:", 1),
	}
	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			if spec == base {
				t.Fatal("test case did not change the spec")
			}
			if _, err := parseClaudeCompactFillRetry([]byte(spec)); err == nil {
				t.Error("parseClaudeCompactFillRetry accepted the bad spec, want an error")
			}
		})
	}
}

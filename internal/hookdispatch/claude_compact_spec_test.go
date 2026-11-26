package hookdispatch

import (
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez"
)

func TestLoadClaudeCompactWaitReadsEmbeddedSpec(t *testing.T) {
	wait, err := LoadClaudeCompactWait()
	if err != nil {
		t.Fatalf("LoadClaudeCompactWait: %v", err)
	}
	want := ClaudeCompactWait{Window: 10 * time.Minute, Wait: 3 * time.Second, Poll: 100 * time.Millisecond}
	if wait != want {
		t.Errorf("LoadClaudeCompactWait = %+v, want %+v", wait, want)
	}
	budget, err := LoadBudget()
	if err != nil {
		t.Fatalf("LoadBudget: %v", err)
	}
	if wait.Wait >= budget {
		t.Errorf("boundary_wait %s must stay under handler_timeout %s", wait.Wait, budget)
	}
}

func TestParseClaudeCompactWaitRejectsBadValues(t *testing.T) {
	raw, err := harnez.DefaultFS.ReadFile("spec/hooks.yaml")
	if err != nil {
		t.Fatal(err)
	}
	base := string(raw)
	cases := map[string]string{
		"poll longer than wait": strings.Replace(base, "boundary_wait: 3s", "boundary_wait: 50ms", 1),
		"zero wait":             strings.Replace(base, "boundary_wait: 3s", "boundary_wait: 0s", 1),
		"malformed window":      strings.Replace(base, "boundary_window: 10m", "boundary_window: ten", 1),
		"missing section":       strings.Replace(base, "claude_compact:", "claude_compact_removed:", 1),
	}
	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			if spec == base {
				t.Fatal("test case did not change the spec")
			}
			if _, err := parseClaudeCompactWait([]byte(spec)); err == nil {
				t.Error("parseClaudeCompactWait accepted the bad spec, want an error")
			}
		})
	}
}

package claude

import (
	"strings"
	"testing"
)

// The configured rules header lists Quota.md. A repo without rules.quota has no
// such file, so the header written for it must not name it.
func TestRulesHeaderForDropsQuotaWhenOff(t *testing.T) {
	header := LoadTestConfig(t).AgentsMD.Rules.Header
	if !strings.Contains(header, quotaRuleReadPath) {
		t.Fatalf("configured header lacks %q; rulesHeaderFor would not drop it: %q", quotaRuleReadPath, header)
	}
	if got := rulesHeaderFor(header, true); got != header {
		t.Fatalf("quota on must keep the header unchanged; got %q", got)
	}
	off := rulesHeaderFor(header, false)
	if strings.Contains(off, "Quota.md") {
		t.Fatalf("quota off still lists Quota.md: %q", off)
	}
	if !strings.Contains(off, ".harnez/rules/Issues.md .harnez/rules/Subagents.md") {
		t.Fatalf("quota off must keep the other rule reads in order: %q", off)
	}
}

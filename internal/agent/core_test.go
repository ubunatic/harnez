package agent

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
)

func TestStartRoleUsesInjectedCallerRole(t *testing.T) {
	var notice bytes.Buffer
	role, _, err := StartRole("advisor", "task", "orchestrator", &notice)
	if err != nil || role != "advisor" {
		t.Fatalf("StartRole = %q, %v", role, err)
	}
	if notice.Len() != 0 {
		t.Fatalf("unexpected classifier notice: %q", notice.String())
	}
	if _, _, err := StartRole("orchestrator", "task", "developer", &notice); err == nil {
		t.Fatal("developer caller unexpectedly started orchestrator")
	}
}

func TestFindResumeSelectorPreservesExactAndAmbiguousRules(t *testing.T) {
	sessions := []*subagent.Session{
		{ID: "12345678-a", Name: "alpha-worker", ProviderSessionID: "provider-a"},
		{ID: "12345678-b", Name: "12345678-a", ProviderSessionID: "provider-b"},
	}
	got, ok, err := FindResumeSelector(sessions, "alpha-worker")
	if err != nil || !ok || got != sessions[0] {
		t.Fatalf("exact selector = %p, %v, %v", got, ok, err)
	}
	if got, ok, err := FindResumeSelector(sessions, "12345678"); err != nil || ok || got != nil {
		t.Fatalf("ambiguous prefix = %p, %v, %v", got, ok, err)
	}
	if _, _, err := FindResumeSelector(sessions, "12345678-a"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("duplicate exact selector error = %v", err)
	}
}

func TestQuotaDecisionIsInjectedAndOverrideIsExplicit(t *testing.T) {
	model := subagent.Model{Provider: "agy", Name: "gemini-3.8-flash"}
	availability := func(provider, _ string) usage.ProviderQuotaAvailability {
		if provider == "agy" {
			return usage.ProviderQuotaAvailability{State: "exhausted", Age: 2 * time.Hour}
		}
		return usage.ProviderQuotaAvailability{}
	}
	err := RejectExhaustedQuota("agy:flash38", model, false, availability)
	if err == nil || !strings.Contains(err.Error(), "quota is exhausted") {
		t.Fatalf("quota rejection = %v", err)
	}
	if err := RejectExhaustedQuota("agy:flash38", model, true, availability); err != nil {
		t.Fatalf("explicit override rejected: %v", err)
	}
}

func TestCheckStreamMode(t *testing.T) {
	for _, mode := range []string{StreamFull, StreamStats} {
		if err := CheckStreamMode(mode); err != nil {
			t.Errorf("mode %q: %v", mode, err)
		}
	}
	if err := CheckStreamMode("quiet"); err == nil {
		t.Fatal("invalid stream mode accepted")
	}
}

// 959: an inferred role is marked as a guess so the agent reports a conflict
// instead of calling it explicit; an explicit role carries no such note.
func TestWithProtocolMarksInferredRole(t *testing.T) {
	inferred := WithProtocol("task", false, "reviewer", true, "")
	if !strings.Contains(inferred, "inferred from the prompt") || !strings.Contains(inferred, "--role developer") {
		t.Fatalf("inferred role not marked:\n%s", inferred)
	}
	if explicit := WithProtocol("task", false, "reviewer", false, ""); strings.Contains(explicit, "inferred from the prompt") {
		t.Fatalf("explicit role marked as inferred:\n%s", explicit)
	}
}

func TestWithProtocolFamilyPreamble(t *testing.T) {
	baseline := WithProtocol("task", false, "developer", false, "")
	for _, provider := range []string{"claude", "codex", "agy"} {
		want, err := subagent.FamilyPreamble(provider)
		if err != nil || want == "" {
			t.Fatalf("FamilyPreamble(%q) = %q, %v", provider, want, err)
		}
		got := WithProtocol("task", false, "developer", false, provider)
		if !strings.Contains(got, want) {
			t.Errorf("WithProtocol(%q) missing family preamble %q:\n%s", provider, want, got)
		}
	}
	for _, without := range []string{"", "lmcoder", "unknown"} {
		if pre, err := subagent.FamilyPreamble(without); err != nil || pre != "" {
			t.Fatalf("FamilyPreamble(%q) = %q, %v; want empty", without, pre, err)
		}
		if got := WithProtocol("task", false, "developer", false, without); got != baseline {
			t.Errorf("WithProtocol(%q) altered baseline text:\n%s", without, got)
		}
	}
}

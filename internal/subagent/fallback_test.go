package subagent

import (
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/modelcatalog"
)

func TestTraitFallback(t *testing.T) {
	models := map[string][]string{"smart": {"sol:med", "opus:med"}, "gui": {"lean", "smart"}, "lean": {"terra:low", "luna:med"}, "ai": {}, "cycle": {"cycle"}}
	resolve := func(s string) (Model, error) { return Model{Name: s}, nil }
	for _, tc := range []struct {
		name, trait           string
		reasons               map[string]string
		want, skip, errorPart string
	}{
		{"first", "smart", nil, "sol:med", "", ""},
		{"disabled", "smart", map[string]string{"sol:med": "disabled"}, "opus:med", "sol:med: disabled", ""},
		{"quota", "smart", map[string]string{"sol:med": "quota exhausted"}, "opus:med", "sol:med: quota exhausted", ""},
		{"denied", "smart", map[string]string{"sol:med": "access denied"}, "opus:med", "sol:med: access denied", ""},
		{"all", "smart", map[string]string{"sol:med": "quota exhausted", "opus:med": "quota exhausted"}, "", "", "sol:med: quota exhausted; opus:med: quota exhausted"},
		{"nested", "gui", map[string]string{"terra:low": "disabled", "luna:med": "quota exhausted", "sol:med": "access denied"}, "opus:med", "terra:low: disabled; luna:med: quota exhausted; sol:med: access denied", ""},
		{"unset", "ai", nil, "", "", "unset"},
		{"cycle", "cycle", nil, "", "", "cycle"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveTraitCandidates(models, tc.trait, resolve, func(m Model) string { return tc.reasons[m.Name] })
			if tc.errorPart != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errorPart) || tc.name == "all" && !strings.Contains(err.Error(), "--model provider:model:tier") {
					t.Fatalf("error %v", err)
				}
				return
			}
			if err != nil || got.Model.Name != tc.want || strings.Join(got.Skipped, "; ") != tc.skip {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestNestedUnsetFallsThrough(t *testing.T) {
	traits := map[string][]string{"gui": {"ai", "lean"}, "ai": {}, "lean": {"terra:low"}}
	got, err := resolveTraitCandidates(traits, "gui", func(s string) (Model, error) { return Model{Name: s}, nil }, func(Model) string { return "" })
	if err != nil || got.Target != "terra:low" || !strings.Contains(strings.Join(got.Skipped, "; "), "ai: unset") {
		t.Fatalf("selection=%+v err=%v", got, err)
	}
}

func TestUnsetDefaultPolicyMessage(t *testing.T) {
	base, err := agentSpecOnce()
	if err != nil {
		t.Fatal(err)
	}
	aliases, err := modelAliasMap()
	if err != nil {
		t.Fatal(err)
	}
	policy := &modelcatalog.Policy{Roles: map[string]modelcatalog.RolePolicy{"developer": {Models: map[string][]string{"lean": {}}}}}
	merged, err := mergeRolePolicy(base, policy, aliases, "/tmp/models.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = resolveRoleSelection(merged, "developer", "", func(s string) (Model, error) { return resolveModelIn(aliases, s) }, func(Model) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "default model") || !strings.Contains(err.Error(), "disabled by user policy") {
		t.Fatalf("error=%v", err)
	}
}

func TestFallbackAvailability(t *testing.T) {
	for _, tc := range []struct {
		name          string
		enabled       bool
		access, quota string
		override      bool
		want          string
	}{
		{"available", true, "unknown", "unknown", false, ""},
		{"disabled", false, "unknown", "unknown", true, "disabled by user policy"},
		{"denied", true, "denied", "exhausted", true, "access denied"},
		{"quota", true, "unknown", "exhausted", false, "quota exhausted"},
		{"override", true, "unknown", "exhausted", true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := candidateUnavailable(tc.enabled, tc.access, tc.quota, tc.override); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestFallbackDefaultDisabled(t *testing.T) {
	spec := agentSpec{Roles: map[string]RoleSpec{"developer": {DefaultModel: "cheap", Models: map[string][]string{"cheap": {"luna:med"}}}}}
	_, err := resolveRoleSelection(spec, "developer", "", func(string) (Model, error) { return Model{Name: "luna"}, nil }, func(Model) string { return "disabled by user policy" })
	if err == nil || !strings.Contains(err.Error(), "default model") || !strings.Contains(err.Error(), "disabled by user policy") || !strings.Contains(err.Error(), "--model") {
		t.Fatalf("default error: %v", err)
	}
}

func TestLatestAccess(t *testing.T) {
	obs := []modelcatalog.AccessObservation{{Status: "denied", ObservedAt: "2026-01-01T00:00:00Z"}, {Status: "accessible", ObservedAt: "2026-02-01T00:00:00Z"}}
	if got := latestAccess(obs); got != "accessible" {
		t.Fatalf("latest %s", got)
	}
	if got := latestAccess(nil); got != "unknown" {
		t.Fatalf("empty %s", got)
	}
}

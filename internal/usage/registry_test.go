package usage

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUsageRegistryRunsIndependentCollectorsAndReportsDiagnostics(t *testing.T) {
	started := make(chan struct{})
	registry, err := NewRegistry(
		Collector{ID: "slow", Provider: "slow", Cadence: time.Minute, Timeout: time.Second, Collect: func(ctx context.Context) (AgentUsage, error) {
			close(started)
			<-ctx.Done()
			return AgentUsage{AgentID: "slow"}, ctx.Err()
		}},
		Collector{ID: "fast", Provider: "fast", Cadence: time.Minute, Timeout: time.Second, Collect: func(context.Context) (AgentUsage, error) {
			<-started
			return AgentUsage{AgentID: "fast", Session: &QuotaWindow{Name: "quota", UsedPercent: 25}}, nil
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics []CollectorDiagnostic
	got := registry.Collect(context.Background(), nil, func(d CollectorDiagnostic) { diagnostics = append(diagnostics, d) })
	if len(got) != 2 || len(diagnostics) != 2 {
		t.Fatalf("collectors=%d diagnostics=%d", len(got), len(diagnostics))
	}
	seen := map[string]CollectorDiagnostic{}
	for _, d := range diagnostics {
		seen[d.ID] = d
	}
	if seen["slow"].Duration <= 0 || seen["slow"].Error == "" {
		t.Errorf("slow diagnostic = %+v", seen["slow"])
	}
	if seen["fast"].Duration <= 0 || seen["fast"].Error != "" {
		t.Errorf("fast diagnostic = %+v", seen["fast"])
	}
}

func TestUsageNewRegistryDeclaresProviderCapabilitiesAndSpecPolicy(t *testing.T) {
	home := t.TempDir()
	registry, err := newUsageRegistry(home, http.DefaultClient, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(registry.collectors) != 3 {
		t.Fatalf("collectors = %d, want Claude/Codex/AGY", len(registry.collectors))
	}
	for _, c := range registry.collectors {
		if c.Cadence != 15*time.Minute || c.Timeout != 2*time.Minute || len(c.Capabilities) == 0 {
			t.Errorf("collector policy = %+v", c)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".gemini")); !os.IsNotExist(err) {
		t.Fatalf("registry construction unexpectedly accessed AGY home: %v", err)
	}
}

func TestUsageRegistryKeepsLastGoodReadingStaleAfterError(t *testing.T) {
	failed := false
	registry, err := NewRegistry(Collector{ID: "source", Provider: "source", Cadence: time.Minute, Timeout: time.Second, Collect: func(context.Context) (AgentUsage, error) {
		if failed {
			return AgentUsage{AgentID: "source"}, errors.New("offline")
		}
		return AgentUsage{AgentID: "source", LastRefreshed: time.Now(), Session: &QuotaWindow{Name: "quota", UsedPercent: 37}}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	registry.Collect(context.Background(), nil, nil)
	failed = true
	got := registry.Collect(context.Background(), nil, nil)
	if len(got) != 1 || got[0].Session == nil || got[0].Session.UsedPercent != 37 || got[0].QuotaFetchError != "offline" || got[0].Session.Name != "quota (stale)" {
		t.Fatalf("stale fallback = %+v", got)
	}
}

func TestUsageNewRegistryRejectsMissingPolicyAndDuplicateIDs(t *testing.T) {
	c := Collector{ID: "x", Provider: "x", Cadence: time.Minute, Timeout: time.Second, Collect: func(context.Context) (AgentUsage, error) { return AgentUsage{}, nil }}
	if _, err := NewRegistry(c, c); err == nil {
		t.Fatal("duplicate id accepted")
	}
	if _, err := NewRegistry(Collector{ID: "bad"}); err == nil {
		t.Fatal("incomplete collector accepted")
	}
}

package agent

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
	"ubunatic.com/harnez/internal/usagestore"
)

var now = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

func entry(provider, name string) subagent.ModelEntry {
	m := subagent.Model{Provider: provider, Name: name, Tier: "low"}
	return subagent.ModelEntry{Spec: provider + ":" + name + ":low", Model: m, Batch: true}
}

func key(e subagent.ModelEntry) string { return e.Model.Provider + "\x00" + e.Model.Name }

func isolateModelRows(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
}

func TestModelRowsFieldsFollowAvailability(t *testing.T) {
	isolateModelRows(t)
	avail, exhausted, blocked, stale, unknown := entry("codex", "a"), entry("codex", "b"), entry("agy", "c"), entry("codex", "d"), entry("codex", "e")
	reset := now.Add(time.Hour)
	until := now.Add(3 * time.Hour)
	availability := map[string]usage.ProviderQuotaAvailability{
		key(avail):     {State: "available", Age: 30 * time.Second, Windows: []usagestore.Window{{UsedFraction: 0.25}, {UsedFraction: 0.6}}},
		key(exhausted): {State: "exhausted", Exhausted: true, ResetIn: time.Hour, Age: time.Minute, Windows: []usagestore.Window{{UsedFraction: 1, ResetAt: &reset}}},
		key(blocked):   {State: "exhausted", Exhausted: true, ResetIn: 3 * time.Hour, BlockedUntil: until, BlockReason: "manual"},
		key(stale):     {State: "stale", Age: time.Hour},
	}
	enabled := func(m subagent.Model) bool { return m.Name != "d" }
	rows := ModelRows([]subagent.ModelEntry{avail, exhausted, blocked, stale, unknown}, availability, enabled, now)

	if r := rows[0]; !r.Startable || r.Quota.State != "available" || r.Quota.AgeS != 30 || r.Quota.RemainingPercent == nil || *r.Quota.RemainingPercent != 40 || r.Quota.Scope != "provider" {
		t.Errorf("available row = %+v", r)
	}
	if r := rows[1]; r.Startable || r.StartableReason != "quota exhausted" || r.Quota.ResetAt == nil || !r.Quota.ResetAt.Equal(reset) {
		t.Errorf("exhausted row = %+v", r)
	}
	if r := rows[2]; r.Startable || r.BlockedUntil == nil || !r.BlockedUntil.Equal(until) || r.BlockReason != "manual" || r.Quota.Scope != "model" || r.StartableReason != "blocked until 2026-11-05T15:00:00Z" {
		t.Errorf("blocked row = %+v", r)
	}
	if r := rows[3]; r.Quota.State != "stale" || r.Entitlement.State != "denied" || r.Startable || r.StartableReason != "model disabled by policy" {
		t.Errorf("stale/denied row = %+v", r)
	}
	if r := rows[4]; r.Quota.State != "unknown" || r.Entitlement.State != "unverified" || r.Entitlement.Source == "" || !r.Startable || r.Quota.RemainingPercent != nil {
		t.Errorf("unknown row = %+v", r)
	}
}

func TestModelRowsValidateAgainstSchema(t *testing.T) {
	isolateModelRows(t)
	e := entry("codex", "a")
	rows := ModelRows([]subagent.ModelEntry{e, entry("agy", "b")}, map[string]usage.ProviderQuotaAvailability{
		key(e): {State: "exhausted", Exhausted: true, ResetIn: time.Hour, BlockedUntil: now.Add(time.Hour), BlockReason: "x"},
	}, nil, now)
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	raw, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/agent-models-row.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(doc); err != nil {
		t.Fatalf("rows violate schema: %v\n%s", err, data)
	}
	// The schema must reject a row missing the consumer fields.
	if err := resolved.Validate([]any{map[string]any{"spec": "x"}}); err == nil {
		t.Fatal("schema accepted an incomplete row")
	}
}

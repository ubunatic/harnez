package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadSnapshotMapsLegacyFieldsWithoutDiagnostics(t *testing.T) {
	dir := t.TempDir()
	resetAt := "2026-10-01T12:00:00Z"
	data := `{
		"fetched_at":"2026-09-25T10:00:00Z",
		"usage":{
			"agent_id":"claude",
			"name":"Claude Code",
			"installed":true,
			"authenticated":true,
			"account":"u***@example.test",
			"plan_tier":"Max",
			"active_model":"claude-sonnet",
			"tokens":{"input_tokens":100,"cache_read_tokens":30,"total_tokens":130},
			"model_tokens":{"claude-sonnet":130},
			"session":{"name":"5-hour","source":"provider","used_percent":25,"remaining_percent":75,"reset_at":"` + resetAt + `","duration_left":60000000000,"is_active":true,"severity":"normal"},
			"weekly":{"name":"weekly","remaining_percent":60,"duration_left":-1000000},
			"extra_windows":{"spend":{"name":"spend","remaining_percent":90}},
			"model_groups":[{"name":"models","description":"Model family","windows":[{"name":"weekly","remaining_percent":50,"source":"provider"}]}],
			"last_refreshed":"2026-09-25T09:59:00Z",
			"sources":["/home/private/.credentials.json"],
			"details":{"access_token":"secret-token"},
			"quota_fetch_error":"provider returned private error text",
			"future_internal_field":"ignored"
		},
		"future_envelope_field":"ignored"
	}`
	writeSnapshotFile(t, dir, "claude", data)

	snapshot, err := ReadSnapshot(dir, ProviderClaude)
	if err != nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}
	if snapshot == nil {
		t.Fatal("ReadSnapshot returned nil snapshot")
	}
	if snapshot.SchemaVersion != SnapshotSchemaVersion || snapshot.ProviderID != ProviderClaude {
		t.Errorf("identity = (%d, %q), want (%d, %q)", snapshot.SchemaVersion, snapshot.ProviderID, SnapshotSchemaVersion, ProviderClaude)
	}
	if snapshot.Status != StatusError || snapshot.Error == nil || snapshot.Error.Category != "provider_fetch" {
		t.Errorf("legacy failure status = %q, error = %+v", snapshot.Status, snapshot.Error)
	}
	if snapshot.Source != "legacy" || !snapshot.ObservedAt.Equal(time.Date(2026, 9, 25, 9, 59, 0, 0, time.UTC)) {
		t.Errorf("provenance = %q at %v", snapshot.Source, snapshot.ObservedAt)
	}
	if snapshot.Usage.Tokens == nil || snapshot.Usage.Tokens.InputTokens != 100 || snapshot.Usage.Tokens.CacheReadTokens != 30 || snapshot.Usage.Tokens.TotalTokens != 130 {
		t.Errorf("tokens = %+v", snapshot.Usage.Tokens)
	}
	if snapshot.Usage.Name != "Claude Code" || snapshot.Usage.ModelTokens["claude-sonnet"] != 130 {
		t.Errorf("provider name/model tokens = %q / %+v", snapshot.Usage.Name, snapshot.Usage.ModelTokens)
	}
	if snapshot.Usage.Account != "u***@example.test" {
		t.Errorf("account = %q, want the legacy masked identifier preserved", snapshot.Usage.Account)
	}
	if snapshot.Usage.Session == nil || snapshot.Usage.Session.DurationLeftMS != 60000 || snapshot.Usage.Session.ResetAt == nil || snapshot.Usage.Session.Source != "provider" || !snapshot.Usage.Session.IsActive || snapshot.Usage.Session.Severity != "normal" {
		t.Errorf("session = %+v", snapshot.Usage.Session)
	}
	if snapshot.Usage.Weekly == nil || snapshot.Usage.Weekly.RemainingPercent != 60 || snapshot.Usage.Weekly.DurationLeftMS != 0 || snapshot.Usage.ExtraWindows["spend"].RemainingPercent != 90 {
		t.Errorf("weekly/extra windows = %+v / %+v", snapshot.Usage.Weekly, snapshot.Usage.ExtraWindows)
	}
	groupOK := len(snapshot.Usage.ModelGroups) == 1
	if groupOK {
		group := snapshot.Usage.ModelGroups[0]
		groupOK = group.Name == "models" && group.Description == "Model family" && len(group.Windows) == 1
		if groupOK {
			groupOK = group.Windows[0].RemainingPercent == 50 && group.Windows[0].Source == "provider"
		}
	}
	if !groupOK {
		t.Errorf("model groups = %+v", snapshot.Usage.ModelGroups)
	}

	publicJSON, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal public snapshot: %v", err)
	}
	for _, forbidden := range []string{"access_token", "secret-token", "private error text", "/home/private", "sources", "details"} {
		if strings.Contains(string(publicJSON), forbidden) {
			t.Errorf("public snapshot leaked %q: %s", forbidden, publicJSON)
		}
	}
}

func TestReadSnapshotAcceptsVersionedEnvelopeAndAdditiveFields(t *testing.T) {
	dir := t.TempDir()
	data := `{"schema_version":1,"provider_id":"agy","fetched_at":"2026-09-25T10:00:00Z","observed_at":"2026-09-25T09:59:00Z","status":"cached","source":"state_cache","error":{"category":"future_error_category"},"usage":{"installed":true,"authenticated":true,"weekly":{"remaining_percent":80}},"new_field":{"kept_by_writer":true}}`
	writeSnapshotFile(t, dir, "agy", data)

	snapshot, err := ReadSnapshot(dir, ProviderAGY)
	if err != nil {
		t.Fatalf("ReadSnapshot: %v", err)
	}
	if snapshot.Status != StatusCached || snapshot.Source != SourceStateCache || snapshot.Error == nil || snapshot.Error.Category != "future_error_category" || snapshot.Usage.Weekly == nil || snapshot.Usage.Weekly.RemainingPercent != 80 {
		t.Errorf("decoded snapshot = %+v", snapshot)
	}
}

func TestReadSnapshotMissingAndInvalidInputs(t *testing.T) {
	dir := t.TempDir()
	snapshot, err := ReadSnapshot(dir, ProviderCodex)
	if err != nil || snapshot != nil {
		t.Fatalf("missing snapshot = (%+v, %v), want (nil, nil)", snapshot, err)
	}
	if _, err := ReadSnapshot(dir, ProviderID("../secrets")); err == nil {
		t.Fatal("expected invalid provider ID error")
	}
	writeSnapshotFile(t, dir, "codex", `null`)
	if _, err := ReadSnapshot(dir, ProviderCodex); err == nil {
		t.Fatal("expected non-object snapshot error")
	}
}

func TestReadSnapshotRejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	writeSnapshotFile(t, dir, "codex", strings.Repeat(" ", maxSnapshotBytes+1))
	if _, err := ReadSnapshot(dir, ProviderCodex); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized snapshot error = %v", err)
	}
}

func TestReadSnapshotRejectsUnsupportedVersionAndProviderMismatch(t *testing.T) {
	dir := t.TempDir()
	writeSnapshotFile(t, dir, "claude", `{"schema_version":2,"provider_id":"claude","usage":{}}`)
	if _, err := ReadSnapshot(dir, ProviderClaude); err == nil || !strings.Contains(err.Error(), "unsupported snapshot schema version") {
		t.Fatalf("unsupported version error = %v", err)
	}
	writeSnapshotFile(t, dir, "claude", `{"schema_version":1,"provider_id":"codex","fetched_at":"2026-09-25T10:00:00Z","status":"live","usage":{}}`)
	if _, err := ReadSnapshot(dir, ProviderClaude); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("provider mismatch error = %v", err)
	}
}

func TestStateDirUsesXDGAndHomeFallback(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/state")
	if got, want := StateDir("/home/test"), filepath.Join("/tmp/state", "harnez", "agents", "usage"); got != want {
		t.Errorf("StateDir with XDG = %q, want %q", got, want)
	}
	t.Setenv("XDG_STATE_HOME", "")
	if got, want := StateDir("/home/test"), filepath.Join("/home/test", ".local", "state", "harnez", "agents", "usage"); got != want {
		t.Errorf("StateDir fallback = %q, want %q", got, want)
	}
}

func writeSnapshotFile(t *testing.T, dir, provider, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, provider+".json"), []byte(contents), 0600); err != nil {
		t.Fatalf("write %s snapshot: %v", provider, err)
	}
}

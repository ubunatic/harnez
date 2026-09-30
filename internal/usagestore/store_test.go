package usagestore

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAndCurrentIdempotentAndFractional(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "usage.sqlite")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := EnsureSchema(context.Background(), func(ctx context.Context, q string) error { return s.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	items := []Window{{Provider: "claude", Key: "5h", Name: "5h", Source: "collect-all", Freshness: "fresh", UsedFraction: .43125, ObservedAt: at}}
	for range 2 {
		if err := s.WriteCurrent(context.Background(), at, items); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].UsedFraction != .43125 || got[0].Freshness != "fresh" {
		t.Fatalf("Current() = %+v", got)
	}
}

func TestWriteLoadObservationPersistsNormalizedPayload(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := EnsureSchema(context.Background(), func(ctx context.Context, q string) error { return s.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	if err := s.WriteLoadObservation(context.Background(), "remote-load", at, map[string]any{"cpu_percent": 12.5, "cores": 8}); err != nil {
		t.Fatal(err)
	}
	var provider, observed, payload string
	if err := s.QueryRow(context.Background(), `SELECT provider,observed_at,payload_json FROM usage_load_observations`).Scan(&provider, &observed, &payload); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatal(err)
	}
	if provider != "remote-load" || observed != at.Format(time.RFC3339Nano) || decoded["cpu_percent"] != 12.5 {
		t.Fatalf("load row provider=%q observed=%q payload=%s", provider, observed, payload)
	}
}

func TestCurrentKeepsLatestAndStaleMetadata(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "usage.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := EnsureSchema(context.Background(), func(ctx context.Context, q string) error { return s.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	for _, w := range []Window{{Provider: "codex", Key: "weekly", Name: "Weekly", Source: "history", Freshness: "stale", UsedFraction: .99, ObservedAt: base}, {Provider: "codex", Key: "weekly", Name: "Weekly", Source: "collect-all", Freshness: "fresh", UsedFraction: .25, ObservedAt: base.Add(time.Minute)}} {
		if err := s.WriteCurrent(context.Background(), base, []Window{w}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Current(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].UsedFraction != .25 || got[0].Source != "collect-all" || got[0].Freshness != "fresh" {
		t.Fatalf("Current() = %+v", got)
	}
}

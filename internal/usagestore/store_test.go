package usagestore

import (
	"context"
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

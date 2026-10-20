package usage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/usagestore"
)

func TestQuotaNoticeFromSharedReadings(t *testing.T) {
	now := time.Date(2026, 10, 20, 10, 41, 47, 0, time.UTC)
	reset := time.Date(2026, 10, 20, 11, 52, 0, 0, time.UTC)
	for _, family := range []struct{ provider, model, pool string }{
		{"claude", "sonnet", ""}, {"codex", "luna", ""},
		{"agy", "sonnet", "Claude and GPT models"},
		{"agy", "gemini-3.8-flash", "Gemini Models"},
		{"agy", "gemini-4-argon", "Gemini 4 Argon"},
	} {
		for _, tc := range []struct {
			name      string
			used      float64
			age       time.Duration
			freshness string
			missing   bool
			want      string
		}{
			{"above", .86, 40 * time.Second, "fresh", false, "5h 86% (read 10:41:07, 40s ago, resets 11:52)"},
			{"below", .79, 40 * time.Second, "fresh", false, ""},
			{"threshold", .80, 40 * time.Second, "fresh", false, "5h 80% (read 10:41:07, 40s ago, resets 11:52)"},
			{"stale", .86, 23 * time.Minute, "fresh", false, "5h unknown, last reading 23m old (read 10:18:47)"},
			{"stale below", .10, 23 * time.Minute, "fresh", false, "5h unknown, last reading 23m old (read 10:18:47)"},
			{"marked stale", .86, 40 * time.Second, "stale", false, "5h unknown, last reading 40s old (read 10:41:07)"},
			{"max age", .86, 2 * time.Minute, "fresh", false, "5h 86% (read 10:39:47, 2m ago, resets 11:52)"},
			{"over max age", .86, 2*time.Minute + time.Millisecond, "fresh", false, "5h unknown, last reading 2m old (read 10:39:46)"},
			{"future timestamp", .86, -time.Minute, "fresh", false, ""},
			{"missing", 0, 0, "", true, ""},
		} {
			t.Run(family.provider+"/"+family.model+"/"+tc.name, func(t *testing.T) {
				ctx := context.Background()
				dbPath := filepath.Join(t.TempDir(), "usage.sqlite")
				store, err := usagestore.Open(dbPath)
				if err != nil {
					t.Fatal(err)
				}
				defer store.Close()
				if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
					t.Fatal(err)
				}
				if !tc.missing {
					windows := []usagestore.Window{{Provider: family.provider, Pool: family.pool, Key: "five_hour", Name: "5h", UsedFraction: tc.used, ObservedAt: now.Add(-tc.age), Freshness: tc.freshness, ResetAt: &reset}}
					if family.provider == "agy" {
						windows = append(windows, usagestore.Window{Provider: "agy", Pool: "unrelated", Key: "weekly", UsedFraction: 1, ObservedAt: now, Freshness: "fresh"})
					}
					if err := store.WriteCurrent(ctx, now, windows); err != nil {
						t.Fatal(err)
					}
				}
				state := ProviderQuotaAvailabilityFromStore(ctx, dbPath, family.provider, family.model, now)
				got := QuotaNotice(family.provider, state, now)
				want := tc.want
				if want != "" {
					want = "quota: " + family.provider + " " + want
				}
				if got != want {
					t.Fatalf("notice = %q, want %q", got, want)
				}
			})
		}
	}
}

func TestQuotaNoticeUsesEachWindowTimestamp(t *testing.T) {
	now := time.Date(2026, 10, 20, 10, 41, 47, 0, time.UTC)
	state := ProviderQuotaAvailability{Windows: []usagestore.Window{
		{Key: "five_hour", UsedFraction: .86, ObservedAt: now.Add(-40 * time.Second), Freshness: "fresh"},
		{Key: "weekly", UsedFraction: .99, ObservedAt: now.Add(-23 * time.Minute), Freshness: "fresh"},
		{Pool: "tokens", Key: "cumulative", UsedFraction: 1, ObservedAt: now, Freshness: "fresh"},
	}}
	want := "quota: codex 5h 86% (read 10:41:07, 40s ago); weekly unknown, last reading 23m old (read 10:18:47)"
	if got := QuotaNotice("codex", state, now); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	state.Windows[1].ObservedAt = now.Add(-time.Minute)
	if got := QuotaNotice("codex", state, now); !strings.Contains(got, "weekly 99% (read 10:40:47, 1m ago)") {
		t.Fatalf("both high windows: %q", got)
	}
}

func TestQuotaNoticePolicyValidation(t *testing.T) {
	for _, tc := range []struct {
		percent float64
		age     string
		valid   bool
	}{
		{80, "2m", true}, {100, "1s", true}, {0, "2m", false}, {-1, "2m", false}, {101, "2m", false},
		{80, "", false}, {80, "0s", false}, {80, "-1s", false}, {80, "bad", false},
	} {
		_, _, err := (usageSpec{AgentQuotaWarnPercent: tc.percent, AgentQuotaMaxReadingAge: tc.age}).quotaNoticePolicy()
		if (err == nil) != tc.valid {
			t.Errorf("policy (%g,%q) err=%v", tc.percent, tc.age, err)
		}
	}
	// Values must come from the supplied spec rather than Go fallbacks.
	percent, age, err := (usageSpec{AgentQuotaWarnPercent: 91, AgentQuotaMaxReadingAge: "7m"}).quotaNoticePolicy()
	if err != nil || percent != 91 || age != 7*time.Minute {
		t.Fatalf("custom policy=%g,%v,%v", percent, age, err)
	}
}

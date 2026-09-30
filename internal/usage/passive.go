package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usagestore"
)

type passivePayload struct {
	Model struct {
		ID string `json:"id"`
	} `json:"model"`
	ObservedAt time.Time `json:"observed_at"`
	RateLimits map[string]struct {
		UsedPercentage *float64 `json:"used_percentage"`
		ResetAt        string   `json:"reset_at"`
		ResetsAt       string   `json:"resets_at"`
	} `json:"rate_limits"`
	Quota map[string]struct {
		RemainingFraction *float64 `json:"remaining_fraction"`
		ResetAt           string   `json:"reset_at"`
		ResetsAt          string   `json:"resets_at"`
	} `json:"quota"`
}

// ObserveStatusline accepts only quota values and reset timestamps from a
// Claude or AGY statusline payload. It deliberately never stores the raw JSON.
func ObserveStatusline(parent context.Context, provider string, payload []byte) {
	dbPath, err := telemetry.DefaultDBPath()
	if err != nil {
		return
	}
	ObserveStatuslineAt(parent, provider, payload, dbPath)
}

// ObserveStatuslineAt is the testable form of ObserveStatusline with an
// explicit telemetry database path.
func ObserveStatuslineAt(parent context.Context, provider string, payload []byte, dbPath string) {
	if provider != "claude" && provider != "agy" {
		return
	}
	dedupe, busy, err := PassivePolicy()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 4*busy)
	defer cancel()
	store, err := usagestore.OpenWithBusyTimeout(dbPath, busy)
	if err != nil {
		return
	}
	defer store.Close()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, query string) error { return store.Exec(ctx, query) }); err != nil {
		return
	}
	if err := store.MigrateStableWindowKeys(ctx); err != nil {
		return
	}
	windows, err := passiveWindows(provider, payload, time.Now())
	if err != nil || len(windows) == 0 {
		return
	}
	_ = store.WritePassive(ctx, dedupe, windows)
}

func passiveWindows(provider string, data []byte, received time.Time) ([]usagestore.Window, error) {
	var in passivePayload
	if err := json.Unmarshal(data, &in); err != nil {
		return nil, fmt.Errorf("decode passive usage: %w", err)
	}
	at := in.ObservedAt
	if at.IsZero() || at.After(received.Add(time.Minute)) || at.Before(received.Add(-24*time.Hour)) {
		at = received
	}
	resetValue := func(resetAt, resetsAt string) *time.Time {
		value := resetAt
		if value == "" {
			value = resetsAt
		}
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05Z07:00"} {
			if t, err := time.Parse(layout, value); err == nil {
				return &t
			}
		}
		return nil
	}
	var windows []usagestore.Window
	if provider == "claude" {
		limits := []struct{ payloadKey, key, name string }{{"five_hour", "five_hour", "5-hour"}, {"seven_day", "weekly", "7-day"}, {"spend_limit", "spend_limit", "spend"}}
		sort.Slice(limits, func(i, j int) bool { return limits[i].key < limits[j].key })
		for _, item := range limits {
			limit, ok := in.RateLimits[item.payloadKey]
			if !ok || limit.UsedPercentage == nil || *limit.UsedPercentage < 0 || *limit.UsedPercentage > 100 {
				continue
			}
			windows = append(windows, usagestore.Window{Provider: provider, Key: item.key, Name: item.name, Source: "statusline", Freshness: "fresh", UsedFraction: *limit.UsedPercentage / 100, ResetAt: resetValue(limit.ResetAt, limit.ResetsAt), ObservedAt: at})
		}
	} else {
		for key, quota := range in.Quota {
			remaining := quota.RemainingFraction
			if remaining == nil || *remaining < 0 || *remaining > 1 {
				continue
			}
			pool := ""
			lower := strings.ToLower(key)
			if strings.HasPrefix(lower, "gemini-") {
				pool = "Gemini Models"
			} else if strings.HasPrefix(lower, "3p-") {
				pool = "Claude and GPT models"
			}
			name := key
			if _, suffix, ok := strings.Cut(key, "-"); ok && pool != "" {
				name = suffix
			}
			stableKey := usagestore.NormalizeWindowKey(provider, pool, name)
			windows = append(windows, usagestore.Window{Provider: provider, Pool: pool, Key: stableKey, Name: name, Source: "statusline", Freshness: "fresh", UsedFraction: 1 - *remaining, ResetAt: resetValue(quota.ResetAt, quota.ResetsAt), ObservedAt: at})
		}
	}
	return windows, nil
}

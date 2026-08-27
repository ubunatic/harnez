package usage

import (
	"testing"
	"time"
)

func TestBuildCodexQuotaWindow(t *testing.T) {
	now := time.Now()

	t.Run("short window labeled by hour count", func(t *testing.T) {
		pw := &CodexRateWindow{UsedPercent: 52, LimitWindowSeconds: 5 * 3600, ResetAfterSeconds: 3600}
		qw := buildCodexQuotaWindow(pw, now)
		if qw.Name != "5-Hour" {
			t.Errorf("Name = %q, want 5-Hour", qw.Name)
		}
		if qw.UsedPercent != 52 || qw.RemainingPercent != 48 {
			t.Errorf("UsedPercent/RemainingPercent = %v/%v, want 52/48", qw.UsedPercent, qw.RemainingPercent)
		}
		if qw.ResetAt == nil || qw.DurationLeft != time.Hour {
			t.Errorf("ResetAt/DurationLeft not derived from ResetAfterSeconds: %+v", qw)
		}
	})

	t.Run("day-or-longer window labeled Weekly", func(t *testing.T) {
		pw := &CodexRateWindow{UsedPercent: 30, LimitWindowSeconds: 7 * 86400}
		qw := buildCodexQuotaWindow(pw, now)
		if qw.Name != "Weekly" {
			t.Errorf("Name = %q, want Weekly", qw.Name)
		}
	})

	t.Run("resetAt takes priority over resetAfterSeconds", func(t *testing.T) {
		resetAt := now.Add(2 * time.Hour).Unix()
		pw := &CodexRateWindow{LimitWindowSeconds: 86400, ResetAt: resetAt, ResetAfterSeconds: 999999}
		qw := buildCodexQuotaWindow(pw, now)
		if qw.ResetAt == nil {
			t.Fatalf("expected ResetAt to be set")
		}
		if qw.DurationLeft <= time.Hour || qw.DurationLeft > 2*time.Hour+time.Minute {
			t.Errorf("DurationLeft = %v, want ~2h derived from ResetAt", qw.DurationLeft)
		}
	})
}

func TestCollectCodex_RateLimitWindowAssignment(t *testing.T) {
	now := time.Now()
	resp := CodexWhamUsageResponse{
		RateLimit: &struct {
			Allowed         bool             `json:"allowed"`
			LimitReached    bool             `json:"limit_reached"`
			PrimaryWindow   *CodexRateWindow `json:"primary_window"`
			SecondaryWindow *CodexRateWindow `json:"secondary_window"`
		}{
			PrimaryWindow:   &CodexRateWindow{UsedPercent: 45, LimitWindowSeconds: 5 * 3600},
			SecondaryWindow: &CodexRateWindow{UsedPercent: 12, LimitWindowSeconds: 7 * 86400},
		},
	}

	var usage AgentUsage
	for _, pw := range []*CodexRateWindow{resp.RateLimit.PrimaryWindow, resp.RateLimit.SecondaryWindow} {
		if pw == nil {
			continue
		}
		qw := buildCodexQuotaWindow(pw, now)
		if qw.Name == "Weekly" {
			usage.Weekly = &qw
		} else if usage.Session == nil {
			usage.Session = &qw
		}
	}

	if usage.Session == nil || usage.Session.Name != "5-Hour" || usage.Session.UsedPercent != 45 {
		t.Errorf("Session = %+v, want 5-Hour window at 45%%", usage.Session)
	}
	if usage.Weekly == nil || usage.Weekly.Name != "Weekly" || usage.Weekly.UsedPercent != 12 {
		t.Errorf("Weekly = %+v, want Weekly window at 12%%", usage.Weekly)
	}
}

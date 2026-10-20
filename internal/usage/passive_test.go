package usage

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/usagestore"
)

func TestPassiveWindowsPreserveClaudeFractionsAndReset(t *testing.T) {
	now := time.Now().UTC()
	data := []byte(`{"rate_limits":{"five_hour":{"used_percentage":0},"seven_day":{"used_percentage":100,"reset_at":"2000-01-01T00:00:00Z"}}}`)
	windows, err := passiveWindows("claude", data, now)
	if err != nil || len(windows) != 2 {
		t.Fatalf("windows=%+v err=%v", windows, err)
	}
	if windows[0].UsedFraction != 0 || windows[1].UsedFraction != 1 {
		t.Fatalf("fractions = %v, %v", windows[0].UsedFraction, windows[1].UsedFraction)
	}
	if windows[1].ResetAt == nil || !windows[1].ResetAt.Before(now) {
		t.Fatalf("past reset = %v", windows[1].ResetAt)
	}
}

func TestPassiveWindowsAcceptPartialAndAGYQuota(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		provider, payload string
		want              float64
	}{
		{"claude", `{"rate_limits":{"five_hour":{"used_percentage":37.125}}}`, .37125},
		{"agy", `{"model":{"id":"gemini-pro"},"quota":{"gemini-weekly":{"remaining_fraction":0.7592765}}}`, .2407235},
		{"claude", `{"rate_limits":{"seven_day":{}}}`, -1},
	} {
		got, err := passiveWindows(tc.provider, []byte(tc.payload), now)
		if err != nil {
			t.Fatal(err)
		}
		if tc.want < 0 {
			if len(got) != 0 {
				t.Errorf("partial quota produced %+v", got)
			}
			continue
		}
		if len(got) != 1 || math.Abs(got[0].UsedFraction-tc.want) > 1e-12 {
			t.Errorf("%s windows=%+v want fraction=%v", tc.provider, got, tc.want)
		}
	}
}

func TestPassiveWriteDeduplicatesAndBestReadingUsesPrecedence(t *testing.T) {
	store, err := usagestore.Open(filepath.Join(t.TempDir(), "u.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	if err := usagestore.EnsureSchema(ctx, func(ctx context.Context, q string) error { return store.Exec(ctx, q) }); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	api := usagestore.Window{Provider: "claude", Key: "five_hour", Name: "5-hour", Source: "claude-api", Freshness: "fresh", UsedFraction: .42, ObservedAt: now.Add(-time.Second)}
	status := api
	status.Source, status.UsedFraction, status.ObservedAt = "statusline", .37125, now
	if err := store.WriteCurrent(ctx, now, []usagestore.Window{api}); err != nil {
		t.Fatal(err)
	}
	if err := store.WritePassive(ctx, 30*time.Second, []usagestore.Window{status}); err != nil {
		t.Fatal(err)
	}
	status.ObservedAt = now.Add(time.Second)
	if err := store.WritePassive(ctx, 30*time.Second, []usagestore.Window{status}); err != nil {
		t.Fatal(err)
	}
	rows, err := store.CurrentFor(ctx, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].UsedFraction != .42 || rows[0].Source != "claude-api" {
		t.Fatalf("best reading = %+v", rows)
	}
	var n int
	if err := store.QueryRow(ctx, `SELECT count(*) FROM usage_observations WHERE source='statusline'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("statusline observation count=%d want one", n)
	}
}

func TestStatuslineSendsWindowsWithoutOpeningLockedStore(t *testing.T) {
	dataHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dataHome, "harnez"), 0700); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dataHome, "harnez", "telemetry.sqlite")
	blocker, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(0)")
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	if _, err := blocker.Exec(`CREATE TABLE lock_probe (x INTEGER)`); err != nil {
		t.Fatal(err)
	}
	tx, err := blocker.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO lock_probe VALUES (1)`); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("HOME", t.TempDir())
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{"rate_limits": map[string]any{"five_hour": map[string]any{"used_percentage": 12.5}}})
	if err != nil {
		t.Fatal(err)
	}
	posted := make(chan struct{}, 1)
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Windows []usagestore.Window `json:"windows"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Windows) != 1 || body.Windows[0].UsedFraction != .125 {
			t.Errorf("posted body = %+v, err=%v", body, err)
		}
		posted <- struct{}{}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer collector.Close()
	path, err := PassiveUpdatePath()
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	result := ObserveStatuslineTo(context.Background(), "claude", data, strings.TrimPrefix(collector.URL, "http://"), path, cfg.UpdateTimeout)
	if elapsed := time.Since(started); elapsed > cfg.UpdateTimeout {
		t.Fatalf("statusline took %s, update timeout %s", elapsed, cfg.UpdateTimeout)
	}
	if result != server.UpdateDelivered {
		t.Fatalf("update result = %q", result)
	}
	<-posted
	var count int
	if err := tx.QueryRow(`SELECT count(*) FROM lock_probe`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("locked store changed: count=%d err=%v", count, err)
	}
}

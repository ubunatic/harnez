package usage

import (
	"context"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
)

// A busy live-fetch lock must not drop the SQLite write. The store is the
// authoritative record that turn-quota baselines read, so a reading fetched
// while a sibling process holds the lock must still be persisted.
func TestCollectClaudeStoresFreshReadingWhenCacheLockIsBusy(t *testing.T) {
	dir := claudeFixtureDir(t)
	held, ok := lockLiveFetchCache(liveFetchCachePath(dir))
	if !ok {
		t.Fatal("test could not take the live-fetch lock")
	}
	defer unlockLiveFetchCache(held)

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"five_hour":{"utilization":71}}`)), Request: r}, nil
	})}
	u := CollectClaude(context.Background(), dir, client)
	if u.QuotaFetchError != "" {
		t.Fatalf("QuotaFetchError = %q; want a successful live fetch", u.QuotaFetchError)
	}
	at, snap, err := ProviderSnapshotFromStore(context.Background(), "claude")
	if err != nil {
		t.Fatal(err)
	}
	if at.IsZero() || snap.Session == nil || math.Abs(snap.Session.UsedPercent-71) > 1e-6 {
		t.Fatalf("store snapshot at=%v session=%+v; want the fresh 71%% session reading persisted despite the busy lock", at, snap.Session)
	}
}

package usage

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"ubunatic.com/harnez/internal/server"
)

// PassiveDropTracker counts valid statusline observations without usable quota
// fields. Counts span statusline processes and reset when the server restarts.
type PassiveDropTracker struct {
	mu     sync.Mutex
	counts map[string]uint64
}

func (t *PassiveDropTracker) record(provider string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.counts == nil {
		t.counts = make(map[string]uint64)
	}
	t.counts[provider]++
}

// Snapshot returns an independent copy of the server's counters.
func (t *PassiveDropTracker) Snapshot() map[string]uint64 {
	counts := map[string]uint64{"claude": 0, "agy": 0}
	if t == nil {
		return counts
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for provider, count := range t.counts {
		counts[provider] = count
	}
	return counts
}

// ReadPassiveDropCounts reads the current server counters, or nil if unavailable.
func ReadPassiveDropCounts(ctx context.Context) map[string]uint64 {
	cfg, err := server.LoadConfig()
	if err != nil {
		return nil
	}
	socket, err := cfg.Socket()
	if err != nil {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/status", nil)
	if err != nil {
		return nil
	}
	resp, err := server.UnixHTTPClient(socket, cfg.HTTPClientTimeout).Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var status struct {
		Drops map[string]uint64 `json:"passive_dropped"`
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		return nil
	}
	if json.NewDecoder(resp.Body).Decode(&status) != nil {
		return nil
	}
	return status.Drops
}

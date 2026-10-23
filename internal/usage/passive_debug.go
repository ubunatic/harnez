package usage

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/usagestore"
)

// PassiveDropTracker counts dropped and successfully persisted observations.
// Counts span statusline processes and reset when the server restarts.
type PassiveDropTracker struct {
	mu       sync.Mutex
	counts   map[string]uint64
	accepted map[string]uint64
}

// No environment-variable registry exists for passive usage diagnostics.
const passiveDropCaptureEnv = "HARNEZ_CAPTURE_PASSIVE_DROPS"

// capturePassiveDrop is best-effort and never affects statusline output. Store
// payload as a string so even malformed JSON is preserved byte-for-byte.
func capturePassiveDrop(provider string, payload []byte, reason string) {
	if os.Getenv(passiveDropCaptureEnv) == "" {
		return
	}
	base := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(base) {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		base = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(base, "harnez")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return
	}
	data, err := json.Marshal(struct {
		Provider   string    `json:"provider"`
		Reason     string    `json:"reason"`
		Payload    string    `json:"payload"`
		CapturedAt time.Time `json:"captured_at"`
	}{provider, reason, string(payload), time.Now().UTC()})
	if err != nil {
		return
	}
	f, err := os.CreateTemp(dir, ".passive-drop-*")
	if err != nil {
		return
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr == nil && closeErr == nil {
		_ = os.Rename(f.Name(), filepath.Join(dir, "passive-last-dropped.json"))
	}
}

func (t *PassiveDropTracker) recordAccepted(windows []usagestore.Window) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.accepted == nil {
		t.accepted = make(map[string]uint64)
	}
	seen := make(map[string]bool)
	for _, window := range windows {
		if !seen[window.Provider] {
			t.accepted[window.Provider]++
			seen[window.Provider] = true
		}
	}
}

func (t *PassiveDropTracker) AcceptedSnapshot() map[string]uint64 {
	counts := map[string]uint64{"claude": 0, "agy": 0}
	if t == nil {
		return counts
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for provider, count := range t.accepted {
		counts[provider] = count
	}
	return counts
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

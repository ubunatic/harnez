package usage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"ubunatic.com/harnez/internal/server"
)

// ClientTracker tracks watching client heartbeats and counts active clients (issue 714 M3).
type ClientTracker struct {
	mu            sync.Mutex
	timeout       time.Duration
	clients       map[string]time.Time
	onFirstClient func()
	now           func() time.Time
}

// NewClientTracker creates a new ClientTracker. onFirstClient (optional) is called
// when the number of active clients transitions from 0 to > 0.
func NewClientTracker(timeout time.Duration, onFirstClient ...func()) *ClientTracker {
	var cb func()
	if len(onFirstClient) > 0 {
		cb = onFirstClient[0]
	}
	return &ClientTracker{
		timeout:       timeout,
		clients:       make(map[string]time.Time),
		onFirstClient: cb,
		now:           time.Now,
	}
}

// SetOnFirstClient sets or updates the callback called on 0->1 active client transition.
func (ct *ClientTracker) SetOnFirstClient(cb func()) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.onFirstClient = cb
}

// Heartbeat registers or refreshes a client heartbeat.
func (ct *ClientTracker) Heartbeat(clientID string) {
	if clientID == "" {
		clientID = generateClientID()
	}
	ct.mu.Lock()

	now := ct.now()
	ct.pruneExpiredLocked(now)

	prevCount := len(ct.clients)
	ct.clients[clientID] = now

	var notify func()
	if prevCount == 0 && len(ct.clients) > 0 && ct.onFirstClient != nil {
		notify = ct.onFirstClient
	}
	ct.mu.Unlock()
	if notify != nil {
		notify()
	}
}

// Count returns the number of currently active clients.
func (ct *ClientTracker) Count() int {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.pruneExpiredLocked(ct.now())
	return len(ct.clients)
}

func (ct *ClientTracker) pruneExpiredLocked(now time.Time) {
	for id, lastSeen := range ct.clients {
		if now.Sub(lastSeen) > ct.timeout {
			delete(ct.clients, id)
		}
	}
}

func generateClientID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// HeartbeatServer handles client heartbeats on the server Unix socket.
type HeartbeatServer struct {
	Path    string
	Tracker *ClientTracker
	Timeout time.Duration
}

type heartbeatPayload struct {
	ClientID string `json:"client_id"`
}

func (s HeartbeatServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+s.Path, s.handleHeartbeat)
}

func (s HeartbeatServer) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var payload heartbeatPayload
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}
	if s.Tracker != nil {
		s.Tracker.Heartbeat(payload.ClientID)
	}
	w.WriteHeader(http.StatusNoContent)
}

// StartClientHeartbeat starts sending periodic heartbeats to the server Unix socket in the background
// until ctx is canceled (issue 714 M3).
func StartClientHeartbeat(ctx context.Context) {
	interval, _, _, err := ClientHeartbeatPolicy()
	if err != nil || interval <= 0 {
		return
	}
	cfg, err := server.LoadConfig()
	if err != nil {
		return
	}
	socket, err := cfg.Socket()
	if err != nil {
		return
	}
	clientID := generateClientID()
	body, _ := json.Marshal(heartbeatPayload{ClientID: clientID})

	send := func() {
		_ = server.PostUpdate(ctx, socket, "/usage/heartbeat", body, cfg.UpdateTimeout)
	}

	send()
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				send()
			}
		}
	}()
}

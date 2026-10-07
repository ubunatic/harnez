package channel

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

// Server bridges one authenticated loopback endpoint to its host's MCP stdio.
type Server struct {
	Config    Config
	SessionID string
	In        io.ReadCloser
	Out       io.Writer
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
}

type transport struct {
	mu      sync.Mutex
	encoder *json.Encoder
}

func (t *transport) write(message any) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.encoder.Encode(message)
}

type messageRequest struct {
	SessionID string `json:"session_id"`
	Content   string `json:"content"`
}

// Accepted confirms an MCP transport write, never Claude receipt or processing.
type Accepted struct {
	Status    string `json:"status"`
	Receipt   string `json:"receipt"`
	SessionID string `json:"session_id"`
	MessageID string `json:"message_id"`
}

// Run serves until stdin EOF, context cancellation, or a transport failure.
// Claude starts the process; this function never starts an agent or a Claude CLI.
func (s Server) Run(ctx context.Context) (runErr error) {
	if err := s.Config.validate(); err != nil {
		return err
	}
	if strings.TrimSpace(s.SessionID) == "" || s.In == nil || s.Out == nil {
		return fmt.Errorf("channel requires CLAUDE_CODE_SESSION_ID and MCP input/output")
	}
	listener, err := net.Listen("tcp", s.Config.ListenAddress)
	if err != nil {
		return fmt.Errorf("channel listen: %w", err)
	}
	defer listener.Close()
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	reg := registration{
		Endpoint: Endpoint{SessionID: s.SessionID, PID: os.Getpid(), URL: "http://" + listener.Addr().String(), WorkingDir: workingDir},
		Token:    rand.Text(),
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Interrupt a blocked stdin read on SIGTERM/cancellation without a reader zombie.
	stopClose := context.AfterFunc(ctx, func() { _ = s.In.Close() })
	defer stopClose()
	output := &transport{encoder: json.NewEncoder(s.Out)}
	var ready atomic.Bool
	handler := channelHandler(s.Config, reg, &ready, output, cancel)
	httpServer := &http.Server{
		Handler: handler, ReadHeaderTimeout: s.Config.timeout(),
		ReadTimeout: s.Config.timeout(), WriteTimeout: s.Config.timeout(),
	}
	served := make(chan error, 1)
	go func() {
		err := httpServer.Serve(listener)
		served <- err
		cancel()
	}()
	registered := false
	defer func() {
		ready.Store(false)
		_ = httpServer.Close()
		if err := <-served; err != nil && !errors.Is(err, http.ErrServerClosed) && runErr == nil {
			runErr = err
		}
		if registered {
			if err := unregister(s.Config, reg); err != nil && runErr == nil {
				runErr = fmt.Errorf("remove channel registration: %w", err)
			}
		}
	}()
	scanner := bufio.NewScanner(s.In)
	scanner.Buffer(make([]byte, min(4096, s.Config.MaxRPCBytes)), s.Config.MaxRPCBytes)
	initialized := false
	for scanner.Scan() {
		var req rpcRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			if err := output.write(rpcFailure(nil, -32700, "parse error")); err != nil {
				return err
			}
			continue
		}
		if req.JSONRPC != "2.0" || req.Method == "" {
			if err := output.write(rpcFailure(req.ID, -32600, "invalid request")); err != nil {
				return err
			}
			continue
		}
		if req.Method == "notifications/initialized" {
			if initialized && !registered {
				if err := register(ctx, s.Config, reg); err != nil {
					return fmt.Errorf("register channel: %w", err)
				}
				registered = true
				ready.Store(true)
			}
			continue
		}
		if len(req.ID) == 0 {
			continue
		}
		var response any
		switch req.Method {
		case "initialize":
			response = map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{
				"protocolVersion": s.Config.ProtocolVersion,
				"serverInfo":      map[string]string{"name": s.Config.ServerName, "version": s.Config.ServerVersion},
				"capabilities":    map[string]any{"experimental": map[string]any{"claude/channel": map[string]any{}}},
				"instructions":    s.Config.Instructions,
			}}
			initialized = true
		case "ping":
			response = map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{}}
		default:
			response = rpcFailure(req.ID, -32601, "method not found")
		}
		if err := output.write(response); err != nil {
			return fmt.Errorf("channel MCP write: %w", err)
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	return scanner.Err()
}

func rpcFailure(id json.RawMessage, code int, message string) any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}}
}

func channelHandler(cfg Config, reg registration, ready *atomic.Bool, output *transport, cancel context.CancelFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+reg.Token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/health" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(reg.Endpoint)
			return
		}
		if r.URL.Path != "/message" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		if !ready.Load() {
			http.Error(w, "MCP initialization not complete", http.StatusServiceUnavailable)
			return
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, int64(cfg.MaxMessageBytes)))
		decoder.DisallowUnknownFields()
		var message messageRequest
		if err := decoder.Decode(&message); err != nil {
			http.Error(w, "invalid or oversized message", http.StatusBadRequest)
			return
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF || message.SessionID != reg.SessionID || strings.TrimSpace(message.Content) == "" {
			http.Error(w, "expected one nonempty message for this session", http.StatusBadRequest)
			return
		}
		id := rand.Text()
		if err := output.write(map[string]any{
			"jsonrpc": "2.0", "method": "notifications/claude/channel",
			"params": map[string]any{"content": message.Content, "meta": map[string]string{"session_id": reg.SessionID, "message_id": id}},
		}); err != nil {
			http.Error(w, "MCP transport write failed", http.StatusServiceUnavailable)
			cancel()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(Accepted{Status: "accepted", Receipt: "unconfirmed", SessionID: reg.SessionID, MessageID: id})
	})
}

package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type rpcCapture struct {
	messages chan map[string]any
	fail     atomic.Bool
}

func (w *rpcCapture) Write(data []byte) (int, error) {
	if w.fail.Load() {
		return 0, io.ErrClosedPipe
	}
	var message map[string]any
	if err := json.Unmarshal(data, &message); err != nil {
		return 0, err
	}
	w.messages <- message
	return len(data), nil
}

type fixture struct {
	cfg    Config
	input  *io.PipeWriter
	output *rpcCapture
	cancel context.CancelFunc
	done   chan struct{}
	err    error
	init   map[string]any
}

func testConfig(t *testing.T) Config {
	t.Helper()
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.RegistryDir = t.TempDir()
	return cfg
}

func startFixture(t *testing.T, cfg Config, sessionID string) *fixture {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	input, writer := io.Pipe()
	f := &fixture{cfg: cfg, input: writer, output: &rpcCapture{messages: make(chan map[string]any, 100)}, cancel: cancel, done: make(chan struct{})}
	go func() {
		f.err = (Server{Config: cfg, SessionID: sessionID, In: input, Out: f.output}).Run(ctx)
		_ = input.Close()
		close(f.done)
	}()
	t.Cleanup(func() {
		cancel()
		_ = writer.Close()
		_ = input.Close()
		f.wait(t)
	})
	request, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{
		"protocolVersion": cfg.ProtocolVersion, "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "canary-test", "version": "1"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	f.write(t, string(request))
	f.init = f.read(t)
	f.write(t, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	// A response after initialized gives a deterministic registry-ready boundary.
	f.write(t, `{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	_ = f.read(t)
	return f
}

func (f *fixture) write(t *testing.T, line string) {
	t.Helper()
	if _, err := io.WriteString(f.input, line+"\n"); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) read(t *testing.T) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), f.cfg.timeout())
	defer cancel()
	select {
	case message := <-f.output.messages:
		return message
	case <-f.done:
		t.Fatalf("server exited before response: %v", f.err)
	case <-ctx.Done():
		t.Fatal("MCP response timed out")
	}
	return nil
}

func (f *fixture) wait(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), f.cfg.timeout())
	defer cancel()
	select {
	case <-f.done:
		if f.err != nil {
			t.Errorf("server exit: %v", f.err)
		}
	case <-ctx.Done():
		t.Error("server did not stop on EOF/cancellation")
	}
}

func TestMCPChannelDeliveryAndCleanExit(t *testing.T) {
	cfg := testConfig(t)
	f := startFixture(t, cfg, "claude-target")
	result := f.init["result"].(map[string]any)
	if result["protocolVersion"] != cfg.ProtocolVersion || result["instructions"] != cfg.Instructions {
		t.Fatalf("initialize does not use embedded spec: %#v", result)
	}
	capabilities := result["capabilities"].(map[string]any)
	experimental := capabilities["experimental"].(map[string]any)
	if channel, ok := experimental["claude/channel"].(map[string]any); !ok || len(channel) != 0 {
		t.Fatalf("missing channel capability: %#v", capabilities)
	}
	if _, ok := experimental["claude/channel/permission"]; ok {
		t.Fatal("M1 must not relay permissions")
	}
	endpoints, err := List(context.Background(), cfg)
	if err != nil || len(endpoints) != 1 || endpoints[0].SessionID != "claude-target" {
		t.Fatalf("discovery = %#v, %v", endpoints, err)
	}
	reg, err := lookup(context.Background(), cfg, "claude-target")
	if err != nil {
		t.Fatal(err)
	}
	for path, mode := range map[string]os.FileMode{cfg.RegistryDir: 0700, registryPath(cfg, reg.SessionID): 0600} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("private registry permissions for %s: %v, %v", path, info, err)
		}
	}
	publicJSON, _ := json.Marshal(endpoints)
	if bytes.Contains(publicJSON, []byte(reg.Token)) {
		t.Fatal("list leaked authentication token")
	}
	content := "line one\nλ <quoted> \"message\""
	accepted, err := Send(context.Background(), cfg, "claude-target", content)
	if err != nil || accepted.Status != "accepted" || accepted.Receipt != "unconfirmed" {
		t.Fatalf("send = %#v, %v", accepted, err)
	}
	notification := f.read(t)
	if notification["method"] != "notifications/claude/channel" || notification["id"] != nil {
		t.Fatalf("not an MCP channel notification: %#v", notification)
	}
	params := notification["params"].(map[string]any)
	meta := params["meta"].(map[string]any)
	if params["content"] != content || meta["session_id"] != "claude-target" || meta["message_id"] != accepted.MessageID {
		t.Fatalf("wrong message routing or encoding: %#v", notification)
	}
	_ = f.input.Close()
	f.wait(t)
	if _, err := os.Stat(registryPath(cfg, "claude-target")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("entry survived clean EOF: %v", err)
	}
	if err := checkRegistration(context.Background(), cfg, reg); !errors.Is(err, ErrStaleSession) {
		t.Fatalf("listener survived clean EOF: %v", err)
	}
}

func TestConcurrentNotificationsAndRPCResponses(t *testing.T) {
	cfg := testConfig(t)
	f := startFixture(t, cfg, "concurrent")
	var group sync.WaitGroup
	for range 12 {
		group.Go(func() {
			if _, err := Send(context.Background(), cfg, "concurrent", "concurrent message"); err != nil {
				t.Errorf("concurrent send: %v", err)
			}
		})
	}
	f.write(t, `{"jsonrpc":"2.0","id":3,"method":"ping"}`)
	group.Wait()
	seen := make(map[string]bool)
	pings := 0
	for range 13 {
		response := f.read(t)
		if response["method"] == "notifications/claude/channel" {
			meta := response["params"].(map[string]any)["meta"].(map[string]any)
			seen[meta["message_id"].(string)] = true
		} else if response["id"] == float64(3) {
			pings++
		} else {
			t.Fatalf("interleaved or malformed output: %#v", response)
		}
	}
	if len(seen) != 12 || pings != 1 {
		t.Fatalf("lost events or response: messages=%d pings=%d", len(seen), pings)
	}
}

func TestCancellationRemovesRegistration(t *testing.T) {
	cfg := testConfig(t)
	f := startFixture(t, cfg, "cancel")
	f.cancel()
	f.wait(t)
	if _, err := os.Stat(registryPath(cfg, "cancel")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("entry survived cancellation: %v", err)
	}
}

func TestNonLoopbackBindRefused(t *testing.T) {
	for _, address := range []string{"0.0.0.0:0", "192.0.2.1:0", "[::]:0", "localhost:0"} {
		t.Run(address, func(t *testing.T) {
			cfg := testConfig(t)
			cfg.ListenAddress = address
			err := (Server{Config: cfg, SessionID: "target", In: io.NopCloser(strings.NewReader("")), Out: io.Discard}).Run(context.Background())
			if err == nil || !strings.Contains(err.Error(), "loopback") {
				t.Fatalf("non-loopback bind not refused: %v", err)
			}
		})
	}
	for _, address := range []string{"127.0.0.1:0", "[::1]:0"} {
		if err := validateLoopback(address); err != nil {
			t.Errorf("valid loopback address %s: %v", address, err)
		}
	}
}

func TestInboundGatingAndLimits(t *testing.T) {
	cfg := testConfig(t)
	reg := registration{Endpoint: Endpoint{SessionID: "target"}, Token: "test-token"}
	output := &rpcCapture{messages: make(chan map[string]any, 1)}
	var ready atomic.Bool
	handler := channelHandler(cfg, reg, &ready, &transport{encoder: json.NewEncoder(output)}, func() {}, newReceipts(cfg))
	for _, tc := range []struct {
		name, token, body string
		ready             bool
		status            int
	}{
		{"unauthorized", "wrong", `{"session_id":"target","content":"hello"}`, true, http.StatusUnauthorized},
		{"not initialized", reg.Token, `{"session_id":"target","content":"hello"}`, false, http.StatusServiceUnavailable},
		{"wrong target", reg.Token, `{"session_id":"other","content":"hello"}`, true, http.StatusBadRequest},
		{"empty", reg.Token, `{"session_id":"target","content":" "}`, true, http.StatusBadRequest},
		{"unknown field", reg.Token, `{"session_id":"target","content":"hello","extra":true}`, true, http.StatusBadRequest},
		{"trailing JSON", reg.Token, `{"session_id":"target","content":"hello"}{}`, true, http.StatusBadRequest},
		{"oversized", reg.Token, `{"session_id":"target","content":"` + strings.Repeat("x", cfg.MaxMessageBytes) + `"}`, true, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ready.Store(tc.ready)
			req := httptest.NewRequest(http.MethodPost, "/message", strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+tc.token)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != tc.status || len(output.messages) != 0 {
				t.Fatalf("status=%d, emitted=%d", response.Code, len(output.messages))
			}
		})
	}
}

func TestTransportFailureDoesNotReportAccepted(t *testing.T) {
	cfg := testConfig(t)
	f := startFixture(t, cfg, "broken-transport")
	f.output.fail.Store(true)
	if _, err := Send(context.Background(), cfg, "broken-transport", "hello"); err == nil {
		t.Fatal("failed transport reported accepted")
	}
	f.wait(t)
}

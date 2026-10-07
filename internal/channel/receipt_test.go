package channel

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReplyToolAndReceipt(t *testing.T) {
	cfg := testConfig(t)
	f := startFixture(t, cfg, "recipient")
	if _, ok := f.init["result"].(map[string]any)["capabilities"].(map[string]any)["tools"]; !ok {
		t.Fatal("tools capability missing")
	}
	f.write(t, `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`)
	tools := f.read(t)["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["name"] != cfg.ReplyToolName {
		t.Fatalf("wrong tools: %#v", tools)
	}
	accepted, err := Send(context.Background(), cfg, "recipient", "ack this")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.read(t)
	// Ack before the client starts waiting: receipt must not be lost.
	call := func(id, text string) bool {
		t.Helper()
		data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": map[string]any{"name": cfg.ReplyToolName, "arguments": map[string]string{"message_id": id, "text": text}}})
		f.write(t, string(data))
		return f.read(t)["result"].(map[string]any)["isError"].(bool)
	}
	if !call("unknown", "") {
		t.Fatal("accepted unknown acknowledgement")
	}
	if call(accepted.MessageID, "hello") || call(accepted.MessageID, "changed duplicate") {
		t.Fatal("valid/duplicate acknowledgement failed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout())
	defer cancel()
	receipt, err := WaitReceipt(ctx, cfg, accepted)
	if err != nil || receipt.Status != "received" || receipt.MessageID != accepted.MessageID || receipt.Text != "hello" {
		t.Fatalf("receipt = %#v, %v", receipt, err)
	}
	reg, err := lookup(ctx, cfg, "recipient")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := channelClient(cfg).Get(reg.URL + "/receipt?message_id=" + accepted.MessageID)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatal("receipt lookup leaked without authentication")
	}
}

func TestReceiptWaitTimeoutAndCancellation(t *testing.T) {
	cfg := testConfig(t)
	cfg.ReceiptPollMS = 1
	f := startFixture(t, cfg, "quiet")
	accepted, err := Send(context.Background(), cfg, "quiet", "no ack")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.read(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	receipt, err := WaitReceipt(ctx, cfg, accepted)
	if !errors.Is(err, context.DeadlineExceeded) || receipt.Status != "unconfirmed" {
		t.Fatalf("timeout = %#v, %v", receipt, err)
	}
	ctx, stop := context.WithCancel(context.Background())
	stop()
	if _, err := WaitReceipt(ctx, cfg, accepted); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel = %v", err)
	}
}

func TestReceiptBoundsExpiryAndToolArguments(t *testing.T) {
	cfg := testConfig(t)
	cfg.MaxReceipts = 1
	r := newReceipts(cfg)
	now := time.Now()
	r.now = func() time.Time { return now }
	if err := r.reserve("s", "m"); err != nil {
		t.Fatal(err)
	}
	if err := r.reserve("s", "other"); err == nil {
		t.Fatal("capacity not bounded")
	}
	for _, params := range []string{`{`, `{"name":"wrong","arguments":{"message_id":"m"}}`, `{"name":"reply","arguments":{"message_id":42}}`, `{"name":"reply","arguments":{}}`} {
		if !r.toolResult(cfg, json.RawMessage(params)).(map[string]any)["isError"].(bool) {
			t.Fatalf("invalid arguments accepted: %s", params)
		}
	}
	data, _ := json.Marshal(map[string]any{"name": cfg.ReplyToolName, "arguments": map[string]string{"message_id": "m", "text": strings.Repeat("x", cfg.MaxMessageBytes+1)}})
	if !r.toolResult(cfg, data).(map[string]any)["isError"].(bool) {
		t.Fatal("oversized reply accepted")
	}
	now = now.Add(time.Duration(cfg.ReceiptRetentionMS) * time.Millisecond)
	if _, ok := r.get("m"); ok {
		t.Fatal("receipt survived expiry")
	}
	if err := r.ack("m", "late"); err == nil {
		t.Fatal("expired ack accepted")
	}
	if err := r.reserve("s", "other"); err != nil {
		t.Fatal("expired entry did not free capacity")
	}
}

func TestWaitRejectsWrongReceiptIdentity(t *testing.T) {
	cfg := testConfig(t)
	var endpoint Endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(endpoint)
			return
		}
		_ = json.NewEncoder(w).Encode(Receipt{Status: "received", SessionID: "someone-else", MessageID: "m"})
	}))
	defer server.Close()
	endpoint = Endpoint{SessionID: "s", PID: os.Getpid(), URL: server.URL}
	reg := registration{Endpoint: endpoint, Token: "private"}
	if err := register(context.Background(), cfg, reg); err != nil {
		t.Fatal(err)
	}
	if _, err := WaitReceipt(context.Background(), cfg, Accepted{SessionID: "s", MessageID: "m"}); err == nil {
		t.Fatal("wrong receipt identity accepted")
	}
}

// Keep the rejected reply shape observable as a standard MCP tool result.
func TestReplyResultShape(t *testing.T) {
	data, err := json.Marshal(replyResult("unknown message", true))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"isError":true`) || !strings.Contains(string(data), `"type":"text"`) {
		t.Fatalf("wrong MCP result %s", data)
	}
}

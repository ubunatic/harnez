package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Receipt records an optional recipient acknowledgement, not task completion.
type Receipt struct {
	Status    string `json:"status"`
	SessionID string `json:"session_id"`
	MessageID string `json:"message_id"`
	Text      string `json:"text,omitempty"`
}

type receiptEntry struct {
	receipt Receipt
	expires time.Time
}

type receipts struct {
	mu      sync.Mutex
	entries map[string]receiptEntry
	cfg     Config
	now     func() time.Time
}

func newReceipts(cfg Config) *receipts {
	return &receipts{entries: make(map[string]receiptEntry), cfg: cfg, now: time.Now}
}

func (r *receipts) prune() {
	for id, entry := range r.entries {
		if !r.now().Before(entry.expires) {
			delete(r.entries, id)
		}
	}
}

func (r *receipts) reserve(sessionID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prune()
	if len(r.entries) >= r.cfg.MaxReceipts {
		return fmt.Errorf("channel receipt capacity reached")
	}
	r.entries[id] = receiptEntry{Receipt{Status: "unconfirmed", SessionID: sessionID, MessageID: id}, r.now().Add(time.Duration(r.cfg.ReceiptRetentionMS) * time.Millisecond)}
	return nil
}

func (r *receipts) get(id string) (Receipt, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prune()
	entry, ok := r.entries[id]
	return entry.receipt, ok
}

func (r *receipts) ack(id, text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prune()
	entry, ok := r.entries[id]
	if !ok {
		return fmt.Errorf("unknown or expired message_id")
	}
	if entry.receipt.Status != "received" {
		entry.receipt.Status = "received"
		entry.receipt.Text = text
		r.entries[id] = entry
	}
	return nil
}

func (r *receipts) toolResult(cfg Config, params json.RawMessage) any {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	var args struct {
		MessageID string `json:"message_id"`
		Text      string `json:"text"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return replyResult("invalid reply arguments", true)
	}
	decoder := json.NewDecoder(bytes.NewReader(call.Arguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return replyResult("invalid reply arguments", true)
	}
	if call.Name != cfg.ReplyToolName {
		return replyResult("unknown tool", true)
	}
	if len(args.Text) > cfg.MaxMessageBytes {
		return replyResult("reply text exceeds message limit", true)
	}
	if err := r.ack(args.MessageID, args.Text); err != nil {
		return replyResult(err.Error(), true)
	}
	return replyResult("received", false)
}

func replyResult(text string, failed bool) any {
	return map[string]any{"content": []any{map[string]string{"type": "text", "text": text}}, "isError": failed}
}

func replyTool(cfg Config) any {
	return map[string]any{"name": cfg.ReplyToolName, "description": cfg.ReplyToolDescription,
		"inputSchema": map[string]any{"type": "object", "additionalProperties": false,
			"required": []string{"message_id"}, "properties": map[string]any{
				"message_id": map[string]any{"type": "string", "minLength": 1},
				"text":       map[string]any{"type": "string"},
			}}}
}

// WaitReceipt waits for a matching acknowledgement until ctx ends. Callers set
// their deadline from the spec or an explicit positive timeout override.
func WaitReceipt(ctx context.Context, cfg Config, accepted Accepted) (Receipt, error) {
	result := Receipt{Status: "unconfirmed", SessionID: accepted.SessionID, MessageID: accepted.MessageID}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	reg, err := lookup(ctx, cfg, accepted.SessionID)
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if err != nil {
		return result, err
	}
	ticker := time.NewTicker(time.Duration(cfg.ReceiptPollMS) * time.Millisecond)
	defer ticker.Stop()
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, reg.URL+"/receipt?message_id="+url.QueryEscape(accepted.MessageID), nil)
		if err != nil {
			return result, err
		}
		request.Header.Set("Authorization", "Bearer "+reg.Token)
		response, err := channelClient(cfg).Do(request)
		if err != nil {
			return result, err
		}
		var receipt Receipt
		err = json.NewDecoder(io.LimitReader(response.Body, int64(cfg.MaxRPCBytes))).Decode(&receipt)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return result, fmt.Errorf("channel receipt: HTTP %d (server may need restart or receipt expired)", response.StatusCode)
		}
		if err != nil {
			return result, fmt.Errorf("channel receipt: %w", err)
		}
		if receipt.SessionID != accepted.SessionID || receipt.MessageID != accepted.MessageID || (receipt.Status != "received" && receipt.Status != "unconfirmed") {
			return result, fmt.Errorf("invalid channel receipt response")
		}
		if receipt.Status == "received" {
			return receipt, nil
		}
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-ticker.C:
		}
	}
}

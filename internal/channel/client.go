package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Send delivers one message to a live, authenticated endpoint by provider session ID.
func Send(ctx context.Context, cfg Config, sessionID, content string) (Accepted, error) {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(content) == "" {
		return Accepted{}, fmt.Errorf("channel send requires a session ID and nonempty message")
	}
	reg, err := lookup(ctx, cfg, sessionID)
	if err != nil {
		return Accepted{}, err
	}
	data, err := json.Marshal(messageRequest{SessionID: sessionID, Content: content})
	if err != nil {
		return Accepted{}, err
	}
	if len(data) > cfg.MaxMessageBytes {
		return Accepted{}, fmt.Errorf("channel message exceeds the spec's %d-byte limit", cfg.MaxMessageBytes)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reg.URL+"/message", bytes.NewReader(data))
	if err != nil {
		return Accepted{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+reg.Token)
	resp, err := channelClient(cfg).Do(req)
	if err != nil {
		return Accepted{}, fmt.Errorf("channel send (receipt unknown): %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return Accepted{}, fmt.Errorf("channel send: HTTP %d", resp.StatusCode)
	}
	var result Accepted
	if err := json.NewDecoder(io.LimitReader(resp.Body, int64(cfg.MaxRPCBytes))).Decode(&result); err != nil {
		return result, fmt.Errorf("channel accepted response: %w", err)
	}
	if result.Status != "accepted" || result.Receipt != "unconfirmed" || result.SessionID != sessionID || result.MessageID == "" {
		return Accepted{}, fmt.Errorf("invalid channel accepted response")
	}
	return result, nil
}

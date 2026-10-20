package server

import (
	"bytes"
	"context"
	"net/http"
	"time"
)

type UpdateResult string

const (
	UpdateDelivered UpdateResult = "delivered"
	UpdateDropped   UpdateResult = "dropped"
)

func PostUpdate(ctx context.Context, address, path string, payload []byte, timeout time.Duration) UpdateResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+address+path, bytes.NewReader(payload))
	if err != nil {
		return UpdateDropped
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return UpdateDropped
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return UpdateDropped
	}
	return UpdateDelivered
}

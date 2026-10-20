package server

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"time"
)

type UpdateResult string

const (
	UpdateDelivered UpdateResult = "delivered"
	UpdateSpooled   UpdateResult = "spooled"
	UpdateDropped   UpdateResult = "dropped"
)

func UnixHTTPClient(socket string, timeout time.Duration) *http.Client {
	transport := &http.Transport{DisableKeepAlives: true, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: timeout}).DialContext(ctx, "unix", socket)
	}}
	return &http.Client{Transport: transport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func PostTelemetry(ctx context.Context, socket, path string, payload []byte, timeout time.Duration, spoolDir string, spoolMaxBytes int64) UpdateResult {
	if result := PostUpdate(ctx, socket, path, payload, timeout); result == UpdateDelivered {
		return result
	}
	if err := AppendTelemetrySpool(spoolDir, payload, spoolMaxBytes); err != nil {
		return UpdateDropped
	}
	return UpdateSpooled
}

func PostUpdate(ctx context.Context, socket, path string, payload []byte, timeout time.Duration) UpdateResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix"+path, bytes.NewReader(payload))
	if err != nil {
		return UpdateDropped
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := UnixHTTPClient(socket, timeout).Do(req)
	if err != nil {
		return UpdateDropped
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return UpdateDropped
	}
	return UpdateDelivered
}

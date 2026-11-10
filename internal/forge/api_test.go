package forge

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Issue 1201: a request deadline, whether it hits before the response or mid-body, reports as a
// timeout that names the spec setting to raise, not as "decoding response" or "request failed".
func TestGetJSONTimeoutNamesSetting(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		midBody    bool
	}{
		{"mid body", "decoding response from test.org: timed out (raise spec/pr.yaml issue_scan.timeout)", true},
		{"before response", "request to test.org timed out (raise spec/pr.yaml issue_scan.timeout)", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			release := make(chan struct{})
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.midBody {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"number": 1`))
					w.(http.Flusher).Flush()
				}
				select { // Hold the response open until the client gives up.
				case <-release:
				case <-r.Context().Done():
				}
			}))
			defer ts.Close()
			defer close(release)

			client := &http.Client{Timeout: 50 * time.Millisecond}
			var v map[string]any
			err := GetJSON(client, Forge{Host: "test.org"}, ts.URL, "", "test request", "issue_scan.timeout", &v)
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("GetJSON error = %T %v; want *APIError", err, err)
			}
			if apiErr.Detail != tt.want || !apiErr.Transient {
				t.Fatalf("detail = %q transient=%v; want %q transient", apiErr.Detail, apiErr.Transient, tt.want)
			}
		})
	}
}

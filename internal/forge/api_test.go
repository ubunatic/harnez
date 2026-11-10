package forge

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetJSON_DeadlineMidBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			w.Write([]byte(`{"number": 1`))
			f.Flush()
		}
		time.Sleep(200 * time.Millisecond)
	}))
	defer ts.Close()

	client := &http.Client{Timeout: 50 * time.Millisecond}
	var v map[string]interface{}
	err := GetJSON(client, Forge{Host: "test.org"}, ts.URL, "", "test request", "test_setting", &v)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Detail != "decoding response from test.org: timed out (raise test_setting)" {
		t.Errorf("unexpected error detail: %q", apiErr.Detail)
	}
}

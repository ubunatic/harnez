package usage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/server"
)

func TestPassiveDropCapture(t *testing.T) {
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	t.Setenv(passiveDropCaptureEnv, "")
	path := filepath.Join(state, "harnez", "passive-last-dropped.json")
	capturePassiveDrop("agy", []byte(`{}`), "disabled")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("disabled capture: %v", err)
	}
	t.Setenv(passiveDropCaptureEnv, "1")
	for _, tc := range []struct{ payload, reason string }{
		{`{"quota":{}}`, "no usable quota windows"},
		{`{`, "decode passive usage"},
		{`{"quota":{"gemini-5h":{"remaining_fraction":0.296}}}`, "post windows"},
	} {
		got := ObserveStatuslineTo(context.Background(), "agy", []byte(tc.payload), filepath.Join(state, "missing.sock"), "/passive", time.Second)
		if got != server.UpdateDropped {
			t.Fatalf("result=%v", got)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var capture struct{ Provider, Reason, Payload string }
		if err := json.Unmarshal(data, &capture); err != nil {
			t.Fatal(err)
		}
		if capture.Provider != "agy" || capture.Payload != tc.payload || !strings.Contains(capture.Reason, tc.reason) {
			t.Fatalf("capture=%+v", capture)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("capture permissions: %v, %v", info, err)
		}
	}
}

func TestPassiveFailedWriteDoesNotCountAccepted(t *testing.T) {
	tracker := &PassiveDropTracker{}
	requests := make(chan PassiveRequest, 1)
	body := `{"windows":[{"provider":"agy","key":"gemini:5h","source":"statusline","observed_at":"2026-10-23T12:00:00Z","used_fraction":0.704}]}`
	// A buffered request plus cancelled context exercises the unavailable path
	// without a worker or timer delay.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	handler := PassiveUpdateServer{Drops: tracker, Requests: requests, Timeout: time.Second, MaxBodyBytes: 4096}
	recorder := httptest.NewRecorder()
	handler.handlePassive(recorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)).WithContext(ctx))
	if recorder.Code != http.StatusServiceUnavailable || tracker.AcceptedSnapshot()["agy"] != 0 {
		t.Fatalf("status=%d accepted=%v", recorder.Code, tracker.AcceptedSnapshot())
	}
}

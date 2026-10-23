package main

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/usage"
)

func TestAGYRenderedQuotaReachesPassiveCollector(t *testing.T) {
	const payload = `{"model":{"id":"gemini-3.1-pro"},"context_window":{"context_window_size":1048576,"current_usage":{"input_tokens":46000}},"quota":{"gemini-5h":{"remaining_fraction":0.296},"gemini-weekly":{"remaining_fraction":0.192},"gemini-argon-wtu-weekly":{"remaining_fraction":0.433}}}`
	tracker := &usage.PassiveDropTracker{}
	requests := make(chan usage.PassiveRequest)
	mux := http.NewServeMux()
	(usage.PassiveUpdateServer{Drops: tracker, Path: "/passive", Requests: requests, Timeout: time.Second, MaxBodyBytes: 8192}).RegisterRoutes(mux)
	listener, err := net.Listen("unix", filepath.Join(t.TempDir(), "server.sock"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(mux)
	srv.Listener.Close()
	srv.Listener = listener
	srv.Start()
	defer srv.Close()
	done := make(chan struct{})
	stop := make(chan struct{})
	defer func() { close(stop); <-done }()
	go func() {
		defer close(done)
		select {
		case request := <-requests:
			want := map[string]float64{"5h": .704, "weekly": .808, "argon-wtu-weekly": .567}
			var checkErr error
			if len(request.Windows) != len(want) {
				checkErr = fmt.Errorf("windows=%+v", request.Windows)
			}
			for _, window := range request.Windows {
				fraction, ok := want[window.Name]
				if !ok || math.Abs(window.UsedFraction-fraction) > 1e-12 || window.Provider != "agy" || window.Pool != "Gemini Models" || window.Source != "statusline" || window.Key == "" {
					checkErr = fmt.Errorf("unexpected window: %+v", window)
				}
				delete(want, window.Name)
			}
			if len(want) != 0 {
				checkErr = fmt.Errorf("missing quotas: %v", want)
			}
			if checkErr != nil {
				t.Error(checkErr)
			}
			request.Done <- checkErr
		case <-stop:
		}
	}()
	var result server.UpdateResult
	cmd := newStatuslineCmdWithObserver(func(ctx context.Context, provider string, data []byte) {
		if string(data) != payload {
			t.Error("stdin changed before observation")
		}
		result = usage.ObserveStatuslineTo(ctx, provider, data, listener.Addr().String(), "/passive", time.Second)
	})
	var out bytes.Buffer
	cmd.SetArgs([]string{"--agent", "agy"})
	cmd.SetIn(strings.NewReader(payload))
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"5h 29.6% left", "argon-wtu-weekly 43.3% left", "weekly 19.2% left"} {
		if !strings.Contains(out.String(), text) {
			t.Errorf("missing %q in %q", text, out.String())
		}
	}
	if result != server.UpdateDelivered || tracker.Snapshot()["agy"] != 0 || tracker.AcceptedSnapshot()["agy"] != 1 {
		t.Fatalf("result=%v dropped=%v accepted=%v", result, tracker.Snapshot(), tracker.AcceptedSnapshot())
	}
	snapshot := tracker.AcceptedSnapshot()
	snapshot["agy"] = 99
	if tracker.AcceptedSnapshot()["agy"] != 1 {
		t.Fatal("accepted snapshot aliases state")
	}
}

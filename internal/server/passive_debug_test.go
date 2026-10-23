package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

type dropCounterService struct{ waitingService }

func (dropCounterService) DroppedStatuslineObservations() map[string]uint64 {
	return map[string]uint64{"agy": 7, "claude": 2}
}

func (dropCounterService) AcceptedStatuslineObservations() map[string]uint64 {
	return map[string]uint64{"agy": 3, "claude": 1}
}

func TestStatusRouteReportsPassiveDropCounts(t *testing.T) {
	if testing.Short() {
		t.Skip("Unix HTTP server: full tier")
	}
	cfg := testConfig(t)
	socket, err := cfg.Socket()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	started := make(chan struct{})
	go func() {
		done <- (Server{Config: cfg, Version: "1.2.3", Services: []Service{dropCounterService{waitingService{started}}}}).Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	<-started
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := UnixHTTPClient(socket, cfg.HTTPClientTimeout).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Drops    map[string]uint64 `json:"passive_dropped"`
		Accepted map[string]uint64 `json:"passive_accepted"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Drops["agy"] != 7 || body.Drops["claude"] != 2 {
		t.Fatalf("counts=%v", body.Drops)
	}
	if body.Accepted["agy"] != 3 || body.Accepted["claude"] != 1 {
		t.Fatalf("accepted=%v", body.Accepted)
	}
}

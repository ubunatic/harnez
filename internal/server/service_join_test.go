package server

import (
	"context"
	"testing"
)

type joiningService struct {
	started, canceled, release chan struct{}
}

func (s joiningService) Run(ctx context.Context) error {
	close(s.started)
	<-ctx.Done()
	close(s.canceled)
	<-s.release
	return nil
}

func TestServerJoinsServicesBeforeReturning(t *testing.T) {
	cfg := testConfig(t)
	s := joiningService{make(chan struct{}), make(chan struct{}), make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- (Server{Config: cfg, Services: []Service{s}}).Run(ctx) }()
	<-s.started
	cancel()
	<-s.canceled
	select {
	case err := <-done:
		close(s.release)
		t.Fatalf("server returned before service joined: %v", err)
	default:
	}
	close(s.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

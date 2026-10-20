package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/telemetry"
	"ubunatic.com/harnez/internal/usage"
)

type usageCollectorService struct {
	interval     time.Duration
	client       *http.Client
	offline      bool
	ready        atomic.Bool
	requests     chan usage.PassiveRequest
	updateServer usage.PassiveUpdateServer
}

var serverBuildIdentity = server.CaptureBuildIdentity()

func (s *usageCollectorService) Run(ctx context.Context) error {
	return usage.RunCollectorWithRequests(ctx, "", s.client, s.interval, os.Stdout, s.offline, s.requests, func() { s.ready.Store(true) })
}

func (s *usageCollectorService) RegisterRoutes(mux *http.ServeMux) {
	s.updateServer.RegisterRoutes(mux)
}

func (s *usageCollectorService) Ready() bool { return s.ready.Load() }

func newServerCmd() *cobra.Command {
	return newServerCmdWithOptions(0, false)
}

func newServerCmdWithOptions(interval time.Duration, offline bool) *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Run Harnez local services",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runHarnezServer(ctx, interval, offline)
		},
	}
}

func runHarnezServer(ctx context.Context, interval time.Duration, offline bool) error {
	cfg, err := server.LoadConfig()
	if err != nil {
		return err
	}
	passivePath, err := usage.PassiveUpdatePath()
	if err != nil {
		return err
	}
	dbPath, err := telemetry.DefaultDBPath()
	if err != nil {
		return err
	}
	if interval <= 0 {
		interval = usage.DefaultCollectorInterval()
	}
	var client *http.Client
	if !offline {
		client = &http.Client{Timeout: cfg.HTTPClientTimeout}
	}
	requests := make(chan usage.PassiveRequest)
	collector := &usageCollectorService{interval: interval, client: client, offline: offline, requests: requests, updateServer: usage.PassiveUpdateServer{Path: passivePath, Requests: requests, Timeout: cfg.UpdateTimeout, MaxBodyBytes: cfg.UpdateMaxBodyBytes}}
	telemetryService := &telemetryCollectorService{config: cfg, path: dbPath, interval: interval}
	return (server.Server{Config: cfg, Version: harnez.Version, BuildIdentity: serverBuildIdentity, Services: []server.Service{collector, telemetryService}}).Run(ctx)
}

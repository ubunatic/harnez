package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/usage"
)

type usageCollectorService struct {
	interval time.Duration
	client   *http.Client
	offline  bool
}

func (s usageCollectorService) Run(ctx context.Context) error {
	return usage.RunCollector(ctx, "", s.client, s.interval, os.Stdout, s.offline)
}

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
	if interval <= 0 {
		interval = usage.DefaultCollectorInterval()
	}
	var client *http.Client
	if !offline {
		client = &http.Client{Timeout: duration(cfg.HTTPClientTimeout)}
	}
	return (server.Server{Config: cfg, Version: harnez.Version, Services: []server.Service{
		usageCollectorService{interval: interval, client: client, offline: offline},
	}}).Run(ctx)
}

func duration(value string) time.Duration {
	d, err := time.ParseDuration(value)
	if err != nil {
		panic(fmt.Sprintf("invalid embedded duration %q: %v", value, err))
	}
	return d
}

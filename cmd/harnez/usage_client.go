package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"

	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/usage"
)

type serverStarter struct {
	probe func(context.Context, server.Config) bool
	start func() error
	wait  func(context.Context, time.Duration) error
}

func ensureLocalServer(ctx context.Context, cfg server.Config) error {
	starter := serverStarter{
		probe: probeHarnezServer,
		start: startDetachedServer,
		wait: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
	return starter.ensure(ctx, cfg)
}

func loadLocalUsage(ctx context.Context, offline bool) (usage.UsageSummary, error) {
	if !offline {
		cfg, err := server.LoadConfig()
		if err != nil {
			return usage.UsageSummary{}, err
		}
		if err := ensureLocalServer(ctx, cfg); err != nil {
			return usage.UsageSummary{}, err
		}
	}
	summary, err := usage.ReadStoredSummary(ctx, usage.UsageStorePath())
	if err != nil && os.IsNotExist(err) && offline {
		return usage.UsageSummary{}, fmt.Errorf("usage store does not exist; start harnez server first (or run usage without --offline)")
	}
	if err == nil && !offline {
		summary.PassiveDropped = usage.ReadPassiveDropCounts(ctx)
	}
	return summary, err
}

func (s serverStarter) ensure(ctx context.Context, cfg server.Config) error {
	if err := s.start(); err != nil {
		return fmt.Errorf("start harnez server: %w", err)
	}
	deadline, cancel := context.WithTimeout(ctx, cfg.StartupTimeout)
	defer cancel()
	for {
		if s.probe(deadline, cfg) {
			return nil
		}
		if err := s.wait(deadline, cfg.StartupPoll); err != nil {
			return fmt.Errorf("harnez server did not become ready within %s: %w", cfg.StartupTimeout, err)
		}
	}
}

func probeHarnezServer(ctx context.Context, cfg server.Config) bool {
	socket, err := cfg.Socket()
	if err != nil {
		return false
	}
	client := server.UnixHTTPClient(socket, cfg.HTTPClientTimeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/status", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var status struct {
		Health        string `json:"health"`
		Server        string `json:"server"`
		Version       string `json:"version"`
		BuildIdentity string `json:"build_identity"`
	}
	return resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&status) == nil && status.Health == "ok" && status.Server == "harnez" && status.Version != "" && status.BuildIdentity != ""
}

func startDetachedServer() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(executable, "server")
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = devNull, devNull, devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

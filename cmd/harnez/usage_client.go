package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"syscall"

	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/usage"
)

type serverStarter struct {
	probe func(context.Context, server.Config) serverProbeResult
	start func() error
}

type serverProbeResult struct {
	ready          bool
	listening      bool
	starting       bool
	passiveNoQuota map[string]uint64
}

type serverCheckResult struct {
	status         string
	passiveNoQuota map[string]uint64
}

func ensureLocalServer(ctx context.Context, cfg server.Config) serverCheckResult {
	starter := serverStarter{
		probe: probeHarnezServer,
		start: startDetachedServer,
	}
	return starter.ensure(ctx, cfg)
}

func loadLocalUsage(ctx context.Context, offline bool) (usage.UsageSummary, error) {
	summary, _, err := loadLocalUsageWithStatus(ctx, offline)
	return summary, err
}

func loadLocalUsageWithStatus(ctx context.Context, offline bool) (usage.UsageSummary, string, error) {
	status := ""
	var passiveNoQuota map[string]uint64
	cfg, err := server.LoadConfig()
	if err != nil {
		return usage.UsageSummary{}, "", err
	}
	ctx, cancel := context.WithTimeout(ctx, cfg.UsageDeadline)
	defer cancel()
	if !offline {
		check := ensureLocalServer(ctx, cfg)
		status = check.status
		passiveNoQuota = check.passiveNoQuota
	}
	summary, err := readStoredSummaryBounded(ctx, usage.UsageStorePath())
	if err != nil && os.IsNotExist(err) && offline {
		return usage.UsageSummary{}, status, fmt.Errorf("usage store does not exist; start harnez server first (or run usage without --offline)")
	}
	if err == nil && !offline {
		summary.PassiveNoQuota = passiveNoQuota
	}
	return summary, status, err
}

type storedSummaryResult struct {
	summary usage.UsageSummary
	err     error
}

// readStoredSummaryBounded lets the caller honor its deadline even if the
// database driver blocks while opening a damaged or unusual store file.
func readStoredSummaryBounded(ctx context.Context, path string) (usage.UsageSummary, error) {
	return readStoredSummaryBoundedWith(ctx, func(ctx context.Context) (usage.UsageSummary, error) {
		return usage.ReadStoredSummary(ctx, path)
	})
}

func readStoredSummaryBoundedWith(ctx context.Context, read func(context.Context) (usage.UsageSummary, error)) (usage.UsageSummary, error) {
	result := make(chan storedSummaryResult, 1)
	go func() {
		summary, err := read(ctx)
		result <- storedSummaryResult{summary: summary, err: err}
	}()
	select {
	case got := <-result:
		return got.summary, got.err
	case <-ctx.Done():
		return usage.UsageSummary{}, ctx.Err()
	}
}

func (s serverStarter) ensure(ctx context.Context, cfg server.Config) serverCheckResult {
	probeCtx, cancel := context.WithTimeout(ctx, cfg.ProbeTimeout)
	result := s.probe(probeCtx, cfg)
	cancel()
	if result.ready {
		return serverCheckResult{passiveNoQuota: result.passiveNoQuota}
	}
	if !result.listening {
		if err := s.start(); err != nil {
			return serverCheckResult{status: "server not answering (could not start: " + err.Error() + ")"}
		}
		return serverCheckResult{status: "server starting"}
	}
	if result.starting {
		return serverCheckResult{status: "server starting", passiveNoQuota: result.passiveNoQuota}
	}
	return serverCheckResult{status: "server not answering", passiveNoQuota: result.passiveNoQuota}
}

func probeHarnezServer(ctx context.Context, cfg server.Config) serverProbeResult {
	socket, err := cfg.Socket()
	if err != nil {
		return serverProbeResult{listening: true}
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socket)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
			return serverProbeResult{}
		}
		// Unknown connection errors are not proof that the socket has no
		// listener. Avoid starting a second server or taking over the socket.
		return serverProbeResult{listening: true}
	}
	defer conn.Close()
	transport := &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
		return conn, nil
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: cfg.ProbeTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/status", nil)
	if err != nil {
		return serverProbeResult{listening: true}
	}
	resp, err := client.Do(req)
	if err != nil {
		return serverProbeResult{listening: true}
	}
	defer resp.Body.Close()
	var status struct {
		Health         string            `json:"health"`
		Server         string            `json:"server"`
		Version        string            `json:"version"`
		BuildIdentity  string            `json:"build_identity"`
		PassiveNoQuota map[string]uint64 `json:"passive_no_quota"`
	}
	decodeErr := json.NewDecoder(resp.Body).Decode(&status)
	if resp.StatusCode == http.StatusServiceUnavailable {
		return serverProbeResult{listening: true, starting: true, passiveNoQuota: status.PassiveNoQuota}
	}
	ready := resp.StatusCode == http.StatusOK && decodeErr == nil && status.Health == "ok" && status.Server == "harnez" && status.Version != "" && status.BuildIdentity != ""
	return serverProbeResult{ready: ready, listening: true, passiveNoQuota: status.PassiveNoQuota}
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

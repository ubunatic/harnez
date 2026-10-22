package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run Harnez local services",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runHarnezServer(ctx, interval, offline)
		},
	}
	cmd.AddCommand(newServerInstallCmd(), newServerUninstallCmd(), newServerStatusCmd())
	cmd.AddCommand(newServerStopCmd(), newServerRestartCmd())
	return cmd
}

func newServerInstallCmd() *cobra.Command {
	return &cobra.Command{Use: "install", Short: "Install and start the systemd user service", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := server.LoadConfig()
		if err != nil {
			return err
		}
		available, err := server.NewSystemdService(cfg).Install(cmd.Context())
		if err != nil {
			return err
		}
		if !available {
			fmt.Fprintln(cmd.OutOrStdout(), "systemd user service is unavailable; run `harnez usage -w` to keep the collector running")
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "installed and started %s\n", cfg.SystemdUnitName)
		return nil
	}}
}

func newServerUninstallCmd() *cobra.Command {
	return &cobra.Command{Use: "uninstall", Short: "Stop and remove the systemd user service", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := server.LoadConfig()
		if err != nil {
			return err
		}
		available, err := server.NewSystemdService(cfg).Uninstall(cmd.Context())
		if err != nil {
			return err
		}
		if !available {
			fmt.Fprintln(cmd.OutOrStdout(), "systemd user service is unavailable; run `harnez usage -w` to keep the collector running")
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "stopped and removed %s\n", cfg.SystemdUnitName)
		return nil
	}}
}

func newServerStatusCmd() *cobra.Command {
	return &cobra.Command{Use: "status", Short: "Show the local server and systemd service owner", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := server.LoadConfig()
		if err != nil {
			return err
		}
		socket, err := cfg.Socket()
		if err != nil {
			return err
		}
		socketPID := localServerPID(cmd.Context(), cfg, socket)
		status, err := server.NewSystemdService(cfg).Status(cmd.Context(), socketPID)
		if err != nil {
			return err
		}
		printServerSystemdStatus(cmd.OutOrStdout(), cfg, status)
		return nil
	}}
}

func newServerStopCmd() *cobra.Command {
	return &cobra.Command{Use: "stop", Short: "Stop the local server", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := server.LoadConfig()
		if err != nil {
			return err
		}
		socket, err := cfg.Socket()
		if err != nil {
			return err
		}
		socketPID := localServerPID(cmd.Context(), cfg, socket)
		status, err := server.NewSystemdService(cfg).Status(cmd.Context(), socketPID)
		if err != nil {
			return err
		}
		message, err := stopLocalServer(cmd.Context(), cfg, socket, socketPID, status, server.NewSystemdService(cfg).Stop)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), message)
		return nil
	}}
}

func newServerRestartCmd() *cobra.Command {
	return &cobra.Command{Use: "restart", Short: "Restart the local server", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := server.LoadConfig()
		if err != nil {
			return err
		}
		socket, err := cfg.Socket()
		if err != nil {
			return err
		}
		socketPID := localServerPID(cmd.Context(), cfg, socket)
		systemd := server.NewSystemdService(cfg)
		status, err := systemd.Status(cmd.Context(), socketPID)
		if err != nil {
			return err
		}
		message, err := restartLocalServer(cmd.Context(), cfg, socket, socketPID, status, systemd.Restart, systemd.Start, stopLocalServer)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), message)
		return nil
	}}
}

func restartLocalServer(ctx context.Context, cfg server.Config, socket string, socketPID int, status server.SystemdStatus, restartUnit func(context.Context) (bool, error), startUnit func(context.Context) (bool, error), stopServer func(context.Context, server.Config, string, int, server.SystemdStatus, func(context.Context) (bool, error)) (string, error)) (string, error) {
	if status.Available && status.Installed {
		if socketPID > 0 && status.MainPID == socketPID {
			available, err := restartUnit(ctx)
			if err != nil {
				return "", err
			}
			if !available {
				return "", fmt.Errorf("systemd user service became unavailable while restarting %s", cfg.SystemdUnitName)
			}
			return "restarted " + cfg.SystemdUnitName, nil
		}
		if socketPID == 0 {
			available, err := startUnit(ctx)
			if err != nil {
				return "", err
			}
			if !available {
				return "", fmt.Errorf("systemd user service became unavailable while starting %s", cfg.SystemdUnitName)
			}
			return "started " + cfg.SystemdUnitName, nil
		}
	}
	if socketPID == 0 {
		return "server not running; run 'harnez server' to start or 'harnez server install' to install systemd service", nil
	}
	// Foreground running server: stop it and report
	if stopServer != nil {
		if _, err := stopServer(ctx, cfg, socket, socketPID, status, nil); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("stopped foreground server (PID %d); no detached background runner configured, run 'harnez server' in a terminal or install systemd service with 'harnez server install'", socketPID), nil
}

func stopLocalServer(ctx context.Context, cfg server.Config, socket string, socketPID int, status server.SystemdStatus, stopUnit func(context.Context) (bool, error)) (string, error) {
	if socketPID == 0 {
		return "server not running", nil
	}
	if status.Available && status.Installed && status.MainPID == socketPID {
		available, err := stopUnit(ctx)
		if err != nil {
			return "", err
		}
		if !available {
			return "", fmt.Errorf("systemd user service became unavailable while stopping %s", cfg.SystemdUnitName)
		}
		return "stopped " + cfg.SystemdUnitName, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix/shutdown", nil)
	if err != nil {
		return "", err
	}
	response, err := server.UnixHTTPClient(socket, cfg.HTTPClientTimeout).Do(request)
	if err != nil {
		return "", fmt.Errorf("request server shutdown: %w", err)
	}
	response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("server shutdown returned %s", response.Status)
	}
	deadline := time.NewTimer(cfg.ShutdownTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(cfg.StartupPoll)
	defer ticker.Stop()
	for {
		probe, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/status", nil)
		if err != nil {
			return "", err
		}
		resp, err := server.UnixHTTPClient(socket, cfg.HTTPClientTimeout).Do(probe)
		if err != nil {
			return "shutdown requested", nil
		}
		resp.Body.Close()
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline.C:
			return "", fmt.Errorf("timed out waiting for server socket to stop answering")
		case <-ticker.C:
		}
	}
}

func printServerSystemdStatus(out io.Writer, cfg server.Config, status server.SystemdStatus) {
	if !status.Available {
		fmt.Fprintln(out, "systemd user service is unavailable; run `harnez usage -w` to keep the collector running")
		if status.SocketPID > 0 {
			fmt.Fprintf(out, "harnez server owns the socket (PID %d)\n", status.SocketPID)
		}
		return
	}
	fmt.Fprintf(out, "unit: %s (installed=%t, active=%t, MainPID=%d)\n", cfg.SystemdUnitName, status.Installed, status.Active, status.MainPID)
	if !status.Installed {
		fmt.Fprintln(out, "install the collector service with: harnez server install")
		if status.SocketPID > 0 {
			fmt.Fprintf(out, "socket owner: another process (PID %d)\n", status.SocketPID)
		} else {
			fmt.Fprintln(out, "socket owner: none")
		}
		return
	}
	if status.SocketPID == 0 {
		fmt.Fprintln(out, "socket owner: none")
		return
	}
	if status.SocketPID == status.MainPID {
		fmt.Fprintf(out, "socket owner: systemd service (PID %d)\n", status.SocketPID)
		return
	}
	fmt.Fprintf(out, "socket owner: another process (PID %d)\n", status.SocketPID)
	fmt.Fprintf(out, "hand back to systemd with: systemctl --user restart %s\n", cfg.SystemdUnitName)
}

func localServerPID(ctx context.Context, cfg server.Config, socket string) int {
	if pid, err := socketPeerPID(socket); err == nil {
		return pid
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/status", nil)
	if err != nil {
		return 0
	}
	response, err := server.UnixHTTPClient(socket, cfg.HTTPClientTimeout).Do(request)
	if err != nil {
		return 0
	}
	defer response.Body.Close()
	var status struct {
		Server string `json:"server"`
		PID    int    `json:"pid"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&status) != nil || status.Server != "harnez" {
		return 0
	}
	return status.PID
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
	return (server.Server{Config: cfg, Version: harnez.Version, BuildIdentity: serverBuildIdentity, Services: []server.Service{collector, telemetryService}, HandOver: server.NewSystemdService(cfg).HandOver}).Run(ctx)
}

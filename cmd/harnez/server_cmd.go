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

func printServerSystemdStatus(out io.Writer, cfg server.Config, status server.SystemdStatus) {
	if !status.Available {
		fmt.Fprintln(out, "systemd user service is unavailable; run `harnez usage -w` to keep the collector running")
		if status.SocketPID > 0 {
			fmt.Fprintf(out, "harnez server owns the socket (PID %d)\n", status.SocketPID)
		}
		return
	}
	fmt.Fprintf(out, "unit: %s (installed=%t, active=%t, MainPID=%d)\n", cfg.SystemdUnitName, status.Installed, status.Active, status.MainPID)
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
	return (server.Server{Config: cfg, Version: harnez.Version, BuildIdentity: serverBuildIdentity, Services: []server.Service{collector, telemetryService}}).Run(ctx)
}

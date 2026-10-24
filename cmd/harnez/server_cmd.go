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
	interval        time.Duration
	client          *http.Client
	offline         bool
	ready           atomic.Bool
	requests        chan usage.PassiveRequest
	updateServer    usage.PassiveUpdateServer
	tracker         *usage.ClientTracker
	heartbeatServer usage.HeartbeatServer
}

var serverBuildIdentity = server.CaptureBuildIdentity()

func (s *usageCollectorService) Run(ctx context.Context) error {
	return usage.RunCollectorWithRequestsAndTracker(ctx, "", s.client, s.interval, os.Stdout, s.offline, s.requests, func() { s.ready.Store(true) }, s.tracker)
}

func (s *usageCollectorService) RegisterRoutes(mux *http.ServeMux) {
	s.updateServer.RegisterRoutes(mux)
	s.heartbeatServer.RegisterRoutes(mux)
}

func (s *usageCollectorService) Ready() bool { return s.ready.Load() }

func (s *usageCollectorService) ReadyReason() string {
	if s.ready.Load() {
		return ""
	}
	return "usage collector is initializing"
}

func (s *usageCollectorService) NoQuotaStatuslineObservations() map[string]uint64 {
	return s.updateServer.NoQuota.NoQuotaSnapshot()
}

func (s *usageCollectorService) AcceptedStatuslineObservations() map[string]uint64 {
	return s.updateServer.NoQuota.AcceptedSnapshot()
}

func (s *usageCollectorService) ActiveClients() int {
	if s.tracker != nil {
		return s.tracker.Count()
	}
	return 0
}

func newServerCmd() *cobra.Command {
	return newServerCmdWithOptions(0, false)
}

func newServerCmdWithOptions(interval time.Duration, offline bool) *cobra.Command {
	indexConfig, indexErr := server.LoadNeusIndexConfig()
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run Harnez local services",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if indexErr != nil {
				return indexErr
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runHarnezServer(ctx, interval, offline, indexConfig)
		},
	}
	cmd.Flags().BoolVar(&indexConfig.Enabled, "neus-index", indexConfig.Enabled, "Enable periodic neus indexing of registered repositories")
	cmd.Flags().StringSliceVar(&indexConfig.Repos, "neus-repo", indexConfig.Repos, "Repository paths to index (repeat or comma-separate)")
	cmd.AddCommand(newServerInstallCmd(), newServerUninstallCmd(), newServerStatusCmd())
	cmd.AddCommand(newServerStopCmd(), newServerRestartCmd(), newServerLogsCmd())
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
	var jsonOutput bool
	cmd := &cobra.Command{Use: "status", Short: "Show the local server and systemd service owner", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := server.LoadConfig()
		if err != nil {
			return err
		}
		socket, err := cfg.Socket()
		if err != nil {
			return err
		}
		srvStatus := queryLocalServerStatus(cmd.Context(), cfg, socket)
		status, err := server.NewSystemdService(cfg).Status(cmd.Context(), srvStatus.PID)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printServerStatusJSON(cmd.OutOrStdout(), cfg, status, srvStatus)
		}
		printServerSystemdStatus(cmd.OutOrStdout(), cfg, status, srvStatus)
		return nil
	}}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output status as JSON")
	return cmd
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

func newServerLogsCmd() *cobra.Command {
	var follow bool
	var lines int
	cmd := &cobra.Command{Use: "logs", Short: "Show logs from the local server", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := server.LoadConfig()
		if err != nil {
			return err
		}
		if cmd.Flags().Changed("lines") && lines <= 0 {
			return fmt.Errorf("invalid line count: %d (must be > 0)", lines)
		}
		if lines <= 0 {
			lines = cfg.DefaultLogLines
		}
		systemd := server.NewSystemdService(cfg)
		status, err := systemd.Status(cmd.Context(), 0)
		if err != nil {
			return err
		}
		return runServerLogs(cmd.Context(), cmd.OutOrStdout(), cfg, status, follow, lines, systemd.Logs)
	}}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow log output")
	cmd.Flags().IntVarP(&lines, "lines", "n", 0, "number of lines to show (default from spec)")
	return cmd
}

func runServerLogs(ctx context.Context, out io.Writer, cfg server.Config, status server.SystemdStatus, follow bool, lines int, logsFn func(context.Context, bool, int) ([]byte, bool, error)) error {
	if status.Available && status.Installed {
		if logsFn != nil {
			data, available, err := logsFn(ctx, follow, lines)
			if err != nil {
				return err
			}
			if available {
				fmt.Fprint(out, string(data))
				return nil
			}
		}
	}
	fmt.Fprintln(out, "systemd user service is not installed or unavailable; foreground server logs are printed to its terminal standard output/error")
	return nil
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

type localServerStatusResponse struct {
	Server          string            `json:"server"`
	Version         string            `json:"version"`
	BuildIdentity   string            `json:"build_identity"`
	PID             int               `json:"pid"`
	StartedAt       time.Time         `json:"started_at"`
	Clients         *int              `json:"clients,omitempty"`
	PassiveNoQuota  map[string]uint64 `json:"passive_no_quota,omitempty"`
	PassiveAccepted map[string]uint64 `json:"passive_accepted,omitempty"`
}

type serverStatusJSON struct {
	Running         bool                  `json:"running"`
	Unit            *serverUnitStatusJSON `json:"unit,omitempty"`
	SocketOwner     string                `json:"socket_owner"`
	PID             int                   `json:"pid"`
	Version         string                `json:"version,omitempty"`
	CLIVersion      string                `json:"cli_version,omitempty"`
	UptimeSeconds   *int64                `json:"uptime_seconds,omitempty"`
	Clients         *int                  `json:"clients,omitempty"`
	PassiveNoQuota  map[string]uint64     `json:"passive_no_quota,omitempty"`
	PassiveAccepted map[string]uint64     `json:"passive_accepted,omitempty"`
	VersionMismatch bool                  `json:"version_mismatch"`
}

type serverUnitStatusJSON struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Installed bool   `json:"installed"`
	Active    bool   `json:"active"`
	MainPID   int    `json:"main_pid"`
}

func printServerStatusJSON(out io.Writer, cfg server.Config, status server.SystemdStatus, srv localServerStatusResponse) error {
	owner := "none"
	if status.SocketPID > 0 {
		if status.Available && status.Installed && status.SocketPID == status.MainPID {
			owner = "systemd"
		} else {
			owner = "another process"
		}
	}
	sj := serverStatusJSON{
		Running:         srv.PID > 0,
		SocketOwner:     owner,
		PID:             srv.PID,
		CLIVersion:      harnez.Version,
		VersionMismatch: srv.PID > 0 && srv.Version != "" && srv.Version != harnez.Version,
	}
	if status.Available || status.Installed {
		sj.Unit = &serverUnitStatusJSON{
			Name:      cfg.SystemdUnitName,
			Available: status.Available,
			Installed: status.Installed,
			Active:    status.Active,
			MainPID:   status.MainPID,
		}
	}
	if srv.PID > 0 {
		sj.Version = srv.Version
		sj.Clients = srv.Clients
		sj.PassiveNoQuota = srv.PassiveNoQuota
		sj.PassiveAccepted = srv.PassiveAccepted
		if !srv.StartedAt.IsZero() {
			secs := int64(time.Since(srv.StartedAt).Seconds())
			if secs < 0 {
				secs = 0
			}
			sj.UptimeSeconds = &secs
		}
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(sj)
}

func printServerSystemdStatus(out io.Writer, cfg server.Config, status server.SystemdStatus, srv localServerStatusResponse) {
	if !status.Available {
		fmt.Fprintln(out, "systemd user service is unavailable; run `harnez usage -w` to keep the collector running")
		if status.SocketPID > 0 {
			fmt.Fprintf(out, "harnez server owns the socket (PID %d)\n", status.SocketPID)
			printServerUptimeAndVersion(out, srv)
		}
		return
	}
	fmt.Fprintf(out, "unit: %s (installed=%t, active=%t, MainPID=%d)\n", cfg.SystemdUnitName, status.Installed, status.Active, status.MainPID)
	if !status.Installed {
		fmt.Fprintln(out, "install the collector service with: harnez server install")
		if status.SocketPID > 0 {
			fmt.Fprintf(out, "socket owner: another process (PID %d)\n", status.SocketPID)
			printServerUptimeAndVersion(out, srv)
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
	} else {
		fmt.Fprintf(out, "socket owner: another process (PID %d)\n", status.SocketPID)
		fmt.Fprintf(out, "hand back to systemd with: systemctl --user restart %s\n", cfg.SystemdUnitName)
	}
	printServerUptimeAndVersion(out, srv)
}

func printServerUptimeAndVersion(out io.Writer, srv localServerStatusResponse) {
	if srv.PID <= 0 {
		return
	}
	if srv.StartedAt.IsZero() {
		fmt.Fprintln(out, "uptime: unknown")
	} else {
		uptime := time.Since(srv.StartedAt).Truncate(time.Second)
		if uptime < 0 {
			uptime = 0
		}
		fmt.Fprintf(out, "uptime: %s\n", uptime)
	}
	if srv.Clients != nil {
		fmt.Fprintf(out, "clients: %d\n", *srv.Clients)
	}
	if srv.Version != "" {
		if srv.Version != harnez.Version {
			fmt.Fprintf(out, "version: %s (restart to update to %s)\n", srv.Version, harnez.Version)
		} else {
			fmt.Fprintf(out, "version: %s\n", srv.Version)
		}
	}
}

func queryLocalServerStatus(ctx context.Context, cfg server.Config, socket string) localServerStatusResponse {
	var resp localServerStatusResponse
	if pid, err := socketPeerPID(socket); err == nil && pid > 0 {
		resp.PID = pid
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/status", nil)
	if err != nil {
		return resp
	}
	response, err := server.UnixHTTPClient(socket, cfg.HTTPClientTimeout).Do(request)
	if err != nil {
		return resp
	}
	defer response.Body.Close()
	if (response.StatusCode != http.StatusOK && response.StatusCode != http.StatusServiceUnavailable) || json.NewDecoder(response.Body).Decode(&resp) != nil || resp.Server != "harnez" {
		return resp
	}
	return resp
}

func localServerPID(ctx context.Context, cfg server.Config, socket string) int {
	return queryLocalServerStatus(ctx, cfg, socket).PID
}

func runHarnezServer(ctx context.Context, interval time.Duration, offline bool, indexing ...server.NeusIndexConfig) error {
	cfg, err := server.LoadConfig()
	if err != nil {
		return err
	}
	indexConfig, err := server.LoadNeusIndexConfig()
	if err != nil {
		return err
	}
	if len(indexing) > 0 {
		indexConfig = indexing[0]
	}
	if err := indexConfig.Validate(); err != nil {
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
	_, clientTimeout, _, err := usage.ClientHeartbeatPolicy()
	if err != nil {
		return err
	}
	tracker := usage.NewClientTracker(clientTimeout)
	heartbeatServer := usage.HeartbeatServer{
		Path:    "/usage/heartbeat",
		Tracker: tracker,
		Timeout: cfg.UpdateTimeout,
	}
	collector := &usageCollectorService{
		interval:        interval,
		client:          client,
		offline:         offline,
		requests:        requests,
		updateServer:    usage.PassiveUpdateServer{NoQuota: &usage.PassiveNoQuotaTracker{}, Path: passivePath, Requests: requests, Timeout: cfg.UpdateTimeout, MaxBodyBytes: cfg.UpdateMaxBodyBytes},
		tracker:         tracker,
		heartbeatServer: heartbeatServer,
	}
	telemetryService := &telemetryCollectorService{config: cfg, path: dbPath, interval: interval}
	services := []server.Service{collector, telemetryService}
	if indexConfig.Enabled {
		services = append(services, &server.NeusIndexer{Config: indexConfig})
	}
	return (server.Server{Config: cfg, Version: harnez.Version, BuildIdentity: serverBuildIdentity, Services: services, HandOver: server.NewSystemdService(cfg).HandOver}).Run(ctx)
}

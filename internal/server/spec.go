package server

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/sshutil"
)

type Config struct {
	SocketPath                string
	ReadHeaderTimeout         time.Duration
	ShutdownTimeout           time.Duration
	HTTPClientTimeout         time.Duration
	ProbeTimeout              time.Duration
	UsageDeadline             time.Duration
	UpdateTimeout             time.Duration
	UpdateMaxBodyBytes        int64
	TelemetryPath             string
	UsageQueryPath            string
	RegistryPath              string
	TelemetrySpoolPath        string
	TelemetrySpoolMaxBytes    int64
	StartupTimeout            time.Duration
	StartupPoll               time.Duration
	TakeoverTimeout           time.Duration
	SystemdUnitName           string
	SystemdUnitSource         string
	SystemdRestartPolicy      string
	SystemdRestartDelay       time.Duration
	TelemetryWarningRateLimit time.Duration
	DefaultLogLines           int
	RemoteUpgrade             RemoteUpgradeConfig
	RunnerChannel             RunnerChannelConfig
}

// RunnerChannelConfig controls the inherited launch-bound runner broker FD.
type RunnerChannelConfig struct {
	FDEnv                   string
	MaxFrameBytes           int64
	FrameHeadroomBytes      int64
	SpoolMaxBytes           int64
	ReplayMaxRecords        int
	ReplayMaxBytes          int64
	MaxConcurrentStreams    int
	UsageQueryMaxModels     int
	UsageQueryMaxStringLen  int
	RegistryMaxIDLen        int
	AllowedTelemetryKinds   []string
	AllowedUsageActions     []string
	AllowedRegistryStatuses []string
}

// RemoteUpgradeConfig controls best-effort replacement of an older remote binary.
type RemoteUpgradeConfig struct {
	InstallPath     string
	ConnectTimeout  time.Duration
	CommandTimeout  time.Duration
	TransferTimeout time.Duration
}

type specConfig struct {
	SocketPath                string `yaml:"socket_path"`
	ReadHeaderTimeout         string `yaml:"read_header_timeout"`
	ShutdownTimeout           string `yaml:"shutdown_timeout"`
	HTTPClientTimeout         string `yaml:"http_client_timeout"`
	ProbeTimeout              string `yaml:"probe_timeout"`
	UsageDeadline             string `yaml:"usage_deadline"`
	UpdateTimeout             string `yaml:"update_timeout"`
	UpdateMaxBodyBytes        int64  `yaml:"update_max_body_bytes"`
	TelemetryPath             string `yaml:"telemetry_update_path"`
	UsageQueryPath            string `yaml:"usage_query_path"`
	RegistryPath              string `yaml:"registry_path"`
	TelemetrySpoolPath        string `yaml:"telemetry_spool_path"`
	TelemetrySpoolMaxBytes    int64  `yaml:"telemetry_spool_max_bytes"`
	StartupTimeout            string `yaml:"startup_timeout"`
	StartupPoll               string `yaml:"startup_poll_interval"`
	TakeoverTimeout           string `yaml:"takeover_timeout"`
	SystemdUnitName           string `yaml:"systemd_unit_name"`
	SystemdUnitSource         string `yaml:"systemd_unit_source"`
	SystemdRestartPolicy      string `yaml:"systemd_restart_policy"`
	SystemdRestartDelay       string `yaml:"systemd_restart_delay"`
	TelemetryWarningRateLimit string `yaml:"telemetry_warning_rate_limit"`
	DefaultLogLines           int    `yaml:"default_log_lines"`
	RemoteUpgrade             struct {
		InstallPath     string `yaml:"install_path"`
		ConnectTimeout  string `yaml:"connect_timeout"`
		CommandTimeout  string `yaml:"command_timeout"`
		TransferTimeout string `yaml:"transfer_timeout"`
	} `yaml:"remote_upgrade"`
	RunnerChannel rawRunnerChannel `yaml:"runner_channel"`
}

type rawRunnerChannel struct {
	FDEnv                   string   `yaml:"fd_env"`
	MaxFrameBytes           int64    `yaml:"max_frame_bytes"`
	FrameHeadroomBytes      int64    `yaml:"frame_headroom_bytes"`
	SpoolMaxBytes           int64    `yaml:"spool_max_bytes"`
	ReplayMaxRecords        int      `yaml:"replay_max_records"`
	ReplayMaxBytes          int64    `yaml:"replay_max_bytes"`
	MaxConcurrentStreams    int      `yaml:"max_concurrent_streams"`
	UsageQueryMaxModels     int      `yaml:"usage_query_max_models"`
	UsageQueryMaxStringLen  int      `yaml:"usage_query_max_string_len"`
	RegistryMaxIDLen        int      `yaml:"registry_max_id_len"`
	AllowedTelemetryKinds   []string `yaml:"allowed_telemetry_kinds"`
	AllowedUsageActions     []string `yaml:"allowed_usage_actions"`
	AllowedRegistryStatuses []string `yaml:"allowed_registry_statuses"`
}

func LoadConfig() (Config, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/server.yaml")
	if err != nil {
		return Config{}, err
	}
	var raw specConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Config{}, err
	}
	if raw.SocketPath == "" || filepath.IsAbs(raw.SocketPath) || strings.Contains(raw.SocketPath, "..") || filepath.Base(raw.SocketPath) != raw.SocketPath {
		return Config{}, fmt.Errorf("spec/server.yaml: socket_path must be a filename")
	}
	cfg := Config{SocketPath: raw.SocketPath, UpdateMaxBodyBytes: raw.UpdateMaxBodyBytes, TelemetryPath: raw.TelemetryPath, UsageQueryPath: raw.UsageQueryPath, RegistryPath: raw.RegistryPath, TelemetrySpoolPath: raw.TelemetrySpoolPath, TelemetrySpoolMaxBytes: raw.TelemetrySpoolMaxBytes, SystemdUnitName: raw.SystemdUnitName, SystemdUnitSource: raw.SystemdUnitSource, SystemdRestartPolicy: raw.SystemdRestartPolicy, DefaultLogLines: raw.DefaultLogLines}
	if !sshutil.ValidInstallPath(raw.RemoteUpgrade.InstallPath) {
		return Config{}, fmt.Errorf("spec/server.yaml: remote_upgrade.install_path must be a safe home-relative or absolute path outside temporary directories")
	}
	cfg.RemoteUpgrade.InstallPath = raw.RemoteUpgrade.InstallPath
	for _, item := range []struct {
		name   string
		value  string
		target *time.Duration
	}{
		{"connect_timeout", raw.RemoteUpgrade.ConnectTimeout, &cfg.RemoteUpgrade.ConnectTimeout},
		{"command_timeout", raw.RemoteUpgrade.CommandTimeout, &cfg.RemoteUpgrade.CommandTimeout},
		{"transfer_timeout", raw.RemoteUpgrade.TransferTimeout, &cfg.RemoteUpgrade.TransferTimeout},
	} {
		d, err := time.ParseDuration(item.value)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("spec/server.yaml: remote_upgrade.%s must be a positive duration", item.name)
		}
		*item.target = d
	}
	if cfg.UpdateMaxBodyBytes <= 0 {
		return Config{}, fmt.Errorf("spec/server.yaml: update_max_body_bytes must be positive")
	}
	if cfg.DefaultLogLines <= 0 {
		return Config{}, fmt.Errorf("spec/server.yaml: default_log_lines must be positive")
	}
	if !strings.HasPrefix(cfg.TelemetryPath, "/") || strings.ContainsAny(cfg.TelemetryPath, "?#") {
		return Config{}, fmt.Errorf("spec/server.yaml: telemetry_update_path must be an absolute route path")
	}
	if !strings.HasPrefix(cfg.UsageQueryPath, "/") || strings.ContainsAny(cfg.UsageQueryPath, "?#") {
		return Config{}, fmt.Errorf("spec/server.yaml: usage_query_path must be an absolute route path")
	}
	if !strings.HasPrefix(cfg.RegistryPath, "/") || strings.ContainsAny(cfg.RegistryPath, "?#") {
		return Config{}, fmt.Errorf("spec/server.yaml: registry_path must be an absolute route path")
	}
	if cfg.TelemetrySpoolPath == "" || filepath.IsAbs(cfg.TelemetrySpoolPath) || strings.Contains(cfg.TelemetrySpoolPath, "..") || filepath.Base(cfg.TelemetrySpoolPath) != cfg.TelemetrySpoolPath {
		return Config{}, fmt.Errorf("spec/server.yaml: telemetry_spool_path must be a directory name")
	}
	if cfg.TelemetrySpoolMaxBytes <= 0 {
		return Config{}, fmt.Errorf("spec/server.yaml: telemetry_spool_max_bytes must be positive")
	}
	if err := loadRunnerChannel(&cfg, raw.RunnerChannel); err != nil {
		return Config{}, err
	}
	if !strings.HasSuffix(cfg.SystemdUnitName, ".service") || filepath.Base(cfg.SystemdUnitName) != cfg.SystemdUnitName {
		return Config{}, fmt.Errorf("spec/server.yaml: systemd_unit_name must be a service filename")
	}
	if !strings.HasPrefix(cfg.SystemdUnitSource, "systemd/") || strings.Contains(cfg.SystemdUnitSource, "..") {
		return Config{}, fmt.Errorf("spec/server.yaml: systemd_unit_source must be inside systemd/")
	}
	if cfg.SystemdRestartPolicy != "on-failure" {
		return Config{}, fmt.Errorf("spec/server.yaml: systemd_restart_policy must be on-failure")
	}
	for _, item := range []struct{ name, value string }{
		{"read_header_timeout", raw.ReadHeaderTimeout}, {"shutdown_timeout", raw.ShutdownTimeout}, {"http_client_timeout", raw.HTTPClientTimeout}, {"probe_timeout", raw.ProbeTimeout}, {"usage_deadline", raw.UsageDeadline},
		{"update_timeout", raw.UpdateTimeout}, {"startup_timeout", raw.StartupTimeout}, {"startup_poll_interval", raw.StartupPoll}, {"takeover_timeout", raw.TakeoverTimeout}, {"systemd_restart_delay", raw.SystemdRestartDelay}, {"telemetry_warning_rate_limit", raw.TelemetryWarningRateLimit},
	} {
		d, err := time.ParseDuration(item.value)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("spec/server.yaml: %s must be a positive duration", item.name)
		}
		switch item.name {
		case "read_header_timeout":
			cfg.ReadHeaderTimeout = d
		case "shutdown_timeout":
			cfg.ShutdownTimeout = d
		case "http_client_timeout":
			cfg.HTTPClientTimeout = d
		case "probe_timeout":
			cfg.ProbeTimeout = d
		case "usage_deadline":
			cfg.UsageDeadline = d
		case "update_timeout":
			cfg.UpdateTimeout = d
		case "startup_timeout":
			cfg.StartupTimeout = d
		case "startup_poll_interval":
			cfg.StartupPoll = d
		case "takeover_timeout":
			cfg.TakeoverTimeout = d
		case "systemd_restart_delay":
			cfg.SystemdRestartDelay = d
		case "telemetry_warning_rate_limit":
			cfg.TelemetryWarningRateLimit = d
		}
	}
	return cfg, nil
}

func (c Config) TelemetrySpoolDir() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(base) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "harnez", c.TelemetrySpoolPath), nil
}

func (c Config) Socket() (string, error) {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if !filepath.IsAbs(base) {
		base = os.Getenv("XDG_STATE_HOME")
		if !filepath.IsAbs(base) {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "state")
		}
	}
	dir := filepath.Join(base, "harnez")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, c.SocketPath)
	if len(path) >= 108 {
		return "", fmt.Errorf("Unix socket path exceeds 107 bytes: %s", path)
	}
	return path, nil
}

func loadRunnerChannel(cfg *Config, raw rawRunnerChannel) error {
	if raw.FDEnv == "" || raw.MaxFrameBytes <= 0 || raw.FrameHeadroomBytes <= 0 || raw.SpoolMaxBytes <= 0 || raw.ReplayMaxRecords <= 0 || raw.ReplayMaxBytes <= 0 || raw.MaxConcurrentStreams <= 0 || raw.UsageQueryMaxModels <= 0 || raw.UsageQueryMaxStringLen <= 0 || raw.RegistryMaxIDLen <= 0 {
		return fmt.Errorf("spec/server.yaml: runner_channel settings must be positive and non-empty")
	}
	if raw.MaxFrameBytes <= raw.ReplayMaxBytes || raw.MaxFrameBytes <= raw.FrameHeadroomBytes {
		return fmt.Errorf("spec/server.yaml: runner_channel.max_frame_bytes must exceed replay_max_bytes and frame_headroom_bytes")
	}
	if len(raw.AllowedTelemetryKinds) == 0 {
		return fmt.Errorf("spec/server.yaml: runner_channel.allowed_telemetry_kinds must be non-empty")
	}
	if len(raw.AllowedUsageActions) == 0 {
		return fmt.Errorf("spec/server.yaml: runner_channel.allowed_usage_actions must be non-empty")
	}
	if len(raw.AllowedRegistryStatuses) == 0 {
		return fmt.Errorf("spec/server.yaml: runner_channel.allowed_registry_statuses must be non-empty")
	}
	cfg.RunnerChannel = RunnerChannelConfig{
		FDEnv:                   raw.FDEnv,
		MaxFrameBytes:           raw.MaxFrameBytes,
		FrameHeadroomBytes:      raw.FrameHeadroomBytes,
		SpoolMaxBytes:           raw.SpoolMaxBytes,
		ReplayMaxRecords:        raw.ReplayMaxRecords,
		ReplayMaxBytes:          raw.ReplayMaxBytes,
		MaxConcurrentStreams:    raw.MaxConcurrentStreams,
		UsageQueryMaxModels:     raw.UsageQueryMaxModels,
		UsageQueryMaxStringLen:  raw.UsageQueryMaxStringLen,
		RegistryMaxIDLen:        raw.RegistryMaxIDLen,
		AllowedTelemetryKinds:   append([]string(nil), raw.AllowedTelemetryKinds...),
		AllowedUsageActions:     append([]string(nil), raw.AllowedUsageActions...),
		AllowedRegistryStatuses: append([]string(nil), raw.AllowedRegistryStatuses...),
	}
	return nil
}

// MaxResultPayloadBytes returns the maximum serialized result size allowed in
// a runner channel response frame.
func (c RunnerChannelConfig) MaxResultPayloadBytes() int64 {
	if c.MaxFrameBytes <= c.FrameHeadroomBytes {
		return 0
	}
	return c.MaxFrameBytes - c.FrameHeadroomBytes
}

// AllowsTelemetryKind reports whether a runner may submit the given record kind.
func (c RunnerChannelConfig) AllowsTelemetryKind(kind string) bool {
	return kind != "" && slices.Contains(c.AllowedTelemetryKinds, kind)
}

// AllowsUsageAction reports whether a runner may execute the given usage query action.
func (c RunnerChannelConfig) AllowsUsageAction(action string) bool {
	return action != "" && slices.Contains(c.AllowedUsageActions, action)
}

// AllowsRegistryStatus reports whether a runner may report the given session status.
func (c RunnerChannelConfig) AllowsRegistryStatus(status string) bool {
	return status != "" && slices.Contains(c.AllowedRegistryStatuses, status)
}

var loadLeafAgentRoles = sync.OnceValues(func() (map[string]bool, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/agent.yaml")
	if err != nil {
		return nil, err
	}
	var raw struct {
		Roles map[string]struct {
			Aliases []string `yaml:"aliases"`
			Spawns  []string `yaml:"spawns"`
		} `yaml:"roles"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := make(map[string]bool)
	for name, role := range raw.Roles {
		if len(role.Spawns) > 0 {
			continue
		}
		if key := strings.ToLower(strings.TrimSpace(name)); key != "" {
			out[key] = true
		}
		for _, alias := range role.Aliases {
			if key := strings.ToLower(strings.TrimSpace(alias)); key != "" {
				out[key] = true
			}
		}
	}
	return out, nil
})

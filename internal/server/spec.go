package server

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

type Config struct {
	SocketPath             string
	ReadHeaderTimeout      time.Duration
	ShutdownTimeout        time.Duration
	HTTPClientTimeout      time.Duration
	UpdateTimeout          time.Duration
	UpdateMaxBodyBytes     int64
	TelemetryPath          string
	TelemetrySpoolPath     string
	TelemetrySpoolMaxBytes int64
	StartupTimeout         time.Duration
	StartupPoll            time.Duration
	TakeoverTimeout        time.Duration
}

type specConfig struct {
	SocketPath             string `yaml:"socket_path"`
	ReadHeaderTimeout      string `yaml:"read_header_timeout"`
	ShutdownTimeout        string `yaml:"shutdown_timeout"`
	HTTPClientTimeout      string `yaml:"http_client_timeout"`
	UpdateTimeout          string `yaml:"update_timeout"`
	UpdateMaxBodyBytes     int64  `yaml:"update_max_body_bytes"`
	TelemetryPath          string `yaml:"telemetry_update_path"`
	TelemetrySpoolPath     string `yaml:"telemetry_spool_path"`
	TelemetrySpoolMaxBytes int64  `yaml:"telemetry_spool_max_bytes"`
	StartupTimeout         string `yaml:"startup_timeout"`
	StartupPoll            string `yaml:"startup_poll_interval"`
	TakeoverTimeout        string `yaml:"takeover_timeout"`
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
	cfg := Config{SocketPath: raw.SocketPath, UpdateMaxBodyBytes: raw.UpdateMaxBodyBytes, TelemetryPath: raw.TelemetryPath, TelemetrySpoolPath: raw.TelemetrySpoolPath, TelemetrySpoolMaxBytes: raw.TelemetrySpoolMaxBytes}
	if cfg.UpdateMaxBodyBytes <= 0 {
		return Config{}, fmt.Errorf("spec/server.yaml: update_max_body_bytes must be positive")
	}
	if !strings.HasPrefix(cfg.TelemetryPath, "/") || strings.ContainsAny(cfg.TelemetryPath, "?#") {
		return Config{}, fmt.Errorf("spec/server.yaml: telemetry_update_path must be an absolute route path")
	}
	if cfg.TelemetrySpoolPath == "" || filepath.IsAbs(cfg.TelemetrySpoolPath) || strings.Contains(cfg.TelemetrySpoolPath, "..") || filepath.Base(cfg.TelemetrySpoolPath) != cfg.TelemetrySpoolPath {
		return Config{}, fmt.Errorf("spec/server.yaml: telemetry_spool_path must be a directory name")
	}
	if cfg.TelemetrySpoolMaxBytes <= 0 {
		return Config{}, fmt.Errorf("spec/server.yaml: telemetry_spool_max_bytes must be positive")
	}
	for _, item := range []struct{ name, value string }{
		{"read_header_timeout", raw.ReadHeaderTimeout}, {"shutdown_timeout", raw.ShutdownTimeout}, {"http_client_timeout", raw.HTTPClientTimeout},
		{"update_timeout", raw.UpdateTimeout}, {"startup_timeout", raw.StartupTimeout}, {"startup_poll_interval", raw.StartupPoll}, {"takeover_timeout", raw.TakeoverTimeout},
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
		case "update_timeout":
			cfg.UpdateTimeout = d
		case "startup_timeout":
			cfg.StartupTimeout = d
		case "startup_poll_interval":
			cfg.StartupPoll = d
		case "takeover_timeout":
			cfg.TakeoverTimeout = d
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

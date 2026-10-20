package server

import (
	"fmt"
	"io/fs"
	"net"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

type Config struct {
	Host               string        `yaml:"host"`
	Port               int           `yaml:"port"`
	ReadHeaderTimeout  time.Duration `yaml:"-"`
	ShutdownTimeout    time.Duration `yaml:"-"`
	HTTPClientTimeout  time.Duration `yaml:"-"`
	UpdateTimeout      time.Duration `yaml:"-"`
	UpdateMaxBodyBytes int64         `yaml:"-"`
	StartupTimeout     time.Duration `yaml:"-"`
	StartupPoll        time.Duration `yaml:"-"`
}

type specConfig struct {
	Host               string `yaml:"host"`
	Port               int    `yaml:"port"`
	ReadHeaderTimeout  string `yaml:"read_header_timeout"`
	ShutdownTimeout    string `yaml:"shutdown_timeout"`
	HTTPClientTimeout  string `yaml:"http_client_timeout"`
	UpdateTimeout      string `yaml:"update_timeout"`
	UpdateMaxBodyBytes int64  `yaml:"update_max_body_bytes"`
	StartupTimeout     string `yaml:"startup_timeout"`
	StartupPoll        string `yaml:"startup_poll_interval"`
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
	if raw.Host != "127.0.0.1" || raw.Port < 1 || raw.Port > 65535 {
		return Config{}, fmt.Errorf("spec/server.yaml: host must be 127.0.0.1 and port must be 1..65535")
	}
	cfg := Config{Host: raw.Host, Port: raw.Port}
	cfg.UpdateMaxBodyBytes = raw.UpdateMaxBodyBytes
	if cfg.UpdateMaxBodyBytes <= 0 {
		return Config{}, fmt.Errorf("spec/server.yaml: update_max_body_bytes must be positive")
	}
	for _, item := range []struct{ name, value string }{
		{"read_header_timeout", raw.ReadHeaderTimeout}, {"shutdown_timeout", raw.ShutdownTimeout}, {"http_client_timeout", raw.HTTPClientTimeout},
		{"update_timeout", raw.UpdateTimeout}, {"startup_timeout", raw.StartupTimeout}, {"startup_poll_interval", raw.StartupPoll},
	} {
		name, value := item.name, item.value
		d, err := time.ParseDuration(value)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("spec/server.yaml: %s must be a positive duration", name)
		}
		switch name {
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
		}
	}
	return cfg, nil
}

func (c Config) Address() string { return net.JoinHostPort(c.Host, fmt.Sprint(c.Port)) }

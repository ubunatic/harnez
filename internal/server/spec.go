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
	Host              string `yaml:"host"`
	Port              int    `yaml:"port"`
	ReadHeaderTimeout string `yaml:"read_header_timeout"`
	ShutdownTimeout   string `yaml:"shutdown_timeout"`
	HTTPClientTimeout string `yaml:"http_client_timeout"`
}

func LoadConfig() (Config, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/server.yaml")
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Host != "127.0.0.1" || cfg.Port < 1 || cfg.Port > 65535 {
		return Config{}, fmt.Errorf("spec/server.yaml: host must be 127.0.0.1 and port must be 1..65535")
	}
	for name, value := range map[string]string{"read_header_timeout": cfg.ReadHeaderTimeout, "shutdown_timeout": cfg.ShutdownTimeout, "http_client_timeout": cfg.HTTPClientTimeout} {
		d, err := time.ParseDuration(value)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("spec/server.yaml: %s must be a positive duration", name)
		}
	}
	return cfg, nil
}

func (c Config) Address() string { return net.JoinHostPort(c.Host, fmt.Sprint(c.Port)) }

func duration(value string) time.Duration { d, _ := time.ParseDuration(value); return d }

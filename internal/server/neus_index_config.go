package server

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// NeusIndexConfig describes opt-in indexing of explicitly registered roots.
type NeusIndexConfig struct {
	Enabled     bool     `yaml:"enabled"`
	Interval    string   `yaml:"interval"`
	Timeout     string   `yaml:"timeout"`
	Command     []string `yaml:"command"`
	ServiceName string   `yaml:"service_name"`
	Repos       []string `yaml:"repos"`
}

// LoadNeusIndexConfig reads indexing defaults from the embedded server spec.
func LoadNeusIndexConfig() (NeusIndexConfig, error) {
	data, err := harnez.DefaultFS.ReadFile("spec/server.yaml")
	if err != nil {
		return NeusIndexConfig{}, err
	}
	var raw struct {
		Index NeusIndexConfig `yaml:"neus_indexing"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return NeusIndexConfig{}, err
	}
	return raw.Index, raw.Index.Validate()
}

// Validate checks the spec and user-supplied root registration before startup.
func (c NeusIndexConfig) Validate() error {
	for _, value := range []string{c.Interval, c.Timeout} {
		if d, err := time.ParseDuration(value); err != nil || d <= 0 {
			return fmt.Errorf("neus indexing: invalid duration %q", value)
		}
	}
	if len(c.Command) == 0 || c.Command[0] == "" || c.ServiceName == "" {
		return fmt.Errorf("neus indexing: missing command or service name")
	}
	if c.Enabled && len(c.Repos) == 0 {
		return fmt.Errorf("neus indexing: register at least one repository with --neus-repo")
	}
	if c.Enabled {
		if _, err := canonicalIndexRoots(c.Repos); err != nil {
			return err
		}
	}
	return nil
}

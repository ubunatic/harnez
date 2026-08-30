package usage

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// localConfigDirEnv is the XDG Base Directory env var that, when set,
// overrides the default config directory location.
const localConfigDirEnv = "XDG_CONFIG_HOME"

// localConfigAppDir/localConfigFileName compose the user-local harnez config
// path: <config-base>/harnez/local.yaml. This file is separate from the
// committed/managed config.yaml — it is optional, never written by `harnez
// apply`, and holds machine-local values (issue 109).
const localConfigAppDir = "harnez"
const localConfigFileName = "local.yaml"

// LocalConfigDir resolves the user-local harnez config directory following
// the XDG Base Directory spec:
//
//	$XDG_CONFIG_HOME/harnez
//
// falling back to
//
//	~/.config/harnez
//
// when XDG_CONFIG_HOME is unset or empty. homeDir lets callers (and tests)
// pin the fallback base explicitly instead of relying on the real
// os.UserHomeDir(); pass "" to resolve it automatically.
func LocalConfigDir(homeDir string) string {
	if xdg := os.Getenv(localConfigDirEnv); xdg != "" {
		return filepath.Join(xdg, localConfigAppDir)
	}
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	return filepath.Join(homeDir, ".config", localConfigAppDir)
}

// LocalConfigPath resolves the full path to the user-local harnez config
// file: LocalConfigDir(homeDir)/local.yaml.
func LocalConfigPath(homeDir string) string {
	return filepath.Join(LocalConfigDir(homeDir), localConfigFileName)
}

// LocalUsageConfig is the `usage:` section of local.yaml.
type LocalUsageConfig struct {
	// DefaultHost is the SSH host used by `harnez usage` when --host is
	// omitted. Empty means no default is configured.
	DefaultHost string `yaml:"default_host"`
}

// LocalConfig is the parsed shape of ~/.config/harnez/local.yaml (issue
// 109). It is a minimal, user-local overlay on top of the committed
// config.yaml — machine-specific values that would otherwise cause drift in
// the shared, managed config.
type LocalConfig struct {
	Usage LocalUsageConfig `yaml:"usage"`
}

// LoadLocalConfig resolves and loads the user-local harnez config file. It
// always returns the resolved path as its second value, so callers (the
// `usage` command's --host fallback, `harnez status`'s presence report) can
// report or use it without re-deriving the XDG resolution themselves.
//
// Absence of the file is not an error (issue 109 acceptance criterion 5):
// it returns (nil, path, nil). A malformed file returns (nil, path, err).
func LoadLocalConfig(homeDir string) (*LocalConfig, string, error) {
	path := LocalConfigPath(homeDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, path, nil
		}
		return nil, path, fmt.Errorf("read local config %s: %w", path, err)
	}
	var cfg LocalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, path, fmt.Errorf("parse local config %s: %w", path, err)
	}
	return &cfg, path, nil
}

package main

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed config.yaml commands docs/src
var defaultFS embed.FS

type Config struct {
	Dir         string            `yaml:"-"`
	FS          fs.FS             `yaml:"-"`
	TargetDir   string            `yaml:"target_dir"`
	Langs       []string          `yaml:"langs"`
	Model       string            `yaml:"model"`
	Effort      string            `yaml:"effort"`
	Verbs       []string          `yaml:"verbs"`
	Permissions Permissions       `yaml:"permissions"`
	Hooks       []Hook            `yaml:"hooks"`
	Env         map[string]string `yaml:"env"`
	MCPServers  []MCPServer       `yaml:"mcp_servers"`
	Commands    []Command         `yaml:"commands"`
	AgentsMD    AgentsMD          `yaml:"agents_md"`
}

type Permissions struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
}

type Hook struct {
	Event   string `yaml:"event"`
	Matcher string `yaml:"matcher,omitempty"`
	Command string `yaml:"command"`
}

type MCPServer struct {
	Name    string            `yaml:"name"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
}

type Command struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Content     string `yaml:"content"`
	File        string `yaml:"file"` // path relative to config dir; overrides content if set
}

type AgentsMD struct {
	Global    AgentsMDTarget      `yaml:"global"`
	Local     AgentsMDTarget      `yaml:"local"`
	Languages map[string]Language `yaml:"languages"`
}

type AgentsMDTarget struct {
	Target   string      `yaml:"target"`
	Symlink  string      `yaml:"symlink"`
	Sections []MDSection `yaml:"sections"`
}

type MDSection struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}

type Language struct {
	Ref     string `yaml:"ref"`
	Source  string `yaml:"source"`
	Target  string `yaml:"target"`
	Symlink string `yaml:"symlink"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.Dir = filepath.Dir(path)
	cfg.FS = os.DirFS(cfg.Dir)
	return &cfg, nil
}

func loadConfigEmbedded() (*Config, error) {
	data, err := defaultFS.ReadFile("config.yaml")
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.Dir = "."
	cfg.FS = defaultFS
	return &cfg, nil
}

func defaultTarget() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home + "/.claude"
}

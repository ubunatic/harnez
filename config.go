package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	TargetDir   string            `yaml:"target_dir"`
	Model       string            `yaml:"model"`
	Effort      string            `yaml:"effort"`
	Verbs       []string          `yaml:"verbs"`
	Permissions Permissions       `yaml:"permissions"`
	Hooks       []Hook            `yaml:"hooks"`
	Env         map[string]string `yaml:"env"`
	Keybindings []Keybinding      `yaml:"keybindings"`
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

type Keybinding struct {
	Key    string `yaml:"key"`
	Action string `yaml:"action"`
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
	return &cfg, yaml.Unmarshal(data, &cfg)
}

func defaultTarget() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home + "/.claude"
}

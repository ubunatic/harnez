package claude

import (
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

type Config struct {
	Dir               string            `yaml:"-"`
	FS                fs.FS             `yaml:"-"`
	TargetDir         string            `yaml:"target_dir"`
	Docs              []string          `yaml:"docs"`
	Model             string            `yaml:"model"`
	Effort            string            `yaml:"effort"`
	Verbs             []string          `yaml:"verbs"`
	Permissions       Permissions       `yaml:"permissions"`
	Hooks             []Hook            `yaml:"hooks"`
	Env               map[string]string `yaml:"env"`
	MCPServers        []MCPServer       `yaml:"mcp_servers"`
	Commands          []Command         `yaml:"commands"`
	Skills            []Command         `yaml:"skills"`
	SkillsTarget      string            `yaml:"skills_target"`
	CodexSkillsTarget string            `yaml:"codex_skills_target"`
	PrimeAgentTarget  string            `yaml:"prime_agent_target"`
	AgentsMD          AgentsMD          `yaml:"agents_md"`
	Make              MakeConfig        `yaml:"make"`
}

// MakeConfig steers how harnez reconciles its own targets (e.g. `help`)
// and the ⚙️ phony sentinel into a project's existing Makefile.
type MakeConfig struct {
	// PhonyFix controls how existing .PHONY declarations are handled:
	//   "ours" (default) — only ensure our own sentinel .PHONY line exists
	//   "all"             — also collapse other explicit .PHONY target lists
	//                       into the sentinel convention
	//   "none"            — never touch .PHONY lines
	PhonyFix string `yaml:"phony_fix"`
	// PhonySentinel overrides the sentinel token (default "⚙️"). If unset,
	// harnez auto-detects an existing sentinel variant in the file
	// (e.g. "⚙︎", the text-presentation form) before falling back to the default.
	PhonySentinel string `yaml:"phony_sentinel"`
}

func (m MakeConfig) phonyFixOrDefault() string {
	if m.PhonyFix == "" {
		return "ours"
	}
	return m.PhonyFix
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
	RepoModes map[string]RepoMode `yaml:"repo_modes"`
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

// RepoMode describes a repo's git setup (solo/fork/team) as a short
// "Repo Setup" section injected into a project's AGENTS.md via `init --repo-mode`.
type RepoMode struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}

type Language struct {
	Name     string `yaml:"name"`
	Ref      string `yaml:"ref"`
	Hint     string `yaml:"hint"`
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	Local    string `yaml:"local"`
	Template string `yaml:"template"` // scaffold file written once to project if absent
	Targets  string `yaml:"targets"`  // managed section injected into existing template file
	// Default controls whether init copies this doc without an explicit --doc flag.
	// "true" = always copy, "false" = only if explicitly requested, "auto" = detect from project.
	Default string `yaml:"default"`
}

func LoadConfig(path string) (*Config, error) {
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

func LoadConfigEmbedded() (*Config, error) {
	data, err := harnez.DefaultFS.ReadFile("config.yaml")
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.Dir = "."
	cfg.FS = harnez.DefaultFS
	return &cfg, nil
}

func DefaultTarget() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home + "/.claude"
}

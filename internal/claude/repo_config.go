package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/gitflow"
	"ubunatic.com/harnez/internal/issues"
	"ubunatic.com/harnez/internal/markdown"
)

// RepoConfig records project-local Harnez choices. It is deliberately separate
// from Config, which describes the installed Harnez application.
type RepoConfig struct {
	Version int `yaml:"version"`
	Issues  struct {
		Tracker     string `yaml:"tracker"`
		MergeDriver bool   `yaml:"merge_driver"`
		ActiveCap   int    `yaml:"active_cap,omitempty"`
	} `yaml:"issues"`
	Docs struct {
		Studies bool     `yaml:"studies"`
		Copies  []string `yaml:"copies"`
	} `yaml:"docs"`
	Rules struct {
		Quota bool `yaml:"quota"`
	} `yaml:"rules"`
	Make struct {
		Scaffold      bool `yaml:"scaffold"`
		ContainerTest bool `yaml:"container_test"`
	} `yaml:"make"`
	// Git stays zero when repo.yaml has no git key, so it is omitted on write;
	// gitflow.Load supplies the spec defaults.
	Git gitflow.Config `yaml:"git,omitempty"`
}

func repoConfigPath(dir string) string { return filepath.Join(dir, ".harnez", "repo.yaml") }

// LoadRepoConfig validates the project's config against its embedded schema.
func LoadRepoConfig(dir string) (*RepoConfig, error) {
	path := repoConfigPath(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	// gitflow.Load runs first so git errors name the file and key (git.mode).
	if _, err := gitflow.Load(dir); err != nil {
		return nil, err
	}
	if err := validateRepoDocument(doc); err != nil {
		return nil, fmt.Errorf("validate %s: %w", path, err)
	}
	var cfg RepoConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if makeConfig, ok := doc["make"].(map[string]any); ok {
		if _, exists := makeConfig["scaffold"]; !exists {
			cfg.Make.Scaffold = true
		}
	}
	return &cfg, nil
}

// validateRepoDocument checks a parsed repo.yaml against the embedded schema.
func validateRepoDocument(doc map[string]any) error {
	schemaData, err := harnez.DefaultFS.ReadFile("spec/schemas/repo.schema.json")
	if err != nil {
		return err
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return err
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return err
	}
	return resolved.Validate(doc)
}

// ActiveIssueCapForRepo returns the per-repo override when configured, or the
// embedded spec default otherwise. The source is suitable for user-facing diagnostics.
func ActiveIssueCapForRepo(dir string) (int, string, error) {
	cfg, err := LoadRepoConfig(dir)
	if err == nil && cfg.Issues.ActiveCap > 0 {
		return cfg.Issues.ActiveCap, ".harnez/repo.yaml", nil
	}
	if err != nil && !os.IsNotExist(err) {
		return 0, "", err
	}
	cap, err := issues.ActiveIssueCap()
	if err != nil {
		return 0, "", err
	}
	return cap, "spec/issues.yaml", nil
}

func detectRepoConfig(dir string, cfg *Config) *RepoConfig {
	r := &RepoConfig{Version: 1}
	issuesPath := filepath.Join(dir, "issues")
	if fileExists(issuesPath) {
		r.Issues.Tracker = "none"
		if hasHarnezIssuesIndex(dir) {
			r.Issues.Tracker = "harnez"
		}
	} else {
		r.Issues.Tracker = "none"
	}
	r.Issues.MergeDriver = r.Issues.Tracker == "harnez"
	r.Docs.Studies = fileExists(filepath.Join(dir, "docs", "studies"))
	r.Make.Scaffold = fileExists(filepath.Join(dir, "Makefile"))
	if cfg != nil {
		for _, name := range docNamesInOrder(cfg) {
			if detectDoc(dir, name) {
				r.Docs.Copies = append(r.Docs.Copies, name)
			}
		}
	}
	agentsPath := filepath.Join(dir, "AGENTS.md")
	agents, _ := os.ReadFile(agentsPath)
	r.Rules.Quota = markdown.ContainsSection(agentsPath, quota1SectionName) || strings.Contains(string(agents), "harnez:begin "+quota1SectionName) || makeTargetExists(dir, "test-q1")
	r.Make.ContainerTest = makeTargetExists(dir, "container-test")
	return r
}

func hasHarnezIssuesIndex(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "issues", "README.md"))
	return err == nil && (strings.Contains(string(data), "managed by harnez index") || strings.Contains(string(data), "harnez:begin"))
}

func makeTargetExists(dir, name string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "Makefile"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon > 0 && strings.TrimSpace(line[:colon]) == name {
			return true
		}
	}
	return false
}

func writeRepoConfig(dir string, cfg *RepoConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	path := repoConfigPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// AdoptRepoIssuesTracker records the file-based Harnez tracker after
// `harnez issues new` creates the issues/ directory in a repo that did not
// have one. Callers must establish that the directory was absent beforehand.
func AdoptRepoIssuesTracker(dir string) error {
	cfg, err := LoadRepoConfig(dir)
	if os.IsNotExist(err) {
		cfg = &RepoConfig{Version: 1}
	} else if err != nil {
		return err
	}
	cfg.Issues.Tracker = "harnez"
	cfg.Issues.MergeDriver = true
	return writeRepoConfig(dir, cfg)
}

func containsRepoDoc(docs []string, name string) bool {
	for _, item := range docs {
		if item == name {
			return true
		}
	}
	return false
}

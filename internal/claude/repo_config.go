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
	"ubunatic.com/harnez/internal/markdown"
)

// RepoConfig records project-local Harnez choices. It is deliberately separate
// from Config, which describes the installed Harnez application.
type RepoConfig struct {
	Version int `yaml:"version"`
	Issues  struct {
		Tracker     string `yaml:"tracker"`
		MergeDriver bool   `yaml:"merge_driver"`
	} `yaml:"issues"`
	Docs struct {
		Studies bool     `yaml:"studies"`
		Copies  []string `yaml:"copies"`
	} `yaml:"docs"`
	Rules struct {
		Quota bool `yaml:"quota"`
	} `yaml:"rules"`
	Make struct {
		ContainerTest bool `yaml:"container_test"`
	} `yaml:"make"`
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
	schemaData, err := harnez.DefaultFS.ReadFile("spec/schemas/repo.schema.json")
	if err != nil {
		return nil, err
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return nil, err
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, err
	}
	if err := resolved.Validate(doc); err != nil {
		return nil, fmt.Errorf("validate %s: %w", path, err)
	}
	var cfg RepoConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

func detectRepoConfig(dir string, cfg *Config) *RepoConfig {
	r := &RepoConfig{Version: 1}
	issuesPath := filepath.Join(dir, "issues")
	if fileExists(issuesPath) {
		r.Issues.Tracker = "none"
		if data, err := os.ReadFile(filepath.Join(issuesPath, "README.md")); err == nil &&
			(len(data) > 0) && (strings.Contains(string(data), "managed by harnez index") || strings.Contains(string(data), "harnez:begin")) {
			r.Issues.Tracker = "harnez"
		}
	} else {
		r.Issues.Tracker = "none"
	}
	r.Issues.MergeDriver = r.Issues.Tracker == "harnez"
	r.Docs.Studies = fileExists(filepath.Join(dir, "docs", "studies"))
	if cfg != nil {
		for _, name := range docNamesInOrder(cfg) {
			if detectDoc(dir, name) {
				r.Docs.Copies = append(r.Docs.Copies, name)
			}
		}
	}
	r.Rules.Quota = markdown.ContainsSection(filepath.Join(dir, "AGENTS.md"), quota1SectionName) || fileExists(filepath.Join(dir, ".harnez", "rules", "Quota.md"))
	r.Make.ContainerTest = fileExists(filepath.Join(dir, "Makefile"))
	return r
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

func containsRepoDoc(docs []string, name string) bool {
	for _, item := range docs {
		if item == name {
			return true
		}
	}
	return false
}

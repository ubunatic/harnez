// Package gitflow reads a repo's git workflow mode from .harnez/repo.yaml and
// answers which branch counts as the default (and so as protected). Defaults
// and fallback branch names live in spec/git.yaml, not in Go.
package gitflow

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// Mode is the repo's git workflow: solo commits on the default branch, team
// and fork use topic branches and pull requests.
type Mode string

const (
	ModeSolo Mode = "solo"
	ModeFork Mode = "fork"
	ModeTeam Mode = "team"
)

// Config is the `git:` key of .harnez/repo.yaml.
type Config struct {
	Mode          Mode   `yaml:"mode"`
	ProtectedMain bool   `yaml:"protected_main"`
	DefaultBranch string `yaml:"default_branch,omitempty"`
}

type specDefaults struct {
	Mode             Mode     `yaml:"mode"`
	ProtectedMain    bool     `yaml:"protected_main"`
	FallbackBranches []string `yaml:"fallback_branches"`
}

var loadSpec = sync.OnceValues(func() (specDefaults, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/git.yaml")
	if err != nil {
		return specDefaults{}, fmt.Errorf("read spec/git.yaml: %w", err)
	}
	schemaData, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/git.schema.json")
	if err != nil {
		return specDefaults{}, fmt.Errorf("read git schema: %w", err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return specDefaults{}, fmt.Errorf("parse spec/git.yaml: %w", err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return specDefaults{}, fmt.Errorf("parse git schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return specDefaults{}, fmt.Errorf("resolve git schema: %w", err)
	}
	if err := resolved.Validate(document); err != nil {
		return specDefaults{}, fmt.Errorf("validate spec/git.yaml: %w", err)
	}
	var spec specDefaults
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return specDefaults{}, fmt.Errorf("decode spec/git.yaml: %w", err)
	}
	return spec, nil
})

var gitKeys = []string{"mode", "protected_main", "default_branch"}

// Load reads only the `git` key of dir/.harnez/repo.yaml. A missing file or
// key yields the spec defaults. Unknown keys, an unknown mode and
// protected_main on a solo repo fail with an error naming the file and key, so
// callers that never run the full repo.yaml schema still reject them.
func Load(dir string) (Config, error) {
	spec, err := loadSpec()
	if err != nil {
		return Config{}, err
	}
	cfg := Config{Mode: spec.Mode, ProtectedMain: spec.ProtectedMain}
	path := filepath.Join(dir, ".harnez", "repo.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}
	var doc struct {
		Git map[string]any `yaml:"git"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	for key := range doc.Git {
		if !slices.Contains(gitKeys, key) {
			return Config{}, fmt.Errorf("%s: unknown key git.%s (want one of %s)", path, key, strings.Join(gitKeys, ", "))
		}
	}
	if v, ok := doc.Git["mode"]; ok {
		s, _ := v.(string)
		switch m := Mode(s); m {
		case ModeSolo, ModeFork, ModeTeam:
			cfg.Mode = m
		default:
			return Config{}, fmt.Errorf("%s: git.mode %v is not one of solo, fork, team", path, v)
		}
	}
	if v, ok := doc.Git["protected_main"]; ok {
		b, isBool := v.(bool)
		if !isBool {
			return Config{}, fmt.Errorf("%s: git.protected_main must be true or false, got %v", path, v)
		}
		cfg.ProtectedMain = b
	}
	if v, ok := doc.Git["default_branch"]; ok {
		s, _ := v.(string)
		if s == "" {
			return Config{}, fmt.Errorf("%s: git.default_branch must be a non-empty branch name, got %v", path, v)
		}
		cfg.DefaultBranch = s
	}
	if cfg.Mode == ModeSolo && cfg.ProtectedMain {
		return Config{}, fmt.Errorf("%s: git.protected_main requires git.mode team or fork, not solo", path)
	}
	return cfg, nil
}

// DefaultBranch returns the repo's default branch: git.default_branch when
// set, else the branch origin/HEAD points to. ok is false when neither is known.
func DefaultBranch(dir string, c Config) (string, bool) {
	if c.DefaultBranch != "" {
		return c.DefaultBranch, true
	}
	out, err := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD").Output()
	if err != nil {
		return "", false
	}
	branch := strings.TrimPrefix(strings.TrimSpace(string(out)), "origin/")
	return branch, branch != ""
}

// ProtectedBranches lists the branches that count as the default branch: the
// detected one, or every spec fallback name when detection fails (the
// conservative choice). Load fails first when the spec is unreadable, so an
// error here yields no fallback names only on a path that skipped Load.
func ProtectedBranches(dir string, c Config) []string {
	if branch, ok := DefaultBranch(dir, c); ok {
		return []string{branch}
	}
	spec, _ := loadSpec()
	return slices.Clone(spec.FallbackBranches)
}

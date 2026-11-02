// Package prwait waits for a pull request to be merged or closed on a Forgejo or GitHub forge.
package prwait

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// Forge describes one supported forge host from spec/pr.yaml.
type Forge struct {
	Host     string   `yaml:"host"`
	Kind     string   `yaml:"kind"`
	APIBase  string   `yaml:"api_base"`
	TokenEnv []string `yaml:"token_env"`
}

// Spec holds the pull request wait settings loaded from spec/pr.yaml.
type Spec struct {
	DefaultRemote        string
	PollInterval         time.Duration
	DefaultTimeout       time.Duration
	MaxConsecutiveErrors int
	Forges               []Forge
}

type rawSpec struct {
	DefaultRemote        string  `yaml:"default_remote"`
	PollInterval         string  `yaml:"poll_interval"`
	DefaultTimeout       string  `yaml:"default_timeout"`
	MaxConsecutiveErrors int     `yaml:"max_consecutive_errors"`
	Forges               []Forge `yaml:"forges"`
}

// LoadSpec reads spec/pr.yaml from the embedded spec FS, validates it against
// spec/schemas/pr.schema.json and decodes it.
func LoadSpec() (Spec, error) {
	return loadSpecFrom(harnez.DefaultFS, "spec/pr.yaml", "spec/schemas/pr.schema.json")
}

func loadSpecFrom(fsys fs.FS, specPath, schemaPath string) (Spec, error) {
	data, err := fs.ReadFile(fsys, specPath)
	if err != nil {
		return Spec{}, fmt.Errorf("read %s: %w", specPath, err)
	}
	schemaData, err := fs.ReadFile(fsys, schemaPath)
	if err != nil {
		return Spec{}, fmt.Errorf("read %s: %w", schemaPath, err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return Spec{}, fmt.Errorf("parse %s: %w", specPath, err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return Spec{}, fmt.Errorf("parse %s: %w", schemaPath, err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return Spec{}, fmt.Errorf("resolve %s: %w", schemaPath, err)
	}
	if err := resolved.Validate(document); err != nil {
		return Spec{}, fmt.Errorf("validate %s: %w", specPath, err)
	}
	var raw rawSpec
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Spec{}, fmt.Errorf("decode %s: %w", specPath, err)
	}
	poll, err := time.ParseDuration(raw.PollInterval)
	if err != nil {
		return Spec{}, fmt.Errorf("%s: invalid poll_interval: %w", specPath, err)
	}
	timeout, err := time.ParseDuration(raw.DefaultTimeout)
	if err != nil {
		return Spec{}, fmt.Errorf("%s: invalid default_timeout: %w", specPath, err)
	}
	return Spec{
		DefaultRemote:        raw.DefaultRemote,
		PollInterval:         poll,
		DefaultTimeout:       timeout,
		MaxConsecutiveErrors: raw.MaxConsecutiveErrors,
		Forges:               raw.Forges,
	}, nil
}

// ForgeFor returns the configured forge for host, matched case-insensitively.
func (s Spec) ForgeFor(host string) (Forge, bool) {
	for _, f := range s.Forges {
		if strings.EqualFold(f.Host, host) {
			return f, true
		}
	}
	return Forge{}, false
}

package decide

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// specPath is the embedded backend registry; see docs/Spec.md.
const specPath = "spec/decide.yaml"

// BackendSpec is one entry in spec/decide.yaml. Mirrors
// spec/schemas/decide.schema.json field-for-field.
type BackendSpec struct {
	Name        string `yaml:"-"`
	Description string `yaml:"description,omitempty"`
	Protocol    string `yaml:"protocol"`
	BaseURL     string `yaml:"base_url"`
	BaseURLEnv  string `yaml:"base_url_env,omitempty"`
	Path        string `yaml:"path"`
	Model       string `yaml:"model,omitempty"`
	APIKeyEnv   string `yaml:"api_key_env,omitempty"`
	Timeout     string `yaml:"timeout"`
}

// Spec is the top-level shape of spec/decide.yaml.
type Spec struct {
	DefaultBackend string                  `yaml:"default_backend"`
	Backends       map[string]*BackendSpec `yaml:"backends"`
}

// protocols maps each protocol named in the schema's enum to its client.
// A new protocol (e.g. a local model with its own wire format) is added
// here and to the enum; TestProtocolsMatchSchema keeps both in step.
var protocols = map[string]func(BackendSpec, func(string) string) (Backend, error){
	"systemone": newSystemOne,
}

// LoadSpec reads and validates the embedded spec.
func LoadSpec() (*Spec, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, specPath)
	if err != nil {
		return nil, err
	}
	return parseSpec(data)
}

func parseSpec(data []byte) (*Spec, error) {
	var s Spec
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("%s: %w", specPath, err)
	}
	if len(s.Backends) == 0 {
		return nil, fmt.Errorf("%s: no backends", specPath)
	}
	if _, ok := s.Backends[s.DefaultBackend]; !ok {
		return nil, fmt.Errorf("%s: default_backend %q is not defined", specPath, s.DefaultBackend)
	}
	for name, b := range s.Backends {
		b.Name = name
		if _, ok := protocols[b.Protocol]; !ok {
			return nil, fmt.Errorf("%s: backend %q: unknown protocol %q", specPath, name, b.Protocol)
		}
		if !strings.HasPrefix(b.Path, "/") {
			return nil, fmt.Errorf("%s: backend %q: path must start with /", specPath, name)
		}
		if d, err := time.ParseDuration(b.Timeout); err != nil || d <= 0 {
			return nil, fmt.Errorf("%s: backend %q: timeout %q is not a positive duration", specPath, name, b.Timeout)
		}
	}
	return &s, nil
}

// BackendNames returns the defined backend names in sorted order.
func (s *Spec) BackendNames() []string {
	names := make([]string, 0, len(s.Backends))
	for n := range s.Backends {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Open returns the named backend, or the default when name is empty.
// getenv supplies keys and base URL overrides; nil means os.Getenv.
func (s *Spec) Open(name string, getenv func(string) string) (Backend, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	if name == "" {
		name = s.DefaultBackend
	}
	b, ok := s.Backends[name]
	if !ok {
		return nil, fmt.Errorf("unknown backend %q (have: %s)", name, strings.Join(s.BackendNames(), ", "))
	}
	return protocols[b.Protocol](*b, getenv)
}

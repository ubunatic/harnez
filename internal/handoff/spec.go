// Package handoff loads the agent profiles used by the /harnez-handoff skill
// (issue 726). `harnez apply` renders the spec into the installed skill (see
// RenderMarkdown), so the skill never reads the spec at runtime.
package handoff

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// specPath is the embedded handoff spec; see docs/Spec.md.
const specPath = "spec/handoff.yaml"

// cloudProfile mirrors the schema's if/then rule: agents with this profile
// must say which hosts they reach and how their result comes back; all other
// agents must not.
const cloudProfile = "cloud"

// Profile is one entry under profiles in spec/handoff.yaml.
type Profile struct {
	Description string   `yaml:"description"`
	Rules       []string `yaml:"rules"`
}

// Agent is one entry under agents in spec/handoff.yaml. Mirrors
// spec/schemas/handoff.schema.json field-for-field.
type Agent struct {
	Key     string   `yaml:"-"`
	Name    string   `yaml:"name"`
	Profile string   `yaml:"profile"`
	Aliases []string `yaml:"aliases"`
	Hosts   []string `yaml:"hosts,omitempty"`
	Facts   []string `yaml:"facts"`
	Result  string   `yaml:"result,omitempty"`
}

// Spec is the top-level shape of spec/handoff.yaml.
type Spec struct {
	Profiles map[string]*Profile `yaml:"profiles"`
	Agents   map[string]*Agent   `yaml:"agents"`
}

// LoadSpec reads and validates the embedded spec.
func LoadSpec() (*Spec, error) {
	return LoadSpecFS(harnez.DefaultFS)
}

// LoadSpecFS reads and validates spec/handoff.yaml from fsys.
func LoadSpecFS(fsys fs.FS) (*Spec, error) {
	data, err := fs.ReadFile(fsys, specPath)
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
	if len(s.Profiles) == 0 {
		return nil, fmt.Errorf("%s: no profiles", specPath)
	}
	for name, p := range s.Profiles {
		if p == nil || p.Description == "" || len(p.Rules) == 0 {
			return nil, fmt.Errorf("%s: profile %q needs a description and rules", specPath, name)
		}
	}
	if len(s.Agents) == 0 {
		return nil, fmt.Errorf("%s: no agents", specPath)
	}
	aliases := map[string]string{}
	for key, a := range s.Agents {
		if a == nil {
			return nil, fmt.Errorf("%s: agent %q is empty", specPath, key)
		}
		a.Key = key
		if a.Name == "" || len(a.Facts) == 0 {
			return nil, fmt.Errorf("%s: agent %q needs a name and facts", specPath, key)
		}
		if a.Profile == "" {
			return nil, fmt.Errorf("%s: agent %q has no profile", specPath, key)
		}
		if _, ok := s.Profiles[a.Profile]; !ok {
			return nil, fmt.Errorf("%s: agent %q: unknown profile %q", specPath, key, a.Profile)
		}
		cloud := a.Profile == cloudProfile
		if cloud && (len(a.Hosts) == 0 || a.Result == "") {
			return nil, fmt.Errorf("%s: cloud agent %q needs hosts and result", specPath, key)
		}
		if !cloud && (len(a.Hosts) > 0 || a.Result != "") {
			return nil, fmt.Errorf("%s: %s agent %q must not set hosts or result", specPath, a.Profile, key)
		}
		if len(a.Aliases) == 0 {
			return nil, fmt.Errorf("%s: agent %q has no aliases", specPath, key)
		}
		for _, alias := range a.Aliases {
			alias = strings.ToLower(alias)
			if other, dup := aliases[alias]; dup {
				return nil, fmt.Errorf("%s: alias %q used by agents %q and %q", specPath, alias, other, key)
			}
			aliases[alias] = key
		}
	}
	return &s, nil
}

// Find returns the agent whose key, name or alias matches name, case-insensitively.
func (s *Spec) Find(name string) *Agent {
	name = strings.ToLower(strings.TrimSpace(name))
	for key, a := range s.Agents {
		if key == name || strings.ToLower(a.Name) == name {
			return a
		}
		for _, alias := range a.Aliases {
			if strings.ToLower(alias) == name {
				return a
			}
		}
	}
	return nil
}

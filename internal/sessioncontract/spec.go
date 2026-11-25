// Package sessioncontract loads the planned session ownership contract.
package sessioncontract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// BwrapContract defines diagnostic guidance and prerequisite messages for
// bubblewrap sandboxing under Taia confinement and on the host.
type BwrapContract struct {
	HostFailurePrefix   string `yaml:"host_failure_prefix" json:"host_failure_prefix"`
	HostFailureGuidance string `yaml:"host_failure_guidance" json:"host_failure_guidance"`
	MissingHostError    string `yaml:"missing_host_error" json:"missing_host_error"`
}

// Ownership assigns a resource to an ownership class.
type Ownership struct {
	Resource string `yaml:"resource" json:"resource"`
	Class    string `yaml:"class" json:"class"`
}

// Contract describes the session state and installation target contract.
type Contract struct {
	Version            int           `yaml:"version" json:"version"`
	Ownership          []Ownership   `yaml:"ownership" json:"ownership"`
	RunnerOperations   []string      `yaml:"runner_operations" json:"runner_operations"`
	HostOnlyOperations []string      `yaml:"host_only_operations" json:"host_only_operations"`
	Bwrap              BwrapContract `yaml:"bwrap" json:"bwrap"`
	Promoter           struct {
		BinaryRoots []string `yaml:"binary_roots" json:"binary_roots"`
		ManRoots    []string `yaml:"man_roots" json:"man_roots"`
		Binary      string   `yaml:"binary" json:"binary"`
		Alias       string   `yaml:"alias" json:"alias"`
		AliasTarget string   `yaml:"alias_target" json:"alias_target"`
		ManManifest string   `yaml:"man_manifest" json:"man_manifest"`
		Promotion   string   `yaml:"promotion" json:"promotion"`
	} `yaml:"promoter" json:"promoter"`
}

var loadOnce = sync.OnceValues(func() (Contract, error) {
	data, err := harnez.DefaultFS.ReadFile("spec/session_isolation.yaml")
	if err != nil {
		return Contract{}, err
	}
	return decode(data)
})

// Load returns the decoded and validated embedded session isolation contract.
func Load() (Contract, error) {
	return loadOnce()
}

// AllowsRunnerOperation reports whether op is listed in RunnerOperations.
func (c Contract) AllowsRunnerOperation(op string) bool {
	return slices.Contains(c.RunnerOperations, op)
}
func decode(data []byte) (Contract, error) {
	var c Contract
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&c); err != nil {
		return c, err
	}
	return c, c.Validate()
}

// Validate checks the embedded schema and cross-field invariants.
func (c Contract) Validate() error {
	data, err := harnez.DefaultFS.ReadFile("spec/schemas/session_isolation.schema.json")
	if err != nil {
		return err
	}
	var schema jsonschema.Schema
	if err = json.Unmarshal(data, &schema); err != nil {
		return err
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		return err
	}
	var doc any
	if err = json.Unmarshal(encoded, &doc); err != nil {
		return err
	}
	if err = resolved.Validate(doc); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, entry := range c.Ownership {
		if seen[entry.Resource] {
			return fmt.Errorf("duplicate ownership: %s", entry.Resource)
		}
		seen[entry.Resource] = true
	}
	for _, op := range c.RunnerOperations {
		if slices.Contains(c.HostOnlyOperations, op) {
			return fmt.Errorf("conflicting operation: %s", op)
		}
	}
	if c.Promoter.AliasTarget != c.Promoter.Binary {
		return fmt.Errorf("alias target differs from binary")
	}
	return nil
}

// Package sessioncontract loads the planned session ownership contract.
package sessioncontract

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

type Ownership struct {
	Resource string `yaml:"resource"`
	Class    string `yaml:"class"`
}
type Contract struct {
	Version            int         `yaml:"version"`
	Ownership          []Ownership `yaml:"ownership"`
	RunnerOperations   []string    `yaml:"runner_operations"`
	HostOnlyOperations []string    `yaml:"host_only_operations"`
	Promoter           struct {
		BinaryRoots []string `yaml:"binary_roots"`
		ManRoots    []string `yaml:"man_roots"`
		Binary      string   `yaml:"binary"`
		Alias       string   `yaml:"alias"`
		AliasTarget string   `yaml:"alias_target"`
		ManManifest string   `yaml:"man_manifest"`
		Promotion   string   `yaml:"promotion"`
	} `yaml:"promoter"`
}

func Load() (Contract, error) {
	var c Contract
	data, err := harnez.DefaultFS.ReadFile("spec/session_isolation.yaml")
	if err != nil {
		return c, err
	}
	if err = yaml.Unmarshal(data, &c); err != nil {
		return c, err
	}
	return c, c.Validate()
}

func (c Contract) Validate() error {
	if c.Version != 1 || len(c.Ownership) == 0 {
		return fmt.Errorf("invalid contract version or ownership")
	}
	for _, entry := range c.Ownership {
		if entry.Resource == "" || !slices.Contains([]string{"runner", "broker", "shared"}, entry.Class) {
			return fmt.Errorf("invalid ownership: %s", entry.Resource)
		}
	}
	for _, op := range c.RunnerOperations {
		if !slices.Contains([]string{"telemetry_ingest", "telemetry_replay", "usage_read", "own_register", "own_update", "own_query"}, op) || slices.Contains(c.HostOnlyOperations, op) {
			return fmt.Errorf("invalid runner operation: %s", op)
		}
	}
	p := c.Promoter
	if !slices.Equal(p.BinaryRoots, []string{"GOBIN", "GOPATH_bin"}) || !slices.Equal(p.ManRoots, []string{"XDG_DATA_HOME_man_man1", "HOME_local_share_man_man1"}) || p.Binary != "harnez" || p.Alias != "⚙" || p.AliasTarget != p.Binary || p.ManManifest != "exact_generated_manifest" || p.Promotion != "pinned_nofollow_atomic_per_file" {
		return fmt.Errorf("invalid promoter contract")
	}
	return nil
}

// ValidateRoot checks a resolved root against a trusted host-derived root and checkout.
// It does not pin paths or protect against filesystem races.
func ValidateRoot(root, approved, checkout string) error {
	if !filepath.IsAbs(root) || !filepath.IsAbs(approved) || !filepath.IsAbs(checkout) || filepath.Clean(root) != root || root != approved {
		return fmt.Errorf("unapproved promoter root")
	}
	rel, err := filepath.Rel(filepath.Clean(checkout), root)
	if err != nil || rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return fmt.Errorf("promoter root inside checkout")
	}
	return nil
}

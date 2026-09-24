package subagent

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

const agentSpecPath = "spec/agent.yaml"

type agentSpec struct {
	DefaultModel string              `yaml:"default_model"`
	DefaultRole  string              `yaml:"default_role"`
	ModelsLegend string              `yaml:"models_legend"`
	Models       map[string]Model    `yaml:"models"`
	Roles        map[string]RoleSpec `yaml:"roles"`
}

// RoleSpec is one entry of the roles table in spec/agent.yaml.
type RoleSpec struct {
	Spawns []string `yaml:"spawns"`
	Rules  string   `yaml:"rules"`
}

func parseAgentSpec(data []byte) (agentSpec, error) {
	var spec agentSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return agentSpec{}, fmt.Errorf("agent spec: parse: %w", err)
	}
	if strings.TrimSpace(spec.DefaultModel) == "" {
		return agentSpec{}, fmt.Errorf("agent spec: default_model is required")
	}
	models := spec.Models
	if len(models) == 0 {
		var err error
		models, err = loadModelAliases()
		if err != nil {
			return agentSpec{}, err
		}
	}
	if _, err := resolveModelIn(models, spec.DefaultModel); err != nil {
		return agentSpec{}, fmt.Errorf("agent spec: default_model: %w", err)
	}
	if err := validateRoles(spec); err != nil {
		return agentSpec{}, err
	}
	return spec, nil
}

func loadModelAliases() (map[string]Model, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, agentSpecPath)
	if err != nil {
		return nil, fmt.Errorf("agent spec: read %s: %w", agentSpecPath, err)
	}
	var spec struct {
		Models map[string]Model `yaml:"models"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("agent spec: parse models: %w", err)
	}
	return spec.Models, nil
}

var modelAliasesOnce = sync.OnceValues(loadModelAliases)

func loadModelGuides() (map[string]ModelGuide, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, agentSpecPath)
	if err != nil {
		return nil, fmt.Errorf("agent spec: read %s: %w", agentSpecPath, err)
	}
	var spec struct {
		Models map[string]ModelGuide `yaml:"models"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("agent spec: parse model guides: %w", err)
	}
	return spec.Models, nil
}

var modelGuidesOnce = sync.OnceValues(loadModelGuides)

func ensureModelAliases() error {
	var err error
	modelAliases, err = modelAliasesOnce()
	return err
}

func loadAgentSpec() (agentSpec, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, agentSpecPath)
	if err != nil {
		return agentSpec{}, fmt.Errorf("agent spec: read %s: %w", agentSpecPath, err)
	}
	return parseAgentSpec(data)
}

var agentSpecOnce = sync.OnceValues(loadAgentSpec)

// ModelsLegend is the one-line key printed under `harnez agent models`.
func ModelsLegend() (string, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return "", err
	}
	return spec.ModelsLegend, nil
}

func DefaultModelSpec() (string, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return "", err
	}
	return spec.DefaultModel, nil
}

func DefaultModel() (Model, error) {
	spec, err := DefaultModelSpec()
	if err != nil {
		return Model{}, err
	}
	return ResolveModel(spec)
}

func validateRoles(spec agentSpec) error {
	if _, ok := spec.Roles[spec.DefaultRole]; !ok {
		return fmt.Errorf("agent spec: default_role %q is not defined in roles", spec.DefaultRole)
	}
	for name, role := range spec.Roles {
		if strings.TrimSpace(role.Rules) == "" {
			return fmt.Errorf("agent spec: role %q has no rules", name)
		}
		for _, child := range role.Spawns {
			if _, ok := spec.Roles[child]; !ok {
				return fmt.Errorf("agent spec: role %q spawns undefined role %q", name, child)
			}
			if child == name {
				return fmt.Errorf("agent spec: role %q may not spawn itself", name)
			}
		}
	}
	return nil
}

// DefaultRole is the role of a session started without --role.
func DefaultRole() (string, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return "", err
	}
	return spec.DefaultRole, nil
}

// RoleNames lists the defined roles in sorted order.
func RoleNames() ([]string, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(spec.Roles))
	for name := range spec.Roles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// RoleRules returns the preamble rules of a role.
func RoleRules(role string) (string, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return "", err
	}
	r, ok := spec.Roles[role]
	if !ok {
		return "", unknownRoleError(spec, role)
	}
	return strings.TrimSpace(r.Rules), nil
}

// CheckSpawn reports whether a caller in callerRole may start a session of
// target. An empty callerRole is a human or an untracked host: unrestricted.
func CheckSpawn(callerRole, target string) error {
	spec, err := agentSpecOnce()
	if err != nil {
		return err
	}
	if _, ok := spec.Roles[target]; !ok {
		return unknownRoleError(spec, target)
	}
	if callerRole == "" {
		return nil
	}
	caller, ok := spec.Roles[callerRole]
	if !ok {
		return unknownRoleError(spec, callerRole)
	}
	if len(caller.Spawns) == 0 {
		return fmt.Errorf("agent role %q is a leaf worker: it must not start, resume or manage agents; do the work yourself and report back; the host runs live harnez agent checks after your commit", callerRole)
	}
	for _, allowed := range caller.Spawns {
		if allowed == target {
			return nil
		}
	}
	return fmt.Errorf("agent role %q may only start %s sessions, not %q", callerRole, strings.Join(caller.Spawns, ", "), target)
}

// IsLeafRole reports whether callerRole is a defined role that may not spawn.
func IsLeafRole(callerRole string) bool {
	spec, err := agentSpecOnce()
	if err != nil || callerRole == "" {
		return false
	}
	r, ok := spec.Roles[callerRole]
	return ok && len(r.Spawns) == 0
}

func unknownRoleError(spec agentSpec, role string) error {
	names := make([]string, 0, len(spec.Roles))
	for n := range spec.Roles {
		names = append(names, n)
	}
	sort.Strings(names)
	return fmt.Errorf("unknown agent role %q; known roles: %s", role, strings.Join(names, ", "))
}

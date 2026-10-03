package subagent

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

const agentSpecPath = "spec/agent.yaml"

type agentSpec struct {
	DefaultModel     string                         `yaml:"default_model"`
	DefaultRole      string                         `yaml:"default_role"`
	ModelsLegend     string                         `yaml:"models_legend"`
	Models           map[string]modelAlias          `yaml:"models"`
	Roles            map[string]RoleSpec            `yaml:"roles"`
	Stop             StopSpec                       `yaml:"stop"`
	Selftest         SelftestSpec                   `yaml:"selftest"`
	ExternalSessions map[string]ExternalSessionSpec `yaml:"external_sessions"`
}

// ExternalSessionSpec describes provider-owned local session metadata.
type ExternalSessionSpec struct {
	Root                     string                `yaml:"root"`
	Pattern                  string                `yaml:"pattern"`
	Format                   string                `yaml:"format"`
	RecordTypePath           string                `yaml:"record_type_path"`
	RecordType               string                `yaml:"record_type"`
	MaxRecordBytes           int                   `yaml:"max_record_bytes"`
	ActiveMatchWindowSeconds int                   `yaml:"active_match_window_seconds"`
	NamePrefix               string                `yaml:"name_prefix"`
	AvailableStatus          string                `yaml:"available_status"`
	Fields                   ExternalSessionFields `yaml:"fields"`
}

// ExternalSessionFields maps normalized values to fields in a provider record.
type ExternalSessionFields struct {
	ID                string `yaml:"id"`
	ProviderSessionID string `yaml:"provider_session_id"`
	WorkingDir        string `yaml:"working_dir"`
	ModelProvider     string `yaml:"model_provider"`
	Source            string `yaml:"source"`
	CreatedAt         string `yaml:"created_at"`
}

// StopSpec defines bounded agent shutdown in milliseconds.
type StopSpec struct {
	GraceMS    int `yaml:"grace_ms"`
	KillWaitMS int `yaml:"kill_wait_ms"`
	PollMS     int `yaml:"poll_ms"`
}

// SelftestSpec defines the agent background self-test duration in milliseconds.
type SelftestSpec struct {
	BackgroundDurationMS int `yaml:"background_duration_ms"`
}

// SelftestBackgroundDuration returns the embedded self-test background duration.
func SelftestBackgroundDuration() (time.Duration, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return 0, err
	}
	if spec.Selftest.BackgroundDurationMS <= 0 {
		return 0, fmt.Errorf("agent spec: positive selftest background duration required")
	}
	return time.Duration(spec.Selftest.BackgroundDurationMS) * time.Millisecond, nil
}

// StopBounds returns embedded shutdown timings.
func StopBounds() (time.Duration, time.Duration, time.Duration, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return 0, 0, 0, err
	}
	if spec.Stop.GraceMS <= 0 || spec.Stop.KillWaitMS <= 0 || spec.Stop.PollMS <= 0 {
		return 0, 0, 0, fmt.Errorf("agent spec: positive stop bounds required")
	}
	return time.Duration(spec.Stop.GraceMS) * time.Millisecond, time.Duration(spec.Stop.KillWaitMS) * time.Millisecond, time.Duration(spec.Stop.PollMS) * time.Millisecond, nil
}

// RoleSpec is one entry of the roles table in spec/agent.yaml.
type RoleSpec struct {
	Description string   `yaml:"description"`
	Aliases     []string `yaml:"aliases"`
	Spawns      []string `yaml:"spawns"`
	Rules       string   `yaml:"rules"`
}

func parseAgentSpec(data []byte) (agentSpec, error) {
	var spec agentSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return agentSpec{}, fmt.Errorf("agent spec: parse: %w", err)
	}
	if strings.TrimSpace(spec.DefaultModel) == "" {
		return agentSpec{}, fmt.Errorf("agent spec: default_model is required")
	}
	models := make(map[string]modelAlias, len(spec.Models))
	for key, entry := range spec.Models {
		models[key] = entry
	}
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
	if err := validateExternalSessions(spec.ExternalSessions); err != nil {
		return agentSpec{}, err
	}
	return spec, nil
}

func validateExternalSessions(specs map[string]ExternalSessionSpec) error {
	if len(specs) == 0 {
		return fmt.Errorf("agent spec: at least one external_sessions provider is required")
	}
	for provider, spec := range specs {
		if provider == "" || spec.Root == "" || spec.Pattern == "" || spec.RecordTypePath == "" || spec.RecordType == "" || spec.NamePrefix == "" || spec.AvailableStatus == "" {
			return fmt.Errorf("agent spec: external_sessions.%s has an empty required value", provider)
		}
		if spec.Format != "jsonl" || spec.MaxRecordBytes <= 0 || spec.ActiveMatchWindowSeconds <= 0 {
			return fmt.Errorf("agent spec: external_sessions.%s requires jsonl, a positive max_record_bytes, and a positive active_match_window_seconds", provider)
		}
		if spec.Fields.ID == "" || spec.Fields.WorkingDir == "" || spec.Fields.ModelProvider == "" || spec.Fields.Source == "" || spec.Fields.CreatedAt == "" {
			return fmt.Errorf("agent spec: external_sessions.%s has incomplete field mappings", provider)
		}
	}
	return nil
}

func loadModelAliases() (map[string]modelAlias, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, agentSpecPath)
	if err != nil {
		return nil, fmt.Errorf("agent spec: read %s: %w", agentSpecPath, err)
	}
	var spec struct {
		Models map[string]modelAlias `yaml:"models"`
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
	roleNames := make(map[string]struct{}, len(spec.Roles))
	for name := range spec.Roles {
		roleNames[name] = struct{}{}
	}
	aliases := make(map[string]string)
	for name, role := range spec.Roles {
		if strings.TrimSpace(role.Description) == "" || strings.ContainsAny(role.Description, "\r\n") {
			return fmt.Errorf("agent spec: role %q requires a one-line description", name)
		}
		for _, alias := range role.Aliases {
			if alias == "" || strings.TrimSpace(alias) != alias {
				return fmt.Errorf("agent spec: role %q has an empty alias", name)
			}
			if _, ok := roleNames[alias]; ok {
				return fmt.Errorf("agent spec: role %q alias %q conflicts with a canonical role", name, alias)
			}
			if previous, ok := aliases[alias]; ok {
				return fmt.Errorf("agent spec: role alias %q is shared by %q and %q", alias, previous, name)
			}
			aliases[alias] = name
		}
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

// ResolveRole resolves a canonical role name or alias to its canonical name.
func ResolveRole(name string) (string, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return "", err
	}
	return resolveRoleIn(spec, name)
}

// RoleHelp returns a sorted one-line catalog for agent command help.
func RoleHelp() (string, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return "", err
	}
	return roleCatalog(spec), nil
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
	canonical, err := resolveRoleIn(spec, role)
	if err != nil {
		return "", err
	}
	r := spec.Roles[canonical]
	return strings.TrimSpace(r.Rules), nil
}

// CheckSpawn reports whether a caller in callerRole may start a session of
// target. An empty callerRole is a human or an untracked host: unrestricted.
func CheckSpawn(callerRole, target string) error {
	spec, err := agentSpecOnce()
	if err != nil {
		return err
	}
	canonical, err := resolveRoleIn(spec, target)
	if err != nil {
		return err
	}
	target = canonical
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
	return fmt.Errorf("unknown agent role %q; known roles:\n%s", role, roleCatalog(spec))
}

func resolveRoleIn(spec agentSpec, name string) (string, error) {
	if _, ok := spec.Roles[name]; ok {
		return name, nil
	}
	for canonical, role := range spec.Roles {
		for _, alias := range role.Aliases {
			if alias == name {
				return canonical, nil
			}
		}
	}
	return "", unknownRoleError(spec, name)
}

func roleCatalog(spec agentSpec) string {
	names := make([]string, 0, len(spec.Roles))
	for name := range spec.Roles {
		names = append(names, name)
	}
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		role := spec.Roles[name]
		aliases := append([]string(nil), role.Aliases...)
		sort.Strings(aliases)
		line := name
		if len(aliases) > 0 {
			line += " (aliases: " + strings.Join(aliases, ", ") + ")"
		}
		lines = append(lines, "  "+line+": "+strings.TrimSpace(role.Description))
	}
	return strings.Join(lines, "\n")
}

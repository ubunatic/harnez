package subagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/modelcatalog"
)

const agentSpecPath = "spec/agent.yaml"

type agentSpec struct {
	ModelTraits           map[string]string              `yaml:"model_traits"`
	DefaultModel          string                         `yaml:"default_model"`
	DefaultRole           string                         `yaml:"default_role"`
	RoleClassifyTimeoutMS int                            `yaml:"role_classify_timeout_ms"`
	ModelsLegend          string                         `yaml:"models_legend"`
	Models                map[string]modelAlias          `yaml:"models"`
	Roles                 map[string]RoleSpec            `yaml:"roles"`
	Stop                  StopSpec                       `yaml:"stop"`
	Selftest              SelftestSpec                   `yaml:"selftest"`
	ExternalSessions      map[string]ExternalSessionSpec `yaml:"external_sessions"`
	ReplyHistoryLimit     int                            `yaml:"reply_history_limit"`
	CompactThreshold      int                            `yaml:"compact_threshold_tokens"`
	CompactThresholds     map[string]int                 `yaml:"compact_thresholds"`
	AutoCompact           AutoCompactSpec                `yaml:"auto_compact"`
	StaleSessionErrors    map[string][]string            `yaml:"stale_session_errors"`
	KnownErrorChecks      []KnownErrorCheckSpec          `yaml:"known_error_checks"`
	ResumeNote            ResumeNoteSpec                 `yaml:"resume_note"`
}

// AutoCompactSpec defines provider compaction routing and watchdog timing.
type AutoCompactSpec struct {
	DefaultMode                 string            `yaml:"default_mode"`
	ProviderModes               map[string]string `yaml:"provider_modes"`
	WatchdogGraceMS             int               `yaml:"watchdog_grace_ms"`
	LegacyGrowthThresholdTokens int               `yaml:"legacy_growth_threshold_tokens"`
	CodexOnDemandLimitTokens    int               `yaml:"codex_on_demand_limit_tokens"`
	CodexStatusPrompt           string            `yaml:"codex_status_prompt"`
}

func (s AutoCompactSpec) mode(provider string) string {
	if mode, ok := s.ProviderModes[provider]; ok {
		return mode
	}
	return s.DefaultMode
}

// AutoCompactMode reports the configured compaction routing for a provider.
func AutoCompactMode(provider string) (string, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return "", err
	}
	return spec.AutoCompact.mode(provider), nil
}

// KnownErrorCheckSpec describes a fixed-text provider error and its single
// configured verification check.
type KnownErrorCheckSpec struct {
	Provider  string `yaml:"provider"`
	MatchText string `yaml:"match_text"`
	Check     string `yaml:"check"`
	Warning   string `yaml:"warning"`
}

// ResumeNoteSpec controls the idle threshold and note added to resumed turns.
type ResumeNoteSpec struct {
	IdleThreshold string `yaml:"idle_threshold"`
	Text          string `yaml:"text"`
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
	InteractiveSources       []string              `yaml:"interactive_sources"`
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
	DefaultModel string              `yaml:"default_model"`
	Models       map[string][]string `yaml:"models"`
	Description  string              `yaml:"description"`
	Aliases      []string            `yaml:"aliases"`
	Spawns       []string            `yaml:"spawns"`
	// AutoApproveTools lets an agy session of this role run tools without a
	// human to approve them (issue 995); other providers ignore it.
	AutoApproveTools bool   `yaml:"auto_approve_tools"`
	Rules            string `yaml:"rules"`
}

// ModelPick describes one configured role and trait.
type ModelPick struct {
	Role     string   `json:"role"`
	Trait    string   `json:"trait"`
	Chain    []string `json:"chain"`
	Resolved string   `json:"resolved"`
	Error    string   `json:"error"`
	Default  bool     `json:"default"`
}

// ModelPicksWithAvailability reports picks using the caller's cached quota state.
func ModelPicksWithAvailability(exhausted func(Model) bool) ([]ModelPick, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return nil, err
	}
	return modelPicksIn(spec, func(role, trait string) (Model, error) {
		selection, err := ResolveModelWithAvailability(role, trait, exhausted, false)
		return selection.Model, err
	}), nil
}

func modelPicksIn(spec agentSpec, resolve func(string, string) (Model, error)) []ModelPick {
	var picks []ModelPick
	for name, role := range spec.Roles {
		for trait, targets := range role.Models {
			pick := ModelPick{Role: name, Trait: trait, Chain: append([]string{}, targets...), Default: role.DefaultModel == trait}
			if len(targets) == 0 {
				pick.Error = "unset"
			} else if model, err := resolve(name, trait); err != nil {
				pick.Error = err.Error()
			} else {
				pick.Resolved = model.Spec()
			}
			picks = append(picks, pick)
		}
	}
	sort.Slice(picks, func(i, j int) bool {
		if picks[i].Role != picks[j].Role {
			return picks[i].Role < picks[j].Role
		}
		return picks[i].Trait < picks[j].Trait
	})
	return picks
}

func parseAgentSpec(data []byte) (agentSpec, error) {
	var spec agentSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return agentSpec{}, fmt.Errorf("agent spec: parse: %w", err)
	}
	if strings.TrimSpace(spec.DefaultModel) == "" {
		return agentSpec{}, fmt.Errorf("agent spec: default_model is required")
	}
	if spec.RoleClassifyTimeoutMS <= 0 {
		return agentSpec{}, fmt.Errorf("agent spec: positive role_classify_timeout_ms required")
	}
	if spec.ReplyHistoryLimit <= 0 {
		return agentSpec{}, fmt.Errorf("agent spec: positive reply_history_limit required")
	}
	if spec.CompactThreshold <= 0 {
		return agentSpec{}, fmt.Errorf("agent spec: positive compact_threshold_tokens required")
	}
	for key, threshold := range spec.CompactThresholds {
		if threshold <= 0 {
			return agentSpec{}, fmt.Errorf("agent spec: compact_thresholds.%s must be positive", key)
		}
	}
	validMode := func(mode string) bool { return mode == "manual" || mode == "native" || mode == "disabled" }
	if !validMode(spec.AutoCompact.DefaultMode) || spec.AutoCompact.ProviderModes == nil || spec.AutoCompact.WatchdogGraceMS <= 0 || spec.AutoCompact.LegacyGrowthThresholdTokens <= 0 {
		return agentSpec{}, fmt.Errorf("agent spec: auto_compact requires valid modes and positive watchdog_grace_ms")
	}
	if spec.AutoCompact.CodexOnDemandLimitTokens <= 0 || strings.TrimSpace(spec.AutoCompact.CodexStatusPrompt) == "" {
		return agentSpec{}, fmt.Errorf("agent spec: auto_compact requires positive codex_on_demand_limit_tokens and nonempty codex_status_prompt")
	}
	for provider, mode := range spec.AutoCompact.ProviderModes {
		if !validMode(mode) {
			return agentSpec{}, fmt.Errorf("agent spec: invalid auto_compact mode for %s", provider)
		}
	}
	models := make(map[string]modelAlias, len(spec.Models))
	for key, entry := range spec.Models {
		models[key] = entry
	}
	if len(models) == 0 {
		var err error
		models, err = modelAliasMap()
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
	hasRoleModels := false
	for _, role := range spec.Roles {
		if role.Models != nil {
			hasRoleModels = true
			break
		}
	}
	if spec.ModelTraits != nil || hasRoleModels {
		if err := validateRoleModels(spec, models); err != nil {
			return agentSpec{}, err
		}
	}
	if err := validateExternalSessions(spec.ExternalSessions); err != nil {
		return agentSpec{}, err
	}
	return spec, nil
}

var purposeTraitName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func validateRoleModels(spec agentSpec, models map[string]modelAlias) error {
	return validateRoleModelsWithUserTraits(spec, models, nil)
}

func validateRoleModelsWithUserTraits(spec agentSpec, models map[string]modelAlias, userTraits map[string]bool) error {
	traits := make(map[string]bool, len(spec.ModelTraits)+len(userTraits))
	for trait := range spec.ModelTraits {
		traits[trait] = true
	}
	for trait := range userTraits {
		traits[trait] = true
	}
	for trait := range traits {
		description, described := spec.ModelTraits[trait]
		if !purposeTraitName.MatchString(trait) || isTier(normalizeTier(trait)) || described && (strings.TrimSpace(description) == "" || strings.ContainsAny(description, "\r\n")) {
			return fmt.Errorf("agent spec: invalid trait %q", trait)
		}
		for key, model := range models {
			if strings.EqualFold(trait, model.Model.Provider) || strings.EqualFold(trait, strings.SplitN(key, ":", 2)[0]) || modelHasAlias(model, trait) || strings.EqualFold(trait, model.Model.Name) {
				return fmt.Errorf("agent spec: trait %q collides with model or provider", trait)
			}
		}
		for name, role := range spec.Roles {
			if strings.EqualFold(trait, name) {
				return fmt.Errorf("agent spec: trait %q collides with role", trait)
			}
			for _, alias := range role.Aliases {
				if strings.EqualFold(trait, alias) {
					return fmt.Errorf("agent spec: trait %q collides with role alias", trait)
				}
			}
		}
	}
	for roleName, role := range spec.Roles {
		if role.DefaultModel != "" {
			if _, ok := role.Models[role.DefaultModel]; !ok {
				return fmt.Errorf("agent spec: role %q default_model %q is undefined", roleName, role.DefaultModel)
			}
		}
		for trait, targets := range role.Models {
			if _, ok := traits[trait]; !ok {
				return fmt.Errorf("agent spec: role %q has unknown trait %q", roleName, trait)
			}
			seen := map[string]bool{}
			for _, target := range targets {
				if target == "" || seen[target] {
					return fmt.Errorf("agent spec: role %q empty or duplicate target for %q", roleName, trait)
				}
				seen[target] = true
			}
		}
		var walk func(string, map[string]bool, int) error
		walk = func(trait string, visiting map[string]bool, depth int) error {
			if depth > maxTraitDepth {
				return fmt.Errorf("trait depth exceeds %d at %q", maxTraitDepth, trait)
			}
			if visiting[trait] {
				return fmt.Errorf("trait cycle at %q", trait)
			}
			visiting[trait] = true
			defer delete(visiting, trait)
			for _, target := range role.Models[trait] {
				if _, ok := role.Models[target]; ok {
					if err := walk(target, visiting, depth+1); err != nil {
						return err
					}
				} else if _, err := resolveModelIn(models, target); err != nil {
					return fmt.Errorf("agent spec: trait target %q: %w", target, err)
				}
			}
			return nil
		}
		for trait := range role.Models {
			if err := walk(trait, map[string]bool{}, 1); err != nil {
				return fmt.Errorf("role %s trait %s: %w", roleName, trait, err)
			}
		}
	}
	return nil
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

func loadAgentSpec() (agentSpec, error) {
	if err := ensureModelCatalog(); err != nil {
		return agentSpec{}, err
	}
	data, err := fs.ReadFile(harnez.DefaultFS, agentSpecPath)
	if err != nil {
		return agentSpec{}, fmt.Errorf("agent spec: read %s: %w", agentSpecPath, err)
	}
	spec, err := parseAgentSpec(data)
	if err != nil {
		return agentSpec{}, err
	}
	return mergeRolePolicy(spec, mergedModelCatalog.UserPolicy(), modelAliases, mergedModelCatalog.UserPolicyPath())
}

func mergeRolePolicy(spec agentSpec, policy *modelcatalog.Policy, models map[string]modelAlias, path string) (agentSpec, error) {
	if policy == nil {
		return spec, nil
	}
	copySpec := spec
	copySpec.ModelTraits = make(map[string]string, len(spec.ModelTraits)+len(policy.ModelTraits))
	for name, description := range spec.ModelTraits {
		copySpec.ModelTraits[name] = description
	}
	for name, description := range policy.ModelTraits {
		copySpec.ModelTraits[name] = description
	}
	copySpec.Roles = make(map[string]RoleSpec, len(spec.Roles))
	for name, role := range spec.Roles {
		copied := role
		copied.Models = make(map[string][]string, len(role.Models))
		for trait, targets := range role.Models {
			copied.Models[trait] = append([]string{}, targets...)
		}
		copySpec.Roles[name] = copied
	}
	userTraits := map[string]bool{}
	var contexts []string
	for name, role := range policy.Roles {
		current, ok := copySpec.Roles[name]
		if !ok {
			if canonical, err := resolveRoleIn(spec, name); err == nil {
				return agentSpec{}, fmt.Errorf("%s: role alias %q is not allowed; use canonical role %q", path, name, canonical)
			}
			return agentSpec{}, fmt.Errorf("%s: unknown role %q", path, name)
		}
		for trait, targets := range role.Models {
			current.Models[trait] = append([]string{}, targets...)
			userTraits[trait] = true
			contexts = append(contexts, name+"/"+trait)
		}
		copySpec.Roles[name] = current
	}
	sort.Strings(contexts)
	if err := validateRoleModelsWithUserTraits(copySpec, models, userTraits); err != nil {
		if strings.HasPrefix(err.Error(), "role ") {
			return agentSpec{}, fmt.Errorf("%s: %w", path, err)
		}
		return agentSpec{}, fmt.Errorf("%s: roles %s: %w", path, strings.Join(contexts, ", "), err)
	}
	return copySpec, nil
}

var agentSpecOnce = sync.OnceValues(loadAgentSpec)

// AgentReplyHistoryLimit returns the configured maximum number of saved replies.
func AgentReplyHistoryLimit() (int, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return 0, err
	}
	return spec.ReplyHistoryLimit, nil
}

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
	if err := ensureModelCatalog(); err != nil {
		return "", err
	}
	return spec.DefaultModel, nil
}

func DefaultModel() (Model, error) {
	spec, err := DefaultModelSpec()
	if err != nil {
		return Model{}, err
	}
	if err := ensureModelCatalog(); err != nil {
		return Model{}, err
	}
	return resolveDefaultModelWithCatalog(modelAliases, mergedModelCatalog, spec)
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

// ResolveRoleDynamic resolves a canonical role or alias first, then asks neus
// to classify unknown role wording. The bool is true when neus supplied the role.
func ResolveRoleDynamic(name string) (string, bool, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return "", false, err
	}
	if role, err := resolveRoleIn(spec, name); err == nil {
		return role, false, nil
	}
	role, err := classifyAgentRole(context.Background(), name, spec)
	if err == nil {
		return role, true, nil
	}
	return "", false, unknownRoleError(spec, name)
}

type roleClassification struct {
	Class string `json:"class"`
}

var runRoleClassifier = runNeusRoleClassifier

func runNeusRoleClassifier(ctx context.Context, input string, spec agentSpec) (string, error) {
	classes := make([]string, 0, len(spec.Roles))
	for role := range spec.Roles {
		classes = append(classes, role)
	}
	sort.Strings(classes)
	timeout := time.Duration(spec.RoleClassifyTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		return "", errors.New("role classifier timeout is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{"classify", "--json", "--classes", strings.Join(classes, ","), "--timeout", timeout.String(), input}
	output, err := exec.CommandContext(ctx, "neus", args...).Output()
	if err != nil {
		return "", err
	}
	var result roleClassification
	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}
	if _, ok := spec.Roles[result.Class]; !ok {
		return "", fmt.Errorf("neus returned unknown role %q", result.Class)
	}
	return result.Class, nil
}

func classifyAgentRole(ctx context.Context, input string, spec agentSpec) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", errors.New("empty role classification input")
	}
	return runRoleClassifier(ctx, input, spec)
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

// RoleAutoApproveTools reports whether role runs agy tools unattended.
func RoleAutoApproveTools(role string) (bool, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return false, err
	}
	canonical, err := resolveRoleIn(spec, role)
	if err != nil {
		return false, err
	}
	return spec.Roles[canonical].AutoApproveTools, nil
}

// KnownErrorChecks returns the ordered error checks from the agent spec.
func KnownErrorChecks() ([]KnownErrorCheckSpec, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return nil, err
	}
	return append([]KnownErrorCheckSpec(nil), spec.KnownErrorChecks...), nil
}

// ResumeNote returns the configured idle threshold and note text.
func ResumeNote() (time.Duration, string, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return 0, "", err
	}
	threshold, err := time.ParseDuration(spec.ResumeNote.IdleThreshold)
	if err != nil || threshold <= 0 {
		return 0, "", fmt.Errorf("agent spec: resume_note.idle_threshold must be a positive duration")
	}
	if strings.TrimSpace(spec.ResumeNote.Text) == "" {
		return 0, "", fmt.Errorf("agent spec: resume_note.text is required")
	}
	return threshold, spec.ResumeNote.Text, nil
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

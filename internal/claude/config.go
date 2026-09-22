package claude

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/agy"
)

type Config struct {
	Dir                string               `yaml:"-"`
	FS                 fs.FS                `yaml:"-"`
	TargetDir          string               `yaml:"target_dir"`
	Docs               []string             `yaml:"docs"`
	DocsProfiles       map[string][]string  `yaml:"docs_profiles"`
	Model              string               `yaml:"model"`
	Effort             string               `yaml:"effort"`
	Verbs              []string             `yaml:"verbs"`
	Permissions        Permissions          `yaml:"permissions"`
	Hooks              []Hook               `yaml:"hooks"`
	Env                map[string]string    `yaml:"env"`
	MCPServers         []MCPServer          `yaml:"mcp_servers"`
	StatusLine         bool                 `yaml:"status_line"`
	Commands           []Command            `yaml:"commands"`
	Skills             []Command            `yaml:"skills"`
	DistillAutopipe    DistillAutopipe      `yaml:"distill_autopipe"`
	SkillsTarget       string               `yaml:"skills_target"`
	CodexSkillsTarget  string               `yaml:"codex_skills_target"`
	CodexHooksTarget   string               `yaml:"codex_hooks_target"`
	AgyHooksTarget     string               `yaml:"agy_hooks_target"`
	ClaudeSkillsTarget string               `yaml:"claude_skills_target"`
	PrimeAgentTarget   string               `yaml:"prime_agent_target"`
	AgyTarget          string               `yaml:"agy_target"`
	AgentsMD           AgentsMD             `yaml:"agents_md"`
	Make               MakeConfig           `yaml:"make"`
	Feedback           FeedbackConfig       `yaml:"feedback"`
	Exec               ExecConfig           `yaml:"exec"`
	Debloat            DebloatConfig        `yaml:"debloat"`
	Decommissioned     DecommissionedConfig `yaml:"decommissioned"`
}

type ExecConfig struct {
	Timeout string `yaml:"timeout"`
}

// DecommissionedConfig lists artifacts that older harnez versions installed
// and current `apply` runs must remove after a rename or retirement.
type DecommissionedConfig struct {
	Commands []string `yaml:"commands"`
	Skills   []string `yaml:"skills"`
}

// DebloatConfig is the single source of truth for `apply --debloat`'s
// preset content (issues 316 and 348): which tool names each preset denies
// and which safe boolean settings it enables. Go code must read these
// values rather than hardcoding preset membership (see
// docs/Spec.md — config.yaml is this project's spec for apply-related
// settings, the same role spec/*.yaml plays for the usage dashboard).
type DebloatConfig struct {
	// CodexFeatures are the only Codex [features] keys managed by either
	// debloat preset. Unlisted Codex settings remain untouched.
	CodexFeatures map[string]bool `yaml:"codex_features"`
	// Agy contains Antigravity tool lists for debloat presets (issue 372).
	Agy agy.DebloatConfig `yaml:"agy"`
	// PresetDisableBundledSkills disables Claude Code's bundled skill
	// catalogue under any selected preset while leaving user/project skills.
	PresetDisableBundledSkills bool `yaml:"preset_disable_bundled_skills"`
	// PresetSkillOverrides apply under either preset. AggressiveSkillOverrides
	// are added only for the explicitly selected aggressive preset.
	PresetSkillOverrides     map[string]string `yaml:"preset_skill_overrides"`
	AggressiveSkillOverrides map[string]string `yaml:"aggressive_skill_overrides"`
	// MinimalDeny are verified integration-only tools denied under any
	// preset ("minimal" and "aggressive" both include these).
	MinimalDeny []string `yaml:"minimal_deny"`
	// AggressiveExtraDeny are interaction/safety-relevant tools denied only
	// under the explicit "aggressive" preset, on top of MinimalDeny.
	AggressiveExtraDeny []string `yaml:"aggressive_extra_deny"`
	// CronDeny/NotebookDeny are separately opt-in via their own flags,
	// independent of preset choice.
	CronDeny     []string `yaml:"cron_deny"`
	NotebookDeny []string `yaml:"notebook_deny"`
}

// FeedbackConfig steers the Tool Feedback Protocol instruction injection
// (the `harnez rate` call-to-action rendered into agents_md.global.sections
// and the tool-feedback-protocol skill) — see issue 142. It deliberately
// does not touch any other telemetry: `harnez rate`, `harnez exec`, and
// `harnez stats` keep working unconditionally regardless of this flag; it
// only stops `harnez apply` from (re-)injecting the instruction that tells
// agents to call `harnez rate` in the first place.
type FeedbackConfig struct {
	// DisableRateProtocol, when true, makes `harnez apply` omit (and
	// actively remove, if already installed) every agents_md.global.sections
	// entry and skill marked `rate_feedback: true` in config.yaml. Also
	// settable via the HARNEZ_DISABLE_RATE_FEEDBACK env var (any value other
	// than "", "0", "false", "no", "off" counts as true) for a quick local
	// override that doesn't require editing config.yaml; the env var wins
	// only in the sense that either source being true disables it — there is
	// no way to force it back on via env once config.yaml disables it.
	DisableRateProtocol bool `yaml:"disable_rate_protocol"`
}

// RateFeedbackDisabled reports whether the Tool Feedback Protocol
// instruction should be omitted (config flag OR env var — see
// FeedbackConfig's doc comment). getenv is injected for tests; nil means
// os.Getenv.
func RateFeedbackDisabled(cfg *Config, getenv func(string) string) bool {
	if cfg.Feedback.DisableRateProtocol {
		return true
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	switch strings.ToLower(strings.TrimSpace(getenv("HARNEZ_DISABLE_RATE_FEEDBACK"))) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

// MakeConfig steers how harnez reconciles its own targets (e.g. `help`)
// and the ⚙️ phony sentinel into a project's existing Makefile.
type MakeConfig struct {
	// PhonyFix controls how existing .PHONY declarations are handled:
	//   "ours" (default) — only ensure our own sentinel .PHONY line exists
	//   "all"             — also collapse other explicit .PHONY target lists
	//                       into the sentinel convention
	//   "none"            — never touch .PHONY lines
	PhonyFix string `yaml:"phony_fix"`
	// PhonySentinel overrides the sentinel token (default "⚙️"). If unset,
	// harnez auto-detects an existing sentinel variant in the file
	// (e.g. "⚙︎", the text-presentation form) before falling back to the default.
	PhonySentinel string `yaml:"phony_sentinel"`
}

func (m MakeConfig) phonyFixOrDefault() string {
	if m.PhonyFix == "" {
		return "ours"
	}
	return m.PhonyFix
}

type Permissions struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
}

type Hook struct {
	Event   string `yaml:"event"`
	Matcher string `yaml:"matcher,omitempty"`
	Command string `yaml:"command"`
}

type MCPServer struct {
	Name    string            `yaml:"name"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
}

type DistillAutopipe struct {
	PiExtensionTarget    string `yaml:"pi_extension_target"`
	OpenCodePluginTarget string `yaml:"opencode_plugin_target"`
}

type Command struct {
	Name        string          `yaml:"name"`
	Description string          `yaml:"description"`
	Content     string          `yaml:"content"`
	File        string          `yaml:"file"` // path relative to config dir; overrides content if set
	Resources   []SkillResource `yaml:"resources,omitempty"`
	// RateFeedback marks this skill as part of the Tool Feedback Protocol
	// instruction (issue 142): `harnez apply` omits/removes it when
	// RateFeedbackDisabled(cfg) is true.
	RateFeedback bool `yaml:"rate_feedback,omitempty"`
}

// SkillResource is a supporting file copied beside an installed skill.
// Source is resolved from the embedded/config filesystem; Target is relative
// to the installed skill directory.
type SkillResource struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

type AgentsMD struct {
	Global    AgentsMDTarget            `yaml:"global"`
	Local     AgentsMDTarget            `yaml:"local"`
	Agents    map[string]AgentsMDTarget `yaml:"agents"`
	Languages map[string]Language       `yaml:"languages"`
	RepoModes map[string]RepoMode       `yaml:"repo_modes"`
}

type AgentsMDTarget struct {
	Target   string      `yaml:"target"`
	Symlink  string      `yaml:"symlink"`
	Template string      `yaml:"template"`
	Content  string      `yaml:"content"`
	Sections []MDSection `yaml:"sections"`
}

type MDSection struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
	// RateFeedback marks this section as part of the Tool Feedback Protocol
	// instruction (issue 142): `harnez apply` omits/removes it when
	// RateFeedbackDisabled(cfg) is true.
	RateFeedback bool `yaml:"rate_feedback,omitempty"`
}

// RepoMode describes a repo's git setup (solo/fork/team) as a short
// "Repo Setup" section injected into a project's AGENTS.md via `init --repo-mode`.
type RepoMode struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}

type Language struct {
	Name   string `yaml:"name"`
	Ref    string `yaml:"ref"`
	Hint   string `yaml:"hint"`
	Source string `yaml:"source"`
	// LiteSource is an optional tagline-only variant of Source for capable
	// models tolerating compressed docs. SourceFor resolves between them.
	LiteSource string `yaml:"lite_source,omitempty"`
	Target     string `yaml:"target"`
	Local      string `yaml:"local"`
	Template   string `yaml:"template"` // scaffold file written once to project if absent
	Targets    string `yaml:"targets"`  // managed section injected into existing template file
	// Default controls whether init copies this doc without an explicit --doc flag.
	// "true" = always copy, "false" = only if explicitly requested, "auto" = detect from project.
	Default string `yaml:"default"`
	// DependsOn lists normative copied-doc dependencies by config name. Init
	// resolves these transitively; illustrative material must not be listed.
	DependsOn []string `yaml:"depends_on,omitempty"`
	// Capabilities documents the optional project capabilities this whole doc
	// governs. Capability docs remain explicit opt-ins; generic docs must phrase
	// capability-specific guidance conditionally rather than assume it applies.
	Capabilities []string `yaml:"capabilities,omitempty"`
}

// SourceFor returns LiteSource when variant is "lite" and LiteSource is set,
// otherwise it falls back to Source. Any other variant value (including "",
// the default) always resolves to Source.
func (l Language) SourceFor(variant string) string {
	if variant == "lite" && l.LiteSource != "" {
		return l.LiteSource
	}
	return l.Source
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.Dir = filepath.Dir(path)
	cfg.FS = os.DirFS(cfg.Dir)
	return &cfg, nil
}

func LoadConfigEmbedded() (*Config, error) {
	data, err := harnez.DefaultFS.ReadFile("config.yaml")
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	cfg.Dir = "."
	cfg.FS = harnez.DefaultFS
	return &cfg, nil
}

// ToolFeedbackProtocolBytes returns the combined byte size of every
// agents_md.global.sections entry and skill marked `rate_feedback: true` in
// config.yaml — the Tool Feedback Protocol instruction text (issue 122)
// that RateFeedbackDisabled lets `harnez apply` omit. It's a rough proxy
// for the one-time, per-session system-prompt cost of that instruction
// (injected once, not per `harnez rate` call) — see issue 142's overhead
// measurement, which pairs this with the per-call cost from
// telemetry.RateCallOverhead.
func ToolFeedbackProtocolBytes(cfg *Config) int {
	n := 0
	for _, s := range cfg.AgentsMD.Global.Sections {
		if s.RateFeedback {
			n += len(s.Content)
		}
	}
	for _, sk := range cfg.Skills {
		if sk.RateFeedback {
			n += len(sk.Content)
		}
	}
	return n
}

func DefaultTarget() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home + "/.claude"
}

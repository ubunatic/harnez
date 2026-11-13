package subagent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ToolCallLimitsSpec is the tool_call_limits section of spec/agent.yaml
// (issue 1033). An absent section means no limit anywhere.
type ToolCallLimitsSpec struct {
	Default             int                       `yaml:"default"`
	Models              map[string]map[string]int `yaml:"models"`
	NativeBackstopCalls int                       `yaml:"native_backstop_calls"`
	SynthesisPrompt     string                    `yaml:"synthesis_prompt"`
}

func (s ToolCallLimitsSpec) absent() bool {
	return s.Default == 0 && len(s.Models) == 0 && s.NativeBackstopCalls == 0 && s.SynthesisPrompt == ""
}

func (s ToolCallLimitsSpec) validate() error {
	if s.absent() {
		return nil
	}
	if s.Default < 0 {
		return fmt.Errorf("agent spec: tool_call_limits.default must not be negative")
	}
	if s.NativeBackstopCalls <= 0 {
		return fmt.Errorf("agent spec: tool_call_limits.native_backstop_calls must be positive")
	}
	if strings.TrimSpace(s.SynthesisPrompt) == "" {
		return fmt.Errorf("agent spec: tool_call_limits.synthesis_prompt is required")
	}
	for model, roles := range s.Models {
		if len(roles) == 0 {
			return fmt.Errorf("agent spec: tool_call_limits.models.%s is empty", model)
		}
		for role, limit := range roles {
			if limit < 0 {
				return fmt.Errorf("agent spec: tool_call_limits.models.%s.%s must not be negative", model, role)
			}
		}
	}
	return nil
}

// Limit returns the tool-call limit for a model and role (0 = off). Model
// keys are tried as in compact_thresholds: spec, provider:name, provider. The
// first matching model entry wins; inside it the role, then "default", and
// without either the section default.
func (s ToolCallLimitsSpec) Limit(model Model, role string) int {
	for _, key := range []string{model.Spec(), model.Provider + ":" + model.Name, model.Provider} {
		roles, ok := s.Models[key]
		if !ok {
			continue
		}
		if n, ok := roles[role]; ok {
			return n
		}
		if n, ok := roles["default"]; ok {
			return n
		}
		break
	}
	return s.Default
}

// ToolCallLimit returns the spec tool-call limit for a model and role.
func ToolCallLimit(model Model, role string) (int, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return 0, err
	}
	return spec.ToolCallLimits.Limit(model, role), nil
}

// ToolCallConfig holds the effective per-turn tool-call limit configuration
// after applying CLI flag, user config, and embedded spec precedence.
type ToolCallConfig struct {
	Limit               int
	NativeBackstopCalls int
	SynthesisPrompt     string
}

type userToolCallLimitsConfig struct {
	Agent struct {
		ToolCallLimits struct {
			Default             *int                      `yaml:"default"`
			Models              map[string]map[string]int `yaml:"models"`
			NativeBackstopCalls *int                      `yaml:"native_backstop_calls"`
			SynthesisPrompt     *string                   `yaml:"synthesis_prompt"`
		} `yaml:"tool_call_limits"`
	} `yaml:"agent"`
}

func matchUserModelLimit(models map[string]map[string]int, model Model, role string) (int, bool, error) {
	for _, key := range []string{model.Spec(), model.Provider + ":" + model.Name, model.Provider} {
		roles, ok := models[key]
		if !ok {
			continue
		}
		if n, ok := roles[role]; ok {
			if n < 0 {
				return 0, false, fmt.Errorf("agent.tool_call_limits.models.%s.%s must not be negative", key, role)
			}
			return n, true, nil
		}
		if n, ok := roles["default"]; ok {
			if n < 0 {
				return 0, false, fmt.Errorf("agent.tool_call_limits.models.%s.default must not be negative", key)
			}
			return n, true, nil
		}
		break
	}
	return 0, false, nil
}

// EffectiveToolCallConfig resolves the tool-call limit using precedence:
// --max-tool-calls N > ~/.harnez/config.yaml > spec/agent.yaml (each matching
// model then role).
func EffectiveToolCallConfig(model Model, role string, flagMax int, flagSet bool) (ToolCallConfig, error) {
	if flagSet && flagMax < 0 {
		return ToolCallConfig{}, fmt.Errorf("--max-tool-calls must not be negative")
	}
	spec, err := agentSpecOnce()
	if err != nil {
		return ToolCallConfig{}, err
	}
	out := ToolCallConfig{
		Limit:               spec.ToolCallLimits.Limit(model, role),
		NativeBackstopCalls: spec.ToolCallLimits.NativeBackstopCalls,
		SynthesisPrompt:     spec.ToolCallLimits.SynthesisPrompt,
	}
	if home, err := os.UserHomeDir(); err == nil {
		b, readErr := os.ReadFile(filepath.Join(home, ".harnez", "config.yaml"))
		if readErr != nil && !os.IsNotExist(readErr) {
			return ToolCallConfig{}, fmt.Errorf("read agent tool_call_limits config: %w", readErr)
		}
		if readErr == nil {
			var cfg userToolCallLimitsConfig
			if err := yaml.Unmarshal(b, &cfg); err != nil {
				return ToolCallConfig{}, fmt.Errorf("parse agent tool_call_limits config: %w", err)
			}
			tc := cfg.Agent.ToolCallLimits
			if tc.NativeBackstopCalls != nil {
				if *tc.NativeBackstopCalls <= 0 {
					return ToolCallConfig{}, fmt.Errorf("agent.tool_call_limits.native_backstop_calls must be positive")
				}
				out.NativeBackstopCalls = *tc.NativeBackstopCalls
			}
			if tc.SynthesisPrompt != nil {
				if strings.TrimSpace(*tc.SynthesisPrompt) == "" {
					return ToolCallConfig{}, fmt.Errorf("agent.tool_call_limits.synthesis_prompt must be non-empty")
				}
				out.SynthesisPrompt = *tc.SynthesisPrompt
			}
			if n, matched, err := matchUserModelLimit(tc.Models, model, role); err != nil {
				return ToolCallConfig{}, err
			} else if matched {
				out.Limit = n
			} else if tc.Default != nil {
				if *tc.Default < 0 {
					return ToolCallConfig{}, fmt.Errorf("agent.tool_call_limits.default must not be negative")
				}
				out.Limit = *tc.Default
			}
		}
	}
	if flagSet {
		out.Limit = flagMax
	}
	return out, nil
}

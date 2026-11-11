package subagent

import (
	"fmt"
	"strings"
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
// The --max-tool-calls flag and user config layers are not wired yet (1033 M1).
func ToolCallLimit(model Model, role string) (int, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return 0, err
	}
	return spec.ToolCallLimits.Limit(model, role), nil
}

package subagent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultCompactThreshold returns compact_threshold_tokens from the embedded
// agent spec, used when ~/.harnez/config.yaml sets no threshold.
func DefaultCompactThreshold() (int, error) {
	spec, err := agentSpecOnce()
	if err != nil {
		return 0, err
	}
	return spec.CompactThreshold, nil
}

// errCompactDisabled is returned when the resolved threshold is zero or
// negative. A zero is an intentional opt-out, not a failure, so callers can
// tell it apart from a real resolution error with errors.Is.
var errCompactDisabled = errors.New("agent compact threshold must be positive")

type compactConfig struct {
	Agent struct {
		CompactThresholdTokens int            `yaml:"compact_threshold_tokens"`
		CompactThresholds      map[string]int `yaml:"compact_thresholds"`
	} `yaml:"agent"`
}

// CompactThreshold prefers user model overrides, spec model defaults, the user
// global threshold, then the spec global default.
func CompactThreshold(model Model) (int, error) {
	var cfg compactConfig
	home, err := os.UserHomeDir()
	if err == nil {
		b, readErr := os.ReadFile(filepath.Join(home, ".harnez", "config.yaml"))
		if readErr != nil && !os.IsNotExist(readErr) {
			return 0, fmt.Errorf("read agent compact config: %w", readErr)
		}
		if readErr == nil {
			if err := yaml.Unmarshal(b, &cfg); err != nil {
				return 0, fmt.Errorf("parse agent compact config: %w", err)
			}
		}
	}
	spec, err := agentSpecOnce()
	if err != nil {
		return 0, err
	}
	keys := []string{model.Spec(), model.Provider + ":" + model.Name, model.Provider}
	threshold := 0
	for _, thresholds := range []map[string]int{cfg.Agent.CompactThresholds, spec.CompactThresholds} {
		for _, key := range keys {
			if n, ok := thresholds[key]; ok {
				threshold = n
				if threshold <= 0 {
					return 0, errCompactDisabled
				}
				return threshold, nil
			}
		}
	}
	threshold = cfg.Agent.CompactThresholdTokens
	if threshold == 0 {
		threshold = spec.CompactThreshold
	}
	if threshold <= 0 {
		return 0, errCompactDisabled
	}
	return threshold, nil
}

// VerifyCompaction requires an explicit compact acknowledgement and a smaller
// provider-reported input count before allowing another prompt to be sent.
func VerifyCompaction(before int, result *TurnResult) error {
	if result == nil {
		return fmt.Errorf("compaction returned no result")
	}
	ack := result.Response
	for _, message := range result.Messages {
		ack += "\n" + message
	}
	if !result.CompactionObserved && !strings.Contains(strings.ToLower(ack), "compact") {
		return fmt.Errorf("compaction completed without an acknowledgement")
	}
	after := result.ContextTokens
	if after <= 0 || after >= before {
		return fmt.Errorf("compaction completed without a verified context drop (before %d, after %d full input tokens)", before, after)
	}
	return nil
}

// EnsureContextUnderThreshold compacts only when the provider-reported full
// input size of the last turn exceeds the limit. Non-positive sizes (negative
// unknown or zero never-recorded) skip compaction.
func EnsureContextUnderThreshold(contextTokens, threshold int, compact func() (*TurnResult, error)) (bool, error) {
	if contextTokens <= 0 {
		return false, nil
	}
	if contextTokens < threshold {
		return false, nil
	}
	result, err := compact()
	if err != nil {
		return false, err
	}
	if err := VerifyCompaction(contextTokens, result); err != nil {
		return false, err
	}
	return true, nil
}

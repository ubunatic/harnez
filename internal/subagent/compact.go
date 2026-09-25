package subagent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultCompactThresholdTokens = 200000

type compactConfig struct {
	Agent struct {
		CompactThresholdTokens int            `yaml:"compact_threshold_tokens"`
		CompactThresholds      map[string]int `yaml:"compact_thresholds"`
	} `yaml:"agent"`
}

// CompactThreshold reads the global agent threshold, with an optional exact
// provider:model[:tier] override. Missing configuration uses the safe default.
func CompactThreshold(model Model) (int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultCompactThresholdTokens, nil
	}
	b, err := os.ReadFile(filepath.Join(home, ".harnez", "config.yaml"))
	if os.IsNotExist(err) {
		return DefaultCompactThresholdTokens, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read agent compact config: %w", err)
	}
	var cfg compactConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return 0, fmt.Errorf("parse agent compact config: %w", err)
	}
	threshold := cfg.Agent.CompactThresholdTokens
	if threshold == 0 {
		threshold = DefaultCompactThresholdTokens
	}
	for _, key := range []string{model.Spec(), model.Provider + ":" + model.Name, model.Provider} {
		if n, ok := cfg.Agent.CompactThresholds[key]; ok {
			threshold = n
			break
		}
	}
	if threshold <= 0 {
		return 0, fmt.Errorf("agent compact threshold must be positive")
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
	if !strings.Contains(strings.ToLower(ack), "compact") {
		return fmt.Errorf("compaction completed without an acknowledgement")
	}
	after := max(result.InputTokens-result.CachedTokens, 0)
	if after <= 0 || after >= before {
		return fmt.Errorf("compaction completed without a verified context drop (before %d, after %d uncached input tokens)", before, after)
	}
	return nil
}

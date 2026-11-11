package subagent

import (
	"encoding/json"
	"errors"
	"strings"
)

type ToolEnvelope struct {
	Tool  string          `json:"tool,omitempty"`
	Args  json.RawMessage `json:"args,omitempty"`
	Final string          `json:"final,omitempty"`
}

// Approach 1: Strict JSON parser
func ParseStrictEnvelope(input string) (*ToolEnvelope, error) {
	var env ToolEnvelope
	err := json.Unmarshal([]byte(strings.TrimSpace(input)), &env)
	if err != nil {
		return nil, err
	}
	if env.Tool == "" && env.Final == "" {
		return nil, errors.New("must provide either tool or final")
	}
	return &env, nil
}

// Approach 2: Lenient JSON parser (extract first JSON object)
func ParseLenientEnvelope(input string) (*ToolEnvelope, error) {
	start := strings.Index(input, "{")
	if start == -1 {
		return nil, errors.New("no json object found")
	}

	var env ToolEnvelope
	// try to find a valid JSON object starting from '{'
	for end := len(input); end > start; end-- {
		if input[end-1] == '}' {
			err := json.Unmarshal([]byte(input[start:end]), &env)
			if err == nil {
				if env.Tool == "" && env.Final == "" {
					continue // try a shorter substring maybe? Or maybe it's just invalid
				}
				return &env, nil
			}
		}
	}

	return nil, errors.New("could not parse json object")
}

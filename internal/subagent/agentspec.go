package subagent

import (
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

const agentSpecPath = "spec/agent.yaml"

type agentSpec struct {
	DefaultModel string `yaml:"default_model"`
}

func parseAgentSpec(data []byte) (agentSpec, error) {
	var spec agentSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return agentSpec{}, fmt.Errorf("agent spec: parse: %w", err)
	}
	if strings.TrimSpace(spec.DefaultModel) == "" {
		return agentSpec{}, fmt.Errorf("agent spec: default_model is required")
	}
	if _, err := ResolveModel(spec.DefaultModel); err != nil {
		return agentSpec{}, fmt.Errorf("agent spec: default_model: %w", err)
	}
	return spec, nil
}

func loadAgentSpec() (agentSpec, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, agentSpecPath)
	if err != nil {
		return agentSpec{}, fmt.Errorf("agent spec: read %s: %w", agentSpecPath, err)
	}
	return parseAgentSpec(data)
}

var agentSpecOnce = sync.OnceValues(loadAgentSpec)

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

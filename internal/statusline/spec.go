package statusline

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

const statuslineSpecPath = "spec/statusline.yaml"

type statuslineSpec struct {
	StatusLine struct {
		Claude string `yaml:"claude"`
		AGY    string `yaml:"agy"`
	} `yaml:"statusline"`
}

type lineData struct {
	Directory  string
	Context    contextData
	RateLimits []rateLimitData
	Quotas     []quotaData
}

type contextData struct {
	Available       bool
	TotalTokens     int
	Size            string
	WindowAvailable bool
	WindowSize      string
	CacheRead       int
	CachePercent    int
	HasCache        bool
}

type rateLimitData struct {
	Label            string
	RemainingPercent string
	Remaining        string
}

type quotaData struct {
	Name              string
	RemainingPercent  string
	RemainingFraction string
}

func renderTemplate(agent string, data lineData) (string, error) {
	specBytes, err := fs.ReadFile(harnez.DefaultFS, statuslineSpecPath)
	if err != nil {
		return "", fmt.Errorf("statusline: read spec: %w", err)
	}
	var spec statuslineSpec
	if err := yaml.Unmarshal(specBytes, &spec); err != nil {
		return "", fmt.Errorf("statusline: parse spec: %w", err)
	}
	var source string
	switch agent {
	case "claude":
		source = spec.StatusLine.Claude
	case "agy":
		source = spec.StatusLine.AGY
	default:
		return "", fmt.Errorf("statusline: unknown template %q", agent)
	}
	if source == "" {
		return "", fmt.Errorf("statusline: missing %s template in %s", agent, statuslineSpecPath)
	}
	tmpl, err := template.New(agent).Option("missingkey=error").Parse(source)
	if err != nil {
		return "", fmt.Errorf("statusline: parse %s template: %w", agent, err)
	}
	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("statusline: render %s template: %w", agent, err)
	}
	return strings.TrimSpace(rendered.String()), nil
}

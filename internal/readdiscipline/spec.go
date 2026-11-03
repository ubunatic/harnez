// Package readdiscipline loads the Reading & Context Discipline spec
// (spec/reading_discipline.yaml, issue 1079). The line threshold and the
// guidance text that cites it live only in the spec; Go code reads them here.
package readdiscipline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

const (
	specPath    = "spec/reading_discipline.yaml"
	schemaPath  = "spec/schemas/reading_discipline.schema.json"
	placeholder = "{max_lines}"
)

// Spec is the parsed reading-discipline spec. Templates use {max_lines}.
type Spec struct {
	MaxLines         int    `yaml:"max_lines"`
	DenyMessage      string `yaml:"deny_message"`
	RepeatMessage    string `yaml:"repeat_message"`
	SubagentGuidance string `yaml:"subagent_guidance"`
}

// Load parses and validates the embedded spec.
func Load() (Spec, error) {
	data, err := harnez.DefaultFS.ReadFile(specPath)
	if err != nil {
		return Spec{}, fmt.Errorf("read embedded %s: %w", specPath, err)
	}
	return Parse(data)
}

// Parse validates spec YAML against the embedded JSON Schema and decodes it.
// The schema is the single authority on shape; no Go-side checks repeat it.
func Parse(data []byte) (Spec, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Spec{}, fmt.Errorf("parse %s: %w", specPath, err)
	}
	if err := validate(doc); err != nil {
		return Spec{}, fmt.Errorf("validate %s: %w", specPath, err)
	}
	var s Spec
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return Spec{}, fmt.Errorf("decode %s: %w", specPath, err)
	}
	return s, nil
}

func validate(doc map[string]any) error {
	schemaData, err := harnez.DefaultFS.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return err
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return err
	}
	return resolved.Validate(doc)
}

// Deny returns the deny message with the threshold filled in.
func (s Spec) Deny() string {
	return render(s.DenyMessage, s.MaxLines)
}

// Repeat returns the deny message for a second native read of one file in a session.
func (s Spec) Repeat() string {
	return render(s.RepeatMessage, s.MaxLines)
}

// Guidance returns the subagent read guidance with the threshold filled in.
func (s Spec) Guidance() string {
	return render(s.SubagentGuidance, s.MaxLines)
}

func render(tmpl string, maxLines int) string {
	return strings.ReplaceAll(tmpl, placeholder, strconv.Itoa(maxLines))
}

var (
	cacheOnce sync.Once
	cached    Spec
	cachedErr error
)

// MustLoad returns the embedded spec, panicking if it is invalid. The embedded
// file is a build input, so an invalid spec is a programming error that the
// package tests catch before release.
func MustLoad() Spec {
	cacheOnce.Do(func() {
		cached, cachedErr = Load()
	})
	if cachedErr != nil {
		panic(fmt.Sprintf("harnez: embedded %s is invalid: %v", specPath, cachedErr))
	}
	return cached
}

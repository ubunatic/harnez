// Package bench is the optional benchmark harness behind `harnez bench`: it
// sends small specced tasks to agent CLIs under a documentation condition
// (lite/full docs, with or without PNG cards) and records scored results in
// a bench database that is separate from the telemetry store.
package bench

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Task is one specced bench task with a mechanically checkable response.
type Task struct {
	ID            string   `yaml:"id"`
	Rule          string   `yaml:"rule"`
	Prompt        string   `yaml:"prompt"`
	Pattern       string   `yaml:"pattern"`
	ForbidPattern string   `yaml:"forbid_pattern"`
	Docs          []string `yaml:"docs"`
}

// Spec is the parsed task file.
type Spec struct {
	Preamble string   `yaml:"preamble"`
	BaseDocs []string `yaml:"base_docs"`
	Tasks    []Task   `yaml:"tasks"`
}

//go:embed tasks.yaml
var embeddedTasks []byte

// LoadSpec parses the embedded task spec.
func LoadSpec() (*Spec, error) { return ParseSpec(embeddedTasks) }

// ParseSpec parses and validates a task spec: unique IDs, compilable RE2
// patterns, and at least one check per task other than the smoke task.
func ParseSpec(data []byte) (*Spec, error) {
	var s Spec
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("bench: parse tasks: %w", err)
	}
	seen := map[string]bool{}
	for _, t := range s.Tasks {
		if t.ID == "" || strings.TrimSpace(t.Prompt) == "" {
			return nil, fmt.Errorf("bench: task %q needs an id and a prompt", t.ID)
		}
		if seen[t.ID] {
			return nil, fmt.Errorf("bench: duplicate task id %q", t.ID)
		}
		seen[t.ID] = true
		if t.Pattern == "" && t.ForbidPattern == "" {
			return nil, fmt.Errorf("bench: task %q has no pattern or forbid_pattern", t.ID)
		}
		for _, p := range []string{t.Pattern, t.ForbidPattern} {
			if p == "" {
				continue
			}
			if _, err := regexp.Compile("(?i)" + p); err != nil {
				return nil, fmt.Errorf("bench: task %q pattern %q: %w", t.ID, p, err)
			}
		}
	}
	return &s, nil
}

// Select returns the tasks with the given IDs in spec order, or all tasks
// when ids is empty. Unknown IDs are an error.
func (s *Spec) Select(ids []string) ([]Task, error) {
	if len(ids) == 0 {
		return s.Tasks, nil
	}
	byID := map[string]Task{}
	for _, t := range s.Tasks {
		byID[t.ID] = t
	}
	var out []Task
	for _, id := range ids {
		t, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("bench: unknown task %q", id)
		}
		out = append(out, t)
	}
	return out, nil
}

// Score checks a response against the task's patterns.
func (t Task) Score(response string) (pass bool, detail string) {
	if strings.TrimSpace(response) == "" {
		return false, "empty response"
	}
	if t.Pattern != "" && !regexp.MustCompile("(?i)"+t.Pattern).MatchString(response) {
		return false, fmt.Sprintf("pattern %q not found", t.Pattern)
	}
	if t.ForbidPattern != "" && regexp.MustCompile("(?i)"+t.ForbidPattern).MatchString(response) {
		return false, fmt.Sprintf("forbid_pattern %q matched", t.ForbidPattern)
	}
	return true, ""
}

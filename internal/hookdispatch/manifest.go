package hookdispatch

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// Manifest is one host agent's hook table from spec/hooks.yaml: the command
// prefix each host entry runs and, per event, the handlers in run order. The
// host settings are derived from it, so events, matchers and commands cannot
// drift from the handlers. cmd/harnez binds the run functions with Bind.
type Manifest struct {
	// Command is the host command prefix. An entry runs Command + " " + event.
	Command string
	Events  map[string]Event
}

// Event is one host event: the handlers it runs and the matcher it writes.
type Event struct {
	// Matcher is an explicit host matcher, for events whose matcher is not a tool
	// list (Codex SessionStart). Empty derives the matcher from the handlers' Tools.
	Matcher  string
	Handlers []Handler
}

// Entry is one host hook entry: the event, the matcher ("" for none) and the command.
type Entry struct {
	Event   string
	Matcher string
	Command string
}

// specFile mirrors the agents part of spec/hooks.yaml.
type specFile struct {
	Agents map[string]specAgent `yaml:"agents"`
}

type specAgent struct {
	Command string               `yaml:"command"`
	Events  map[string]specEvent `yaml:"events"`
}

type specEvent struct {
	Matcher  string        `yaml:"matcher"`
	Handlers []specHandler `yaml:"handlers"`
}

type specHandler struct {
	Name  string   `yaml:"name"`
	Kind  string   `yaml:"kind"`
	Tools []string `yaml:"tools"`
}

// LoadManifest returns the hook manifest for agent ("claude" or "codex") from
// the embedded spec/hooks.yaml.
func LoadManifest(agent string) (Manifest, error) {
	raw, err := harnez.DefaultFS.ReadFile("spec/hooks.yaml")
	if err != nil {
		return Manifest{}, fmt.Errorf("load hooks spec: %w", err)
	}
	return ParseManifest(raw, agent)
}

// ParseManifest validates raw against spec/schemas/hooks.schema.json and returns
// the manifest for agent. Handler run functions are not set; Bind adds them.
func ParseManifest(raw []byte, agent string) (Manifest, error) {
	if err := validateSpec(raw); err != nil {
		return Manifest{}, fmt.Errorf("hooks spec: %w", err)
	}
	var file specFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return Manifest{}, fmt.Errorf("parse hooks spec: %w", err)
	}
	a, ok := file.Agents[agent]
	if !ok {
		return Manifest{}, fmt.Errorf("hooks spec: no agent %q", agent)
	}
	m := Manifest{Command: a.Command, Events: map[string]Event{}}
	for name, se := range a.Events {
		ev := Event{Matcher: se.Matcher}
		for _, sh := range se.Handlers {
			kind, err := parseKind(sh.Kind)
			if err != nil {
				return Manifest{}, fmt.Errorf("hooks spec %s %s/%s: %w", agent, name, sh.Name, err)
			}
			ev.Handlers = append(ev.Handlers, Handler{Name: sh.Name, Kind: kind, Tools: sh.Tools})
		}
		m.Events[name] = ev
	}
	return m, nil
}

func parseKind(s string) (Kind, error) {
	switch s {
	case "json":
		return JSON, nil
	case "silent":
		return Silent, nil
	case "text":
		return Text, nil
	}
	return 0, fmt.Errorf("unknown handler kind %q", s)
}

// EventNames returns the manifest's event names in sorted order.
func (m Manifest) EventNames() []string {
	return sortedKeys(m.Events)
}

// Entries returns one entry per event, in event order. The matcher is the
// explicit Matcher if set; otherwise the union of the handlers' Tools in run
// order, joined with "|", and empty when no handler declares tools. An event
// that mixes handlers with and without tools is rejected, because the matcher
// would skip the untooled ones. An explicit matcher cannot be combined with tools.
func (m Manifest) Entries() ([]Entry, error) {
	var out []Entry
	for _, event := range m.EventNames() {
		ev := m.Events[event]
		var tools []string
		tooled, untooled := 0, 0
		for _, h := range ev.Handlers {
			if len(h.Tools) == 0 {
				untooled++
				continue
			}
			tooled++
			for _, tool := range h.Tools {
				if !slices.Contains(tools, tool) {
					tools = append(tools, tool)
				}
			}
		}
		if ev.Matcher != "" && tooled > 0 {
			return nil, fmt.Errorf("hook event %s sets a matcher and handler tools; use one", event)
		}
		if tooled > 0 && untooled > 0 {
			return nil, fmt.Errorf("hook event %s mixes handlers with and without tools; a matcher would skip the untooled ones", event)
		}
		matcher := ev.Matcher
		if matcher == "" {
			matcher = strings.Join(tools, "|")
		}
		out = append(out, Entry{Event: event, Matcher: matcher, Command: m.Command + " " + event})
	}
	return out, nil
}

// Bind returns the registry for m. runs maps "Event/Name" to the handler body.
// Bind fails if a manifest handler has no body or a body has no manifest
// handler. m itself is not changed.
func (m Manifest) Bind(runs map[string]Func) (Registry, error) {
	want := map[string]bool{}
	reg := Registry{}
	for _, event := range m.EventNames() {
		for _, h := range m.Events[event].Handlers {
			key := event + "/" + h.Name
			want[key] = true
			run, ok := runs[key]
			if !ok {
				return nil, fmt.Errorf("hook manifest %s: no Run bound", key)
			}
			h.Run = run
			reg[event] = append(reg[event], h)
		}
	}
	for _, key := range sortedKeys(runs) {
		if !want[key] {
			return nil, fmt.Errorf("hook Run %s has no manifest handler", key)
		}
	}
	if err := reg.Validate(); err != nil {
		return nil, err
	}
	return reg, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

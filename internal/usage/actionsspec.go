package usage

import (
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// actionsSpecPath is the embedded location of the hotkey registry (issue
// 132). watch.go's key dispatch and title/hint rendering read the
// key->action->symbol mapping from here rather than a hardcoded Go
// switch-to-symbol table — see docs/Spec.md.
const actionsSpecPath = "spec/actions.yaml"

// watchAction is one entry in spec/actions.yaml: the key(s) that trigger it,
// how it's labeled, and how it's shown in the TUI. Mirrors
// spec/schemas/actions.schema.json field-for-field.
type watchAction struct {
	Title       string   `yaml:"title"`
	Description string   `yaml:"description,omitempty"`
	Keys        []string `yaml:"keys"`
	Symbol      string   `yaml:"symbol"`
	Category    string   `yaml:"category"`
	Box         string   `yaml:"box,omitempty"`
}

// watchActionsSpec is the top-level shape of spec/actions.yaml.
type watchActionsSpec struct {
	Actions map[string]watchAction `yaml:"actions"`
}

var validActionCategories = map[string]bool{
	"box":     true,
	"data":    true,
	"mode":    true,
	"session": true,
}

// validBoxIDs mirrors spec/schemas/actions.schema.json's "box" enum: the
// watchSections fields a category:box action is allowed to name.
var validBoxIDs = map[string]bool{
	"all_usage": true,
	"claude":    true,
	"agy":       true,
	"codex":     true,
	"history":   true,
	"processes": true,
	"load":      true,
	"mic":       true,
}

// decodeSpecKey turns one spec key token into the byte RunWatchWithOptions's
// tty reader compares keypresses against. Single-character tokens map
// directly; the two bracketed tokens name non-printable control keys.
func decodeSpecKey(tok string) (byte, bool) {
	switch tok {
	case "<c-c>":
		return 3, true
	case "<esc>":
		return 27, true
	}
	if len(tok) == 1 {
		return tok[0], true
	}
	return 0, false
}

// parseWatchActionsYAML parses and validates spec/actions.yaml content. It
// returns a clear error (never panics) for malformed YAML or a spec that
// violates the schema's invariants: missing required fields, an unknown
// category, or a category:box action without a recognized box id — issue
// 132's "fail clearly, not panic" acceptance criterion.
func parseWatchActionsYAML(data []byte) (watchActionsSpec, error) {
	var spec watchActionsSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return watchActionsSpec{}, fmt.Errorf("actions spec: parse: %w", err)
	}
	if len(spec.Actions) == 0 {
		return watchActionsSpec{}, fmt.Errorf("actions spec: no actions defined")
	}
	for name, a := range spec.Actions {
		if strings.TrimSpace(a.Title) == "" {
			return watchActionsSpec{}, fmt.Errorf("actions spec: action %q: missing title", name)
		}
		if len(a.Keys) == 0 {
			return watchActionsSpec{}, fmt.Errorf("actions spec: action %q: no keys defined", name)
		}
		for _, k := range a.Keys {
			if _, ok := decodeSpecKey(k); !ok {
				return watchActionsSpec{}, fmt.Errorf("actions spec: action %q: unrecognized key token %q", name, k)
			}
		}
		if strings.TrimSpace(a.Symbol) == "" {
			return watchActionsSpec{}, fmt.Errorf("actions spec: action %q: missing symbol", name)
		}
		if !validActionCategories[a.Category] {
			return watchActionsSpec{}, fmt.Errorf("actions spec: action %q: invalid category %q", name, a.Category)
		}
		if a.Category == "box" {
			if a.Box == "" {
				return watchActionsSpec{}, fmt.Errorf("actions spec: action %q: category box requires a box id", name)
			}
			if !validBoxIDs[a.Box] {
				return watchActionsSpec{}, fmt.Errorf("actions spec: action %q: unknown box id %q", name, a.Box)
			}
		} else if a.Box != "" {
			return watchActionsSpec{}, fmt.Errorf("actions spec: action %q: box id set on non-box category %q", name, a.Category)
		}
	}
	return spec, nil
}

// watchActions is the parsed, indexed form of spec/actions.yaml that
// watch.go's dispatch and rendering code query at runtime.
type watchActions struct {
	spec      watchActionsSpec
	keyAction map[byte]string // keypress -> action name
	boxSymbol map[string]string
}

func buildWatchActions(spec watchActionsSpec) *watchActions {
	wa := &watchActions{
		spec:      spec,
		keyAction: map[byte]string{},
		boxSymbol: map[string]string{},
	}
	for name, a := range spec.Actions {
		for _, k := range a.Keys {
			if b, ok := decodeSpecKey(k); ok {
				wa.keyAction[b] = name
			}
		}
		if a.Category == "box" {
			wa.boxSymbol[a.Box] = a.Symbol
		}
	}
	return wa
}

// loadWatchActions reads and validates the embedded spec/actions.yaml.
func loadWatchActions() (*watchActions, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, actionsSpecPath)
	if err != nil {
		return nil, fmt.Errorf("actions spec: read %s: %w", actionsSpecPath, err)
	}
	spec, err := parseWatchActionsYAML(data)
	if err != nil {
		return nil, err
	}
	return buildWatchActions(spec), nil
}

var watchActionsOnce = sync.OnceValues(loadWatchActions)

// mustWatchActions returns the parsed embedded actions spec. It is expected
// to always succeed — spec/actions.yaml is compiled into the binary and
// covered by TestEmbeddedActionsSpecIsValid — so a failure here means the
// binary itself was built with a broken spec, not a runtime/user condition.
func mustWatchActions() *watchActions {
	wa, err := watchActionsOnce()
	if err != nil {
		panic(fmt.Sprintf("harnez usage: embedded %s is invalid: %v", actionsSpecPath, err))
	}
	return wa
}

// actionForKey returns the action name bound to key, or "" if key is not
// mapped to anything in the spec.
func (wa *watchActions) actionForKey(key byte) string {
	return wa.keyAction[key]
}

// symbol returns the display symbol for a named action ("" if unknown).
func (wa *watchActions) symbol(action string) string {
	return wa.spec.Actions[action].Symbol
}

// boxTitleSymbol returns the superscript (or other) symbol shown next to a
// box's title for the box-toggle action that controls boxID's visibility.
func (wa *watchActions) boxTitleSymbol(boxID string) string {
	return wa.boxSymbol[boxID]
}

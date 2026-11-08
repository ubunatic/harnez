// Package disable reads and writes the owner overrides behind the /disable
// and /enable skills (issue 1188). Each suspended item is one YAML file under
// the spec's state_dir; it applies only in the session that wrote it.
package disable

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// Spec mirrors spec/disable.yaml.
type Spec struct {
	StateDir   string     `yaml:"state_dir"`
	SessionEnv []string   `yaml:"session_env"`
	Items      []SpecItem `yaml:"items"`
	Never      []string   `yaml:"never"`
}

// SpecItem is one suspendable item. Item "*" accepts any name in its class.
type SpecItem struct {
	Class       string `yaml:"class"`
	Item        string `yaml:"item"`
	Description string `yaml:"description"`
}

// State is one suspended item on disk.
type State struct {
	Class     string    `yaml:"class"`
	Item      string    `yaml:"item"`
	SetAt     time.Time `yaml:"set_at"`
	SessionID string    `yaml:"session_id"`
	Reason    string    `yaml:"reason,omitempty"`
}

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// LoadSpec parses the embedded spec/disable.yaml.
func LoadSpec() (Spec, error) {
	b, err := harnez.DefaultFS.ReadFile("spec/disable.yaml")
	if err != nil {
		return Spec{}, fmt.Errorf("load disable spec: %w", err)
	}
	var s Spec
	if err := yaml.Unmarshal(b, &s); err != nil {
		return Spec{}, fmt.Errorf("parse disable spec: %w", err)
	}
	if s.StateDir == "" || len(s.SessionEnv) == 0 || len(s.Items) == 0 || len(s.Never) == 0 {
		return Spec{}, errors.New("disable spec: state_dir, session_env, items and never are required")
	}
	if filepath.IsAbs(s.StateDir) || !filepath.IsLocal(filepath.FromSlash(s.StateDir)) {
		return Spec{}, fmt.Errorf("disable spec: state_dir %q must be a relative path inside the repo", s.StateDir)
	}
	return s, nil
}

// SessionID returns the first non-empty session variable from the spec.
func (s Spec) SessionID(getenv func(string) string) string {
	if getenv == nil {
		getenv = os.Getenv
	}
	for _, name := range s.SessionEnv {
		if v := strings.TrimSpace(getenv(name)); v != "" {
			return v
		}
	}
	return ""
}

// Known reports whether class/item is a suspendable item in the spec.
func (s Spec) Known(class, item string) bool {
	if !namePattern.MatchString(class) || !namePattern.MatchString(item) {
		return false
	}
	for _, it := range s.Items {
		if it.Class == class && (it.Item == item || it.Item == "*") {
			return true
		}
	}
	return false
}

// Path is the state file for class/item under repo root.
func (s Spec) Path(root, class, item string) string {
	return filepath.Join(root, filepath.FromSlash(s.StateDir), class+"-"+item+".yaml")
}

// Write records a suspension. It refuses items the spec does not list.
func (s Spec) Write(root string, st State) error {
	if !s.Known(st.Class, st.Item) {
		return fmt.Errorf("disable: %s %s is not a suspendable item", st.Class, st.Item)
	}
	path := s.Path(root, st.Class, st.Item)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("disable: %w", err)
	}
	b, err := yaml.Marshal(st)
	if err != nil {
		return fmt.Errorf("disable: %w", err)
	}
	return os.WriteFile(path, b, 0o644)
}

// Remove deletes the state for class/item. A missing file is not an error.
func (s Spec) Remove(root, class, item string) error {
	if !namePattern.MatchString(class) || !namePattern.MatchString(item) {
		return fmt.Errorf("enable: invalid item %q %q", class, item)
	}
	err := os.Remove(s.Path(root, class, item))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("enable: %w", err)
	}
	return nil
}

// List returns all recorded states, sorted by file name. Unreadable or
// malformed files are skipped.
func (s Spec) List(root string) ([]State, error) {
	dir := filepath.Join(root, filepath.FromSlash(s.StateDir))
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("disable: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var out []State
	for _, name := range names {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var st State
		if yaml.Unmarshal(b, &st) != nil || st.Class == "" || st.Item == "" {
			continue
		}
		out = append(out, st)
	}
	return out, nil
}

// Status is the result of checking one item.
type Status struct {
	Applied bool   // recorded in this session: the rule is suspended
	Stale   bool   // recorded in another (or unknown) session: the rule is enforced
	State   *State // the recorded state, nil if none
}

// Check reports whether class/item is suspended for the current session.
func (s Spec) Check(root, class, item string, getenv func(string) string) Status {
	if !namePattern.MatchString(class) || !namePattern.MatchString(item) {
		return Status{}
	}
	b, err := os.ReadFile(s.Path(root, class, item))
	if err != nil {
		return Status{}
	}
	var st State
	if yaml.Unmarshal(b, &st) != nil || st.Class != class || st.Item != item {
		return Status{}
	}
	current := s.SessionID(getenv)
	if current != "" && st.SessionID == current {
		return Status{Applied: true, State: &st}
	}
	return Status{Stale: true, State: &st}
}

package repocfg

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez/internal/gitflow"
	"ubunatic.com/harnez/internal/issues"
	"ubunatic.com/harnez/internal/repospec"
)

// Entry is the effective value of one key.
type Entry struct {
	Key     Key
	Value   string // the value as `harnez config <key>` prints it
	Set     bool   // false: not in repo.yaml and no spec default
	Default bool   // Value comes from a spec default, not from repo.yaml
}

// String renders the `--list` line: key=value, plus " (default)" for a spec
// default.
func (e Entry) String() string {
	s := e.Key.Path + "=" + e.Value
	if e.Default {
		s += " (default)"
	}
	return s
}

// Get returns the effective value of k for the repo at dir: the repo.yaml
// value, else the spec default, else an Entry with Set false. A missing
// repo.yaml is not an error; it just means defaults only.
func Get(dir string, k Key) (Entry, error) {
	doc, err := readDocument(dir)
	if err != nil {
		return Entry{}, err
	}
	if v, ok := lookupValue(doc, k.Path); ok {
		return Entry{Key: k, Value: formatValue(v), Set: true}, nil
	}
	value, ok, err := specDefault(dir, k.Path)
	if err != nil || !ok {
		return Entry{Key: k}, err
	}
	return Entry{Key: k, Value: value, Set: true, Default: true}, nil
}

// List returns the effective entries that have a value, sorted by key.
func List(dir string) ([]Entry, error) {
	keys, err := Keys()
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, k := range keys {
		e, err := Get(dir, k)
		if err != nil {
			return nil, err
		}
		if e.Set {
			out = append(out, e)
		}
	}
	return out, nil
}

func readDocument(dir string) (map[string]any, error) {
	data, err := os.ReadFile(repoPath(dir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", repoPath(dir), err)
	}
	return doc, nil
}

func lookupValue(doc map[string]any, path string) (any, bool) {
	var cur any = doc
	for _, seg := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[seg]; !ok {
			return nil, false
		}
	}
	return cur, true
}

func formatValue(v any) string {
	if list, ok := v.([]any); ok {
		items := make([]string, len(list))
		for i, item := range list {
			items[i] = fmt.Sprint(item)
		}
		return strings.Join(items, ",")
	}
	return fmt.Sprint(v)
}

// specDefault resolves a key left out of repo.yaml from the file that owns its
// default: spec/git.yaml (through gitflow), spec/issues.yaml, or
// spec/repo-config.yaml.
func specDefault(dir, path string) (value string, ok bool, err error) {
	switch path {
	case "git.mode", "git.protected_main":
		cfg, err := gitflow.Load(dir)
		if err != nil {
			return "", false, err
		}
		if path == "git.mode" {
			return string(cfg.Mode), true, nil
		}
		return strconv.FormatBool(cfg.ProtectedMain), true, nil
	case "issues.active_cap":
		n, err := issues.ActiveIssueCap()
		return strconv.Itoa(n), err == nil, err
	}
	spec, err := repospec.Load()
	if err != nil {
		return "", false, err
	}
	if b, found := spec.BoolDefault(path); found {
		return strconv.FormatBool(b), true, nil
	}
	return "", false, nil
}

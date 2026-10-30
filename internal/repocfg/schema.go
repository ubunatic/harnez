// Package repocfg reads and edits the keys of .harnez/repo.yaml for
// `harnez config`. The key list, types and enums come from the embedded
// spec/schemas/repo.schema.json, and which keys need `harnez init` after a
// change comes from spec/repo-config.yaml; neither is copied into Go.
package repocfg

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"ubunatic.com/harnez"
)

// Kind is the value type of a key.
type Kind int

const (
	KindString Kind = iota
	KindBool
	KindInt
	KindStringList
)

// Key describes one leaf of repo.yaml.
type Key struct {
	Path      string   // dotted path, for example "git.mode"
	Kind      Kind     // value type
	Enum      []string // allowed values (a const counts as a single value)
	Min       *float64 // inclusive minimum of an int key
	NonEmpty  bool     // a string key with minLength >= 1
	Required  bool     // listed in its parent's "required"
	itemEmpty bool     // list items must be non-empty
}

var loadKeys = sync.OnceValues(func() ([]Key, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/repo.schema.json")
	if err != nil {
		return nil, fmt.Errorf("read repo schema: %w", err)
	}
	var root jsonschema.Schema
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse repo schema: %w", err)
	}
	var keys []Key
	if err := walk(&root, "", true, &keys); err != nil {
		return nil, err
	}
	slices.SortFunc(keys, func(a, b Key) int { return strings.Compare(a.Path, b.Path) })
	return keys, nil
})

func walk(s *jsonschema.Schema, prefix string, required bool, out *[]Key) error {
	if len(s.Properties) > 0 {
		for name, child := range s.Properties {
			path := name
			if prefix != "" {
				path = prefix + "." + name
			}
			if err := walk(child, path, slices.Contains(s.Required, name), out); err != nil {
				return err
			}
		}
		return nil
	}
	k := Key{Path: prefix, Required: required, Min: s.Minimum}
	switch s.Type {
	case "boolean":
		k.Kind = KindBool
	case "integer":
		k.Kind = KindInt
	case "string":
		k.Kind = KindString
		k.NonEmpty = s.MinLength != nil && *s.MinLength > 0
	case "array":
		if s.Items == nil || s.Items.Type != "string" {
			return fmt.Errorf("repo schema: %s: only arrays of strings are supported", prefix)
		}
		k.Kind = KindStringList
		k.itemEmpty = s.Items.MinLength != nil && *s.Items.MinLength > 0
	default:
		return fmt.Errorf("repo schema: %s: unsupported type %q", prefix, s.Type)
	}
	for _, v := range s.Enum {
		k.Enum = append(k.Enum, fmt.Sprint(v))
	}
	if s.Const != nil {
		k.Enum = append(k.Enum, fmt.Sprint(*s.Const))
	}
	*out = append(*out, k)
	return nil
}

// Keys returns every settable key of repo.yaml, sorted by dotted path.
func Keys() ([]Key, error) { return loadKeys() }

// Lookup finds a key by dotted path. The error for an unknown path lists the
// valid ones.
func Lookup(path string) (Key, error) {
	keys, err := Keys()
	if err != nil {
		return Key{}, err
	}
	for _, k := range keys {
		if k.Path == path {
			return k, nil
		}
	}
	names := make([]string, len(keys))
	for i, k := range keys {
		names[i] = k.Path
	}
	return Key{}, fmt.Errorf("unknown key %q (want one of %s)", path, strings.Join(names, ", "))
}

// Parse converts the command-line text of a value into the key's type: bool,
// int, string or []string (comma-separated, trimmed; "" is the empty list).
// The error names the key and, for enums, lists the allowed values.
func (k Key) Parse(raw string) (any, error) {
	switch k.Kind {
	case KindBool:
		switch raw {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
		return nil, fmt.Errorf("%s: want true or false (got %q)", k.Path, raw)
	case KindInt:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: want an integer (got %q)", k.Path, raw)
		}
		if k.Min != nil && float64(n) < *k.Min {
			return nil, fmt.Errorf("%s: must be at least %v (got %d)", k.Path, *k.Min, n)
		}
		if err := k.checkEnum(strconv.Itoa(n)); err != nil {
			return nil, err
		}
		return n, nil
	case KindStringList:
		items := []string{}
		if strings.TrimSpace(raw) != "" {
			for _, item := range strings.Split(raw, ",") {
				item = strings.TrimSpace(item)
				if item == "" && k.itemEmpty {
					return nil, fmt.Errorf("%s: empty item in %q", k.Path, raw)
				}
				items = append(items, item)
			}
		}
		return items, nil
	}
	if k.NonEmpty && raw == "" {
		return nil, fmt.Errorf("%s: must not be empty", k.Path)
	}
	if err := k.checkEnum(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (k Key) checkEnum(value string) error {
	if len(k.Enum) == 0 || slices.Contains(k.Enum, value) {
		return nil
	}
	return fmt.Errorf("%s: allowed values: %s (got %q)", k.Path, strings.Join(k.Enum, ", "), value)
}

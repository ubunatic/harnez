package repocfg

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/fsutil"
	"ubunatic.com/harnez/internal/repospec"
)

// Result reports what a Set or Unset did.
type Result struct {
	Changed   bool // repo.yaml was rewritten
	NeedsInit bool // the key drives rendered output: run `harnez init`
}

// defaultIndent matches claude.writeRepoConfig, which wrote most repo.yaml files.
const defaultIndent = 4

func repoPath(dir string) string { return filepath.Join(dir, ".harnez", "repo.yaml") }

// Set parses raw for k and writes it to dir/.harnez/repo.yaml.
func Set(dir string, k Key, raw string) (Result, error) {
	value, err := k.Parse(raw)
	if err != nil {
		return Result{}, err
	}
	return apply(dir, k, func(root *yaml.Node) error {
		return setNode(root, strings.Split(k.Path, "."), valueNode(value))
	})
}

// Unset removes k from dir/.harnez/repo.yaml. A key that is not set, or whose
// removal would make the file invalid, is an error and nothing is written.
func Unset(dir string, k Key) (Result, error) {
	return apply(dir, k, func(root *yaml.Node) error {
		return unsetNode(root, k.Path)
	})
}

// apply edits the file's yaml nodes, validates the result with the same checks
// LoadRepoConfig runs, and writes only when that passes and the content
// changed. Any failure leaves the file untouched.
func apply(dir string, k Key, edit func(root *yaml.Node) error) (Result, error) {
	path := repoPath(dir)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Result{}, fmt.Errorf("no .harnez/repo.yaml in %s; run: harnez init -d %s", dir, dir)
	}
	if err != nil {
		return Result{}, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Result{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return Result{}, fmt.Errorf("%s: top level is not a mapping", path)
	}
	if err := edit(doc.Content[0]); err != nil {
		return Result{}, fmt.Errorf("%s: %w", k.Path, err)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(indentOf(data))
	if err := enc.Encode(&doc); err != nil {
		return Result{}, err
	}
	if err := enc.Close(); err != nil {
		return Result{}, err
	}
	out := buf.Bytes()
	if sameDocument(data, out) {
		return Result{}, nil
	}
	if _, err := claude.ParseRepoConfig(path, out); err != nil {
		return Result{}, fmt.Errorf("%s: refused, repo.yaml would be invalid: %w", k.Path, err)
	}
	if _, err := fsutil.WriteIfChanged(path, out); err != nil {
		return Result{}, err
	}
	spec, err := repospec.Load()
	if err != nil {
		return Result{}, err
	}
	return Result{Changed: true, NeedsInit: spec.Renders(k.Path)}, nil
}

// sameDocument compares what two YAML texts mean, ignoring formatting, so a
// set to the current value does not reformat a hand-written file.
func sameDocument(a, b []byte) bool {
	var x, y any
	return yaml.Unmarshal(a, &x) == nil && yaml.Unmarshal(b, &y) == nil && reflect.DeepEqual(x, y)
}

// indentOf returns the file's own indent width, so an edit does not re-indent
// a hand-written file.
func indentOf(data []byte) int {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if n := len(line) - len(trimmed); n > 0 {
			return min(max(n, 2), 9)
		}
	}
	return defaultIndent
}

func valueNode(v any) *yaml.Node {
	switch v := v.(type) {
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(v)}
	case int:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(v)}
	case []string:
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, item := range v {
			seq.Content = append(seq.Content, strNode(item))
		}
		return seq
	}
	return strNode(v.(string))
}

// strNode is a plain-style !!str node; the encoder quotes it when the text
// would otherwise read back as another type ("true", "123").
func strNode(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}

// pairIndex returns the index of key in a mapping's Content, or -1.
func pairIndex(m *yaml.Node, key string) int {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// setNode puts val at the dotted path below root, creating missing parent
// maps. An existing key node keeps its comments, and the old value hands its
// comments and flow/block style to the new one.
func setNode(root *yaml.Node, segs []string, val *yaml.Node) error {
	m := root
	for i, seg := range segs[:len(segs)-1] {
		idx := pairIndex(m, seg)
		if idx < 0 {
			child := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			m.Content = append(m.Content, strNode(seg), child)
			m = child
			continue
		}
		m = m.Content[idx+1]
		if m.Kind != yaml.MappingNode {
			return fmt.Errorf("%s is not a mapping", strings.Join(segs[:i+1], "."))
		}
	}
	last := segs[len(segs)-1]
	idx := pairIndex(m, last)
	if idx < 0 {
		m.Content = append(m.Content, strNode(last), val)
		return nil
	}
	old := m.Content[idx+1]
	val.HeadComment, val.LineComment, val.FootComment = old.HeadComment, old.LineComment, old.FootComment
	if old.Kind == yaml.SequenceNode && val.Kind == yaml.SequenceNode {
		val.Style = old.Style
	}
	m.Content[idx+1] = val
	return nil
}

// unsetNode deletes the dotted path below root, then every parent map that
// the removal left empty.
func unsetNode(root *yaml.Node, path string) error {
	segs := strings.Split(path, ".")
	maps := []*yaml.Node{root}
	for _, seg := range segs[:len(segs)-1] {
		idx := pairIndex(maps[len(maps)-1], seg)
		if idx < 0 {
			return fmt.Errorf("not set in .harnez/repo.yaml")
		}
		child := maps[len(maps)-1].Content[idx+1]
		if child.Kind != yaml.MappingNode {
			return fmt.Errorf("%s is not a mapping", seg)
		}
		maps = append(maps, child)
	}
	for depth := len(segs) - 1; depth >= 0; depth-- {
		m := maps[depth]
		idx := pairIndex(m, segs[depth])
		if idx < 0 {
			return fmt.Errorf("not set in .harnez/repo.yaml")
		}
		m.Content = append(m.Content[:idx], m.Content[idx+2:]...)
		if len(m.Content) > 0 {
			break
		}
	}
	return nil
}

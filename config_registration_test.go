package harnez

import (
	"io/fs"
	"path"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestCommandDocsAreRegisteredInConfig keeps every embedded docs/commands/*.md
// discoverable: each must be a skill/command `file:` or a resource `source:`
// in config.yaml (formerly scripts/lint.sh, issue 1254).
func TestCommandDocsAreRegisteredInConfig(t *testing.T) {
	cfg, err := DefaultFS.ReadFile("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	docs, err := fs.Glob(DefaultFS, "docs/commands/*.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) == 0 {
		t.Fatal("no docs/commands/*.md embedded")
	}
	missing, err := unregisteredCommandDocs(cfg, docs)
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range missing {
		t.Errorf("unregistered docs command source: %s (add it to config.yaml as file: or source:)", doc)
	}
}

func TestUnregisteredCommandDocsReadsFileAndSourceKeysAtAnyDepth(t *testing.T) {
	cfg := []byte(`
skills:
  - name: a
    file: docs/commands/a.md
    resources:
      - source: docs/commands/b.md
other:
  note: docs/commands/c.md
`)
	docs := []string{"docs/commands/a.md", "docs/commands/b.md", "docs/commands/c.md"}
	got, err := unregisteredCommandDocs(cfg, docs)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"docs/commands/c.md"}; !slices.Equal(got, want) {
		t.Fatalf("unregistered = %v, want %v (a value under another key does not register a doc)", got, want)
	}
}

// unregisteredCommandDocs returns the docs that no `file:` or `source:` value
// in cfg names.
func unregisteredCommandDocs(cfg []byte, docs []string) ([]string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(cfg, &root); err != nil {
		return nil, err
	}
	registered := map[string]bool{}
	var walk func(*yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				key, val := n.Content[i], n.Content[i+1]
				if (key.Value == "file" || key.Value == "source") && val.Kind == yaml.ScalarNode {
					registered[path.Clean(val.Value)] = true
				}
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(&root)
	var missing []string
	for _, doc := range docs {
		if !registered[doc] {
			missing = append(missing, doc)
		}
	}
	return missing, nil
}

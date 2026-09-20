// Command quote-yaml-strings rewrites YAML string scalars with explicit quotes.
// Numeric and boolean scalars retain their native YAML representation.
package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: quote-yaml-strings <file>")
		os.Exit(2)
	}
	path := os.Args[1]
	b, err := os.ReadFile(path)
	if err != nil {
		fatal(err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		fatal(err)
	}
	quoteStrings(&doc)
	out, err := yaml.Marshal(&doc)
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		fatal(err)
	}
}

func quoteStrings(node *yaml.Node) {
	if node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
		node.Style = yaml.DoubleQuotedStyle
	}
	for _, child := range node.Content {
		quoteStrings(child)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

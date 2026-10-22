package claude

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"
)

func genSkillContent(cmd Command, fsys fs.FS) (string, error) {
	body := cmd.Content
	if cmd.File != "" {
		data, err := fs.ReadFile(fsys, cmd.File)
		if err != nil {
			return "", fmt.Errorf("skill %s: %w", cmd.Name, err)
		}
		body = string(data)
	}
	body, err := renderSkillBody(cmd.Name, body, fsys)
	if err != nil {
		return "", err
	}
	_, sourceFrontmatter, sourceBody, hasFrontmatter, err := parseSkillFrontmatter(body)
	if err != nil {
		return "", fmt.Errorf("skill %s frontmatter: %w", cmd.Name, err)
	}
	var sb strings.Builder
	sb.WriteString("---\n")
	if hasFrontmatter {
		merged, err := mergeSkillFrontmatter(cmd, sourceFrontmatter)
		if err != nil {
			return "", fmt.Errorf("skill %s frontmatter: %w", cmd.Name, err)
		}
		sb.WriteString(merged)
		sb.WriteString("---\n")
		body = sourceBody
	} else {
		fmt.Fprintf(&sb, "name: %q\n", cmd.Name)
		if cmd.Description != "" {
			fmt.Fprintf(&sb, "description: %q\n", cmd.Description)
		}
		sb.WriteString("---\n")
	}
	sb.WriteString(strings.TrimRight(body, "\n"))
	sb.WriteString("\n")
	return sb.String(), nil
}

func mergeSkillFrontmatter(cmd Command, raw string) (string, error) {
	var document yaml.Node
	if strings.TrimSpace(raw) == "" {
		document.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	} else if err := yaml.Unmarshal([]byte(raw), &document); err != nil {
		return "", err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return "", fmt.Errorf("frontmatter must be a YAML mapping")
	}
	source := document.Content[0]
	fields := make(map[string]*yaml.Node, len(source.Content)/2)
	for i := 0; i+1 < len(source.Content); i += 2 {
		key := source.Content[i]
		if key.Kind == yaml.ScalarNode {
			fields[key.Value] = source.Content[i+1]
		}
	}
	name := stringNode(cmd.Name)
	if node, ok := fields["name"]; ok {
		var err error
		name, err = sourceStringNode("name", node)
		if err != nil {
			return "", err
		}
	}
	description := fields["description"]
	if description != nil {
		var err error
		description, err = sourceStringNode("description", description)
		if err != nil {
			return "", err
		}
	} else if cmd.Description != "" {
		description = stringNode(cmd.Description)
	}

	merged := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	appendPair := func(key string, value *yaml.Node) {
		merged.Content = append(merged.Content, keyNode(key), value)
	}
	appendPair("name", name)
	if description != nil {
		appendPair("description", description)
	}
	for i := 0; i+1 < len(source.Content); i += 2 {
		key, value := source.Content[i], source.Content[i+1]
		if key.Kind == yaml.ScalarNode && (key.Value == "name" || key.Value == "description") {
			continue
		}
		merged.Content = append(merged.Content, key, value)
	}
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	if err := encoder.Encode(merged); err != nil {
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func stringNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value, Style: yaml.DoubleQuotedStyle}
}

func keyNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func sourceStringNode(key string, node *yaml.Node) (*yaml.Node, error) {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return nil, fmt.Errorf("%s frontmatter value must be a string", key)
	}
	node.Style = yaml.DoubleQuotedStyle
	return node, nil
}

// parseSkillFrontmatter extracts only a YAML block that starts on line one.
// It returns the parsed keys, raw YAML block, remaining body, and presence bit.
func parseSkillFrontmatter(body string) (map[string]any, string, string, bool, error) {
	lines := strings.SplitAfter(body, "\n")
	if len(lines) == 0 || strings.TrimSuffix(strings.TrimSuffix(lines[0], "\n"), "\r") != "---" {
		return nil, "", body, false, nil
	}
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSuffix(strings.TrimSuffix(lines[i], "\n"), "\r")
		if line != "---" {
			continue
		}
		raw := strings.Join(lines[1:i], "")
		metadata := make(map[string]any)
		if err := yaml.Unmarshal([]byte(raw), &metadata); err != nil {
			return nil, "", "", true, err
		}
		return metadata, raw, strings.Join(lines[i+1:], ""), true, nil
	}
	return nil, "", "", true, fmt.Errorf("opening delimiter has no closing delimiter")
}

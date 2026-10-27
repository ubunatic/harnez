// Package pickdocs renders the embedded role-pick specification for documentation.
package pickdocs

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const Begin = "<!-- harnez:model-picks:begin -->"
const End = "<!-- harnez:model-picks:end -->"

func Markdown(fsys fs.FS) (string, error) {
	data, err := fs.ReadFile(fsys, "spec/agent.yaml")
	if err != nil {
		return "", err
	}
	var spec struct {
		Traits map[string]string `yaml:"model_traits"`
		Roles  map[string]struct {
			Default string              `yaml:"default_model"`
			Models  map[string][]string `yaml:"models"`
		} `yaml:"roles"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return "", err
	}
	var roles []string
	for role := range spec.Roles {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	var b strings.Builder
	b.WriteString("| Role | Trait | Meaning | Chain | Default |\n|---|---|---|---|---|\n")
	for _, name := range roles {
		role := spec.Roles[name]
		var traits []string
		for trait := range role.Models {
			traits = append(traits, trait)
		}
		sort.Strings(traits)
		for _, trait := range traits {
			chain := strings.Join(role.Models[trait], ", ")
			if chain == "" {
				chain = "unset"
			}
			def := ""
			if role.Default == trait {
				def = "yes"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", name, trait, spec.Traits[trait], chain, def)
		}
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

func Render(fsys fs.FS, body string) (string, error) {
	if !strings.Contains(body, Begin) && !strings.Contains(body, End) {
		return body, nil
	}
	md, err := Markdown(fsys)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for {
		start := strings.Index(body, Begin)
		if start < 0 {
			if strings.Contains(body, End) {
				return "", fmt.Errorf("unmatched picks block end")
			}
			out.WriteString(body)
			return out.String(), nil
		}
		out.WriteString(body[:start])
		body = body[start+len(Begin):]
		end := strings.Index(body, End)
		if end < 0 || strings.Contains(body[:end], Begin) {
			return "", fmt.Errorf("unmatched picks block begin")
		}
		out.WriteString(Begin + "\n" + md + "\n" + End)
		body = body[end+len(End):]
	}
}

func WithoutBlocks(s string) string {
	for {
		a := strings.Index(s, Begin)
		if a < 0 {
			return s
		}
		b := strings.Index(s[a+len(Begin):], End)
		if b < 0 {
			return s
		}
		s = s[:a] + s[a+len(Begin)+b+len(End):]
	}
}

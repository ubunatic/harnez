// Package companion installs companion tools (gokay, loom, fj, ...) and
// resolves apply profiles from spec/tools.yaml (issue 1190).
package companion

import (
	"fmt"
	"io"
	"os/exec"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

// Tool is one installable companion tool.
type Tool struct {
	Name      string `yaml:"name"`
	Installer string `yaml:"installer"`
	Package   string `yaml:"package"`
	Binary    string `yaml:"binary"`
}

// Profile groups docs, tools and harnez components.
type Profile struct {
	Docs       []string `yaml:"docs"`
	Tools      []string `yaml:"tools"`
	Components []string `yaml:"components"`
}

// Spec mirrors spec/tools.yaml.
type Spec struct {
	Tools      []Tool              `yaml:"tools"`
	Installers map[string][]string `yaml:"installers"`
	Profiles   map[string]Profile  `yaml:"profiles"`
}

// LoadSpec parses the embedded spec/tools.yaml and checks its references.
func LoadSpec() (Spec, error) {
	b, err := harnez.DefaultFS.ReadFile("spec/tools.yaml")
	if err != nil {
		return Spec{}, fmt.Errorf("load tools spec: %w", err)
	}
	var s Spec
	if err := yaml.Unmarshal(b, &s); err != nil {
		return Spec{}, fmt.Errorf("parse tools spec: %w", err)
	}
	for _, t := range s.Tools {
		if len(s.Installers[t.Installer]) == 0 {
			return Spec{}, fmt.Errorf("tools spec: tool %q uses unknown installer %q", t.Name, t.Installer)
		}
	}
	for name, p := range s.Profiles {
		for _, tool := range p.Tools {
			if _, ok := s.tool(tool); !ok {
				return Spec{}, fmt.Errorf("tools spec: profile %q names unknown tool %q", name, tool)
			}
		}
	}
	return s, nil
}

func (s Spec) tool(name string) (Tool, bool) {
	for _, t := range s.Tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

// ToolNames lists the known tools in spec order.
func (s Spec) ToolNames() []string {
	out := make([]string, 0, len(s.Tools))
	for _, t := range s.Tools {
		out = append(out, t.Name)
	}
	return out
}

// ProfileNames lists the known profiles, sorted.
func (s Spec) ProfileNames() []string {
	out := make([]string, 0, len(s.Profiles))
	for n := range s.Profiles {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Selection is the union of the requested profiles plus explicit flags.
type Selection struct {
	Docs, Tools, Components []string
}

// Resolve merges the named profiles with explicit docs, tools and components,
// keeping first-seen order and dropping duplicates. Unknown names are errors.
func (s Spec) Resolve(profiles, docs, tools, components []string) (Selection, error) {
	var sel Selection
	add := func(dst *[]string, items ...string) {
		for _, it := range items {
			if it != "" && !slices.Contains(*dst, it) {
				*dst = append(*dst, it)
			}
		}
	}
	add(&sel.Docs, docs...)
	add(&sel.Components, components...)
	for _, t := range tools {
		if _, ok := s.tool(t); !ok {
			return Selection{}, fmt.Errorf("unknown tool %q (known: %s)", t, strings.Join(s.ToolNames(), ", "))
		}
		add(&sel.Tools, t)
	}
	for _, name := range profiles {
		p, ok := s.Profiles[name]
		if !ok {
			return Selection{}, fmt.Errorf("unknown profile %q (known: %s)", name, strings.Join(s.ProfileNames(), ", "))
		}
		add(&sel.Docs, p.Docs...)
		add(&sel.Tools, p.Tools...)
		add(&sel.Components, p.Components...)
	}
	return sel, nil
}

// Env abstracts PATH lookup and command execution for tests.
type Env struct {
	LookPath func(string) (string, error)
	Run      func(argv []string, out io.Writer) error
	Upgrade  bool // reinstall tools already on PATH
}

// DefaultEnv uses the real PATH and runs installers with output to out.
func DefaultEnv() Env {
	return Env{
		LookPath: exec.LookPath,
		Run: func(argv []string, out io.Writer) error {
			cmd := exec.Command(argv[0], argv[1:]...)
			cmd.Stdout, cmd.Stderr = out, out
			return cmd.Run()
		},
	}
}

// Install installs the named tools. A missing installer or a failed install
// is a warning on warn, never an error: apply continues. It returns the
// number of warnings.
func (s Spec) Install(names []string, env Env, out, warn io.Writer) int {
	warnings := 0
	for _, name := range names {
		t, ok := s.tool(name)
		if !ok {
			fmt.Fprintf(warn, "warning: tool %s: not in spec/tools.yaml, skipped\n", name)
			warnings++
			continue
		}
		if !env.Upgrade {
			if p, err := env.LookPath(t.Binary); err == nil {
				fmt.Fprintf(out, "  tool %s: already installed (%s)\n", t.Name, p)
				continue
			}
		}
		argv := append(slices.Clone(s.Installers[t.Installer]), t.Package)
		if _, err := env.LookPath(argv[0]); err != nil {
			fmt.Fprintf(warn, "warning: tool %s: installer %q not on PATH, skipped (run: %s)\n", t.Name, argv[0], strings.Join(argv, " "))
			warnings++
			continue
		}
		fmt.Fprintf(out, "  tool %s: %s\n", t.Name, strings.Join(argv, " "))
		if err := env.Run(argv, out); err != nil {
			fmt.Fprintf(warn, "warning: tool %s: install failed: %v\n", t.Name, err)
			warnings++
		}
	}
	return warnings
}

// Package companion installs companion tools (gokay, loom, fj, ...) and
// resolves apply profiles from spec/tools.yaml (issue 1190).
package companion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
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

// LoadSpec reads spec/tools.yaml, validates it against its schema, decodes it
// strictly and checks cross references.
func LoadSpec() (Spec, error) {
	return loadSpecFrom(harnez.DefaultFS, "spec/tools.yaml", "spec/schemas/tools.schema.json")
}

func loadSpecFrom(fsys fs.FS, specPath, schemaPath string) (Spec, error) {
	data, err := fs.ReadFile(fsys, specPath)
	if err != nil {
		return Spec{}, fmt.Errorf("read %s: %w", specPath, err)
	}
	schemaData, err := fs.ReadFile(fsys, schemaPath)
	if err != nil {
		return Spec{}, fmt.Errorf("read %s: %w", schemaPath, err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return Spec{}, fmt.Errorf("parse %s: %w", specPath, err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return Spec{}, fmt.Errorf("parse %s: %w", schemaPath, err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return Spec{}, fmt.Errorf("resolve %s: %w", schemaPath, err)
	}
	if err := resolved.Validate(document); err != nil {
		return Spec{}, fmt.Errorf("validate %s: %w", specPath, err)
	}
	var s Spec
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return Spec{}, fmt.Errorf("decode %s: %w", specPath, err)
	}
	seen := map[string]bool{}
	for _, t := range s.Tools {
		if seen[t.Name] {
			return Spec{}, fmt.Errorf("%s: duplicate tool %q", specPath, t.Name)
		}
		seen[t.Name] = true
		if len(s.Installers[t.Installer]) == 0 {
			return Spec{}, fmt.Errorf("%s: tool %q uses unknown installer %q", specPath, t.Name, t.Installer)
		}
	}
	for name, p := range s.Profiles {
		for _, tool := range p.Tools {
			if !seen[tool] {
				return Spec{}, fmt.Errorf("%s: profile %q names unknown tool %q", specPath, name, tool)
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
	var withComponents, without []string
	for _, name := range profiles {
		p, ok := s.Profiles[name]
		if !ok {
			return Selection{}, fmt.Errorf("unknown profile %q (known: %s)", name, strings.Join(s.ProfileNames(), ", "))
		}
		if len(p.Components) > 0 {
			withComponents = append(withComponents, name)
		} else {
			without = append(without, name)
		}
	}
	// A profile without components means all components; mixing it with one
	// that narrows them would silently drop parts, so refuse.
	if len(withComponents) > 0 && len(without) > 0 {
		return Selection{}, fmt.Errorf("profiles %s narrow harnez components and cannot be combined with %s; run them separately",
			strings.Join(withComponents, ","), strings.Join(without, ","))
	}
	for _, name := range profiles {
		p := s.Profiles[name]
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

// installTimeout bounds one installer run so a network stall cannot hang apply.
const installTimeout = 15 * time.Minute

// DefaultEnv uses the real PATH and runs installers with output to out.
func DefaultEnv() Env {
	return Env{
		LookPath: exec.LookPath,
		Run: func(argv []string, out io.Writer) error {
			ctx, cancel := context.WithTimeout(context.Background(), installTimeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
			cmd.Stdout, cmd.Stderr = out, out
			cmd.WaitDelay = 5 * time.Second
			err := cmd.Run()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("timed out after %s", installTimeout)
			}
			return err
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

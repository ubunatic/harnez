package companion

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func mustSpec(t *testing.T) Spec {
	t.Helper()
	s, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSpecMatchesSchema(t *testing.T) {
	raw, err := harnez.DefaultFS.ReadFile("spec/schemas/tools.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := harnez.DefaultFS.ReadFile("spec/tools.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(doc); err != nil {
		t.Fatalf("spec/tools.yaml violates its schema: %v", err)
	}
	mustSpec(t)
}

// fakeEnv: bins on PATH, and installs that fail by tool package.
func fakeEnv(onPath []string, failing string, ran *[][]string) Env {
	return Env{
		LookPath: func(b string) (string, error) {
			if slices.Contains(onPath, b) {
				return "/bin/" + b, nil
			}
			return "", errors.New("not found")
		},
		Run: func(argv []string, _ io.Writer) error {
			*ran = append(*ran, argv)
			if failing != "" && argv[len(argv)-1] == failing {
				return errors.New("exit status 1")
			}
			return nil
		},
	}
}

func TestInstallSuccessSkipAndWarnings(t *testing.T) {
	s := mustSpec(t)
	var ran [][]string
	var out, warn bytes.Buffer
	// go present, gokay already installed, cargo missing.
	n := s.Install([]string{"gokay", "loom", "fj"}, fakeEnv([]string{"go", "gokay"}, "", &ran), &out, &warn)
	if n != 1 {
		t.Fatalf("warnings = %d, want 1 (cargo missing)\nwarn: %s", n, warn.String())
	}
	if len(ran) != 1 || ran[0][0] != "go" || ran[0][2] != "ubunatic.com/loom/cmd/loom@latest" {
		t.Errorf("ran = %v, want only go install loom", ran)
	}
	if !strings.Contains(out.String(), "gokay: already installed") {
		t.Errorf("out lacks skip line: %s", out.String())
	}
	if !strings.Contains(warn.String(), `installer "cargo" not on PATH`) {
		t.Errorf("warn lacks missing-installer line: %s", warn.String())
	}
}

func TestInstallFailureIsAWarning(t *testing.T) {
	s := mustSpec(t)
	var ran [][]string
	var out, warn bytes.Buffer
	n := s.Install([]string{"fj", "gokay"}, fakeEnv([]string{"go", "cargo"}, "forgejo-cli", &ran), &out, &warn)
	if n != 1 || !strings.Contains(warn.String(), "tool fj: install failed") {
		t.Fatalf("warnings = %d, warn %q", n, warn.String())
	}
	if len(ran) != 2 {
		t.Errorf("a failed install stopped the others: ran %v", ran)
	}
}

func TestResolveProfiles(t *testing.T) {
	s := mustSpec(t)
	sel, err := s.Resolve([]string{"go", "codeberg"}, []string{"canary", "golang"}, []string{"loom"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"canary", "golang", "make", "agit"}; !slices.Equal(sel.Docs, want) {
		t.Errorf("docs = %v, want %v", sel.Docs, want)
	}
	if want := []string{"loom", "gokay", "fj"}; !slices.Equal(sel.Tools, want) {
		t.Errorf("tools = %v, want %v", sel.Tools, want)
	}
	if sel, _ := s.Resolve([]string{"minimal"}, nil, nil, nil); !slices.Equal(sel.Components, []string{"docs-only"}) {
		t.Errorf("minimal components = %v", sel.Components)
	}
	if _, err := s.Resolve([]string{"nope"}, nil, nil, nil); err == nil {
		t.Error("unknown profile accepted")
	}
	if _, err := s.Resolve(nil, nil, []string{"nope"}, nil); err == nil {
		t.Error("unknown tool accepted")
	}
	if _, err := s.Resolve([]string{"minimal", "go"}, nil, nil, nil); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Errorf("minimal+go: err = %v, want refusal", err)
	}
}

func TestLoadSpecRejectsInvalid(t *testing.T) {
	schema, err := harnez.DefaultFS.ReadFile("spec/schemas/tools.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"unknown field":     "tools: [{name: a, installer: go, package: p, binary: a, extra: 1}]\ninstallers: {go: [go, install]}\nprofiles: {}\n",
		"duplicate tool":    "tools: [{name: a, installer: go, package: p, binary: a}, {name: a, installer: go, package: q, binary: a}]\ninstallers: {go: [go, install]}\nprofiles: {}\n",
		"unknown installer": "tools: [{name: a, installer: npm, package: p, binary: a}]\ninstallers: {go: [go, install]}\nprofiles: {}\n",
		"unknown tool":      "tools: [{name: a, installer: go, package: p, binary: a}]\ninstallers: {go: [go, install]}\nprofiles: {x: {tools: [b]}}\n",
	}
	for name, spec := range cases {
		fsys := fstest.MapFS{"t.yaml": {Data: []byte(spec)}, "s.json": {Data: schema}}
		if _, err := loadSpecFrom(fsys, "t.yaml", "s.json"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func TestInitWizardScriptAndPersistence(t *testing.T) {
	current := detectRepoConfig(t.TempDir(), nil)
	current.Issues.ActiveCap = 37
	current.Make.ContainerTest = true
	var output bytes.Buffer
	dir := t.TempDir()
	next, err := writeInitWizardConfig(dir, current, []string{"golang"}, strings.NewReader("maybe\nyes\non\ny\ny\nyes\non"), &output)
	if err != nil {
		t.Fatal(err)
	}
	if next.Issues.Tracker != "harnez" || !next.Issues.MergeDriver || !next.Docs.Studies || !next.Make.Scaffold || !next.Rules.Quota || !reflect.DeepEqual(next.Docs.Copies, []string{"golang"}) {
		t.Fatalf("choices not applied: %+v", next)
	}
	if strings.Count(output.String(), "Issue tracker —") != 2 || !strings.Contains(output.String(), "Please answer") {
		t.Fatalf("invalid answer not retried: %s", &output)
	}
	loaded, err := LoadRepoConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(next, loaded) || loaded.Issues.ActiveCap != 37 || !loaded.Make.ContainerTest {
		t.Fatalf("persistence lost choices: %+v", loaded)
	}
	if current.Issues.Tracker != "none" || len(current.Docs.Copies) != 0 {
		t.Fatal("input mutated")
	}
}

func TestInitWizardDefaultsAndDisable(t *testing.T) {
	current := detectRepoConfig(t.TempDir(), nil)
	current.Issues.Tracker = "harnez"
	current.Issues.MergeDriver = true
	current.Docs.Studies = true
	current.Docs.Copies = []string{"bash"}
	current.Make.Scaffold = true
	current.Rules.Quota = true
	for _, input := range []string{"", "\n\n\n\n\n\n", "invalid"} {
		got, err := promptInitConfig(current, []string{"golang"}, strings.NewReader(input), io.Discard)
		if err != nil || !reflect.DeepEqual(got, current) {
			t.Fatalf("input %q: got %+v, err %v", input, got, err)
		}
	}
	got, err := promptInitConfig(current, nil, strings.NewReader("n\nno\noff\nn\nno\noff\n"), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if got.Issues.Tracker != "none" || got.Docs.Studies || got.Issues.MergeDriver || len(got.Docs.Copies) != 0 || got.Make.Scaffold || got.Rules.Quota {
		t.Fatalf("disable failed: %+v", got)
	}
	if current.Docs.Copies[0] != "bash" {
		t.Fatal("input slice mutated")
	}
}

func TestInitWizardTrackerControlsMergeDriverPrompt(t *testing.T) {
	current := detectRepoConfig(t.TempDir(), nil)
	current.Issues.Tracker = "none"
	current.Issues.MergeDriver = true
	var output bytes.Buffer
	got, err := promptInitConfig(current, nil, strings.NewReader("\n\nyes\n\n\n"), &output)
	if err != nil {
		t.Fatal(err)
	}
	if got.Issues.MergeDriver {
		t.Fatal("merge driver enabled without tracker")
	}
	if strings.Contains(output.String(), "Merge driver —") {
		t.Fatal("merge driver prompt shown while tracker is disabled")
	}

	current.Issues.Tracker = "harnez"
	current.Issues.MergeDriver = false
	output.Reset()
	got, err = promptInitConfig(current, nil, strings.NewReader("\n\n\n\n\n\n"), &output)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Issues.MergeDriver {
		t.Fatal("enabled tracker did not default merge driver on")
	}
	if !strings.Contains(output.String(), "Merge driver —") {
		t.Fatal("merge driver prompt missing while tracker is enabled")
	}
}

func TestInitWizardEmptyOptionalCopiesExplainsNoEffect(t *testing.T) {
	current := detectRepoConfig(t.TempDir(), nil)
	var output bytes.Buffer
	got, err := promptInitConfig(current, nil, strings.NewReader("\n\nyes\n\n\n"), &output)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Docs.Copies) != 0 {
		t.Fatalf("unexpected docs copies: %v", got.Docs.Copies)
	}
	if !strings.Contains(output.String(), "No optional docs are detected or recorded") {
		t.Fatalf("empty choice was not explained: %s", &output)
	}
}

type wizardFailReader struct{}

func (wizardFailReader) Read([]byte) (int, error) { return 0, errors.New("input failed") }

func TestInitWizardInputErrorDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	_, err := writeInitWizardConfig(dir, detectRepoConfig(dir, nil), nil, wizardFailReader{}, io.Discard)
	if err == nil {
		t.Fatal("read failure swallowed")
	}
	if _, err := LoadRepoConfig(dir); err == nil {
		t.Fatal("config written on input failure")
	}
}

func TestInitWizardSpec(t *testing.T) {
	data, err := harnez.DefaultFS.ReadFile("spec/init-wizard.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	schemaData, err := harnez.DefaultFS.ReadFile("spec/schemas/init-wizard.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(doc); err != nil {
		t.Fatal(err)
	}
	var spec initWizardSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, prompt := range spec.Prompts {
		if seen[prompt.Key] {
			t.Fatalf("duplicate prompt %s", prompt.Key)
		}
		seen[prompt.Key] = true
	}
}

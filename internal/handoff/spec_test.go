package handoff

import (
	"encoding/json"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"testing"

	"ubunatic.com/harnez"
)

func TestEmbeddedSpecLoads(t *testing.T) {
	if _, err := LoadSpec(); err != nil {
		t.Fatal(err)
	}
}

// Every agent named in issue 726 is present with the profile the ticket gives it.
func TestTicketAgentsPresent(t *testing.T) {
	s, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	for name, profile := range map[string]string{
		"Claude":                      "local",
		"Codex":                       "local",
		"Gemini":                      "local",
		"Google Jules":                "cloud",
		"Jules":                       "cloud",
		"GitHub Copilot coding agent": "cloud",
		"Copilot":                     "cloud",
	} {
		a := s.Find(name)
		if a == nil {
			t.Errorf("agent %q missing", name)
			continue
		}
		if a.Profile != profile {
			t.Errorf("agent %q profile = %q, want %q", name, a.Profile, profile)
		}
	}
}

type schemaDoc struct {
	Definitions map[string]struct {
		Properties map[string]json.RawMessage `json:"properties"`
	} `json:"definitions"`
	Properties struct {
		Profiles struct {
			Required   []string                   `json:"required"`
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"profiles"`
		Agents struct {
			AdditionalProperties struct {
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"additionalProperties"`
		} `json:"agents"`
	} `json:"properties"`
}

func loadSchema(t *testing.T) schemaDoc {
	t.Helper()
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/handoff.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc schemaDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func yamlFields(v any) []string {
	var names []string
	rt := reflect.TypeOf(v)
	for i := range rt.NumField() {
		tag := strings.Split(rt.Field(i).Tag.Get("yaml"), ",")[0]
		if tag != "" && tag != "-" {
			names = append(names, tag)
		}
	}
	slices.Sort(names)
	return names
}

func sortedKeys[V any](m map[string]V) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// The schema and the Go structs declare the same fields, and the profile enum
// matches both the schema's profile list and the profiles in the YAML.
func TestSchemaMatchesCode(t *testing.T) {
	doc := loadSchema(t)
	agentProps := doc.Properties.Agents.AdditionalProperties.Properties
	if got, want := sortedKeys(agentProps), yamlFields(Agent{}); !slices.Equal(got, want) {
		t.Errorf("schema agent fields %v, Go fields %v", got, want)
	}
	if got, want := sortedKeys(doc.Definitions["profile"].Properties), yamlFields(Profile{}); !slices.Equal(got, want) {
		t.Errorf("schema profile fields %v, Go fields %v", got, want)
	}

	var profile struct {
		Enum []string `json:"enum"`
	}
	if err := json.Unmarshal(agentProps["profile"], &profile); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	enum := slices.Sorted(slices.Values(profile.Enum))
	for name, got := range map[string][]string{
		"schema profiles.required":   slices.Sorted(slices.Values(doc.Properties.Profiles.Required)),
		"schema profiles.properties": sortedKeys(doc.Properties.Profiles.Properties),
		"spec profiles":              sortedKeys(s.Profiles),
	} {
		if !slices.Equal(got, enum) {
			t.Errorf("%s = %v, want profile enum %v", name, got, enum)
		}
	}
	if !slices.Contains(enum, cloudProfile) {
		t.Errorf("profile enum %v lacks %q", enum, cloudProfile)
	}
}

func TestParseSpecRejects(t *testing.T) {
	const profiles = `profiles:
  local: {description: d, rules: [r]}
  cloud: {description: d, rules: [r]}
`
	cases := map[string]string{
		"agent without profile":      `agents: {x: {name: X, aliases: [x], facts: [f]}}`,
		"agent with unknown profile": `agents: {x: {name: X, profile: remote, aliases: [x], facts: [f]}}`,
		"unknown field":              `agents: {x: {name: X, profile: local, aliases: [x], facts: [f], color: red}}`,
		"cloud without hosts":        `agents: {x: {name: X, profile: cloud, aliases: [x], facts: [f], result: r}}`,
		"cloud without result":       `agents: {x: {name: X, profile: cloud, aliases: [x], facts: [f], hosts: [github.com]}}`,
		"local with result":          `agents: {x: {name: X, profile: local, aliases: [x], facts: [f], result: r}}`,
		"agent without aliases":      `agents: {x: {name: X, profile: local, facts: [f]}}`,
		"duplicate alias":            `agents: {x: {name: X, profile: local, aliases: [a], facts: [f]}, y: {name: Y, profile: local, aliases: [A], facts: [f]}}`,
		"no agents":                  `agents: {}`,
	}
	for name, agents := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseSpec([]byte(profiles + agents)); err == nil {
				t.Fatal("parseSpec accepted invalid spec")
			}
		})
	}
	ok := profiles + `agents: {x: {name: X, profile: cloud, aliases: [x], facts: [f], hosts: [github.com], result: r}}`
	if _, err := parseSpec([]byte(ok)); err != nil {
		t.Fatalf("valid spec rejected: %v", err)
	}
}

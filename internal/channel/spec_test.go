package channel

import (
	"encoding/json"
	"reflect"
	"regexp"
	"testing"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func TestEmbeddedChannelSpecAndSchema(t *testing.T) {
	if _, err := LoadConfig(); err != nil {
		t.Fatal(err)
	}
	data, err := harnez.DefaultFS.ReadFile("spec/channel.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatal(err)
	}
	data, err = harnez.DefaultFS.ReadFile("spec/schemas/channel.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Type                 string   `json:"type"`
		AdditionalProperties bool     `json:"additionalProperties"`
		Required             []string `json:"required"`
		Properties           map[string]struct {
			Type      string `json:"type"`
			Minimum   int    `json:"minimum"`
			MinLength int    `json:"minLength"`
			Pattern   string `json:"pattern"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	fields := reflect.TypeFor[Config]()
	if schema.Type != "object" || schema.AdditionalProperties || len(schema.Properties) != fields.NumField() || len(schema.Required) != fields.NumField() {
		t.Fatalf("schema and Config fields diverged: %#v", schema)
	}
	for _, key := range schema.Required {
		if _, ok := values[key]; !ok {
			t.Errorf("required spec field missing: %s", key)
		}
	}
	for i := range fields.NumField() {
		field := fields.Field(i)
		key := field.Tag.Get("yaml")
		property, ok := schema.Properties[key]
		if !ok {
			t.Fatalf("schema field missing: %s", key)
		}
		switch value := values[key].(type) {
		case string:
			if property.Type != "string" || len([]rune(value)) < property.MinLength {
				t.Errorf("spec %s violates schema type/length", key)
			}
			if property.Pattern != "" {
				pattern, err := regexp.Compile(property.Pattern)
				if err != nil || !pattern.MatchString(value) {
					t.Errorf("spec %s violates schema pattern: %v", key, err)
				}
			}
		case int:
			if property.Type != "integer" || value < property.Minimum {
				t.Errorf("spec %s violates schema type/minimum", key)
			}
		default:
			t.Errorf("unexpected spec type for %s: %T", key, value)
		}
	}
}

package agy

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func TestEmbeddedSpecLoads(t *testing.T) {
	spec, err := LoadSpec()
	if err != nil {
		t.Fatalf("LoadSpec() failed: %v", err)
	}

	if len(spec.DisallowedTools) == 0 {
		t.Fatal("expected at least one disallowed tool in embedded spec")
	}

	schedule, ok := spec.DisallowedTools["schedule"]
	if !ok {
		t.Fatal("expected 'schedule' to be present in disallowed_tools")
	}

	wantReason := "violates the harnez rule to not use schedules but run commands in the background and just wait until completion"
	if schedule.Reason != wantReason {
		t.Errorf("schedule.Reason = %q, want %q", schedule.Reason, wantReason)
	}
}

func TestEmbeddedSpecMatchesSchema(t *testing.T) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/agy_tools.schema.json")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}

	var schema struct {
		Required             []string `json:"required"`
		AdditionalProperties *bool    `json:"additionalProperties"`
		Properties           struct {
			DisallowedTools struct {
				Type                 string `json:"type"`
				MinProperties        int    `json:"minProperties"`
				AdditionalProperties struct {
					Type                 string   `json:"type"`
					Required             []string `json:"required"`
					AdditionalProperties *bool    `json:"additionalProperties"`
					Properties           struct {
						Reason struct {
							Type      string `json:"type"`
							MinLength int    `json:"minLength"`
						} `json:"reason"`
					} `json:"properties"`
				} `json:"additionalProperties"`
			} `json:"disallowed_tools"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	if schema.AdditionalProperties == nil || *schema.AdditionalProperties != false {
		t.Errorf("top-level additionalProperties = %v, want explicitly false", schema.AdditionalProperties)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "disallowed_tools" {
		t.Errorf("schema required = %v, want ['disallowed_tools']", schema.Required)
	}
	if schema.Properties.DisallowedTools.Type != "object" {
		t.Errorf("disallowed_tools type = %q, want object", schema.Properties.DisallowedTools.Type)
	}
	if schema.Properties.DisallowedTools.MinProperties != 1 {
		t.Errorf("disallowed_tools minProperties = %d, want 1", schema.Properties.DisallowedTools.MinProperties)
	}
	if schema.Properties.DisallowedTools.AdditionalProperties.AdditionalProperties == nil || *schema.Properties.DisallowedTools.AdditionalProperties.AdditionalProperties != false {
		t.Errorf("entry additionalProperties = %v, want explicitly false", schema.Properties.DisallowedTools.AdditionalProperties.AdditionalProperties)
	}
	if schema.Properties.DisallowedTools.AdditionalProperties.Type != "object" {
		t.Errorf("entry type = %q, want object", schema.Properties.DisallowedTools.AdditionalProperties.Type)
	}
	entryReq := schema.Properties.DisallowedTools.AdditionalProperties.Required
	if len(entryReq) != 1 || entryReq[0] != "reason" {
		t.Errorf("disallowed entry required = %v, want ['reason']", entryReq)
	}
	if schema.Properties.DisallowedTools.AdditionalProperties.Properties.Reason.Type != "string" {
		t.Errorf("reason type = %q, want string", schema.Properties.DisallowedTools.AdditionalProperties.Properties.Reason.Type)
	}
	if schema.Properties.DisallowedTools.AdditionalProperties.Properties.Reason.MinLength < 1 {
		t.Errorf("reason minLength = %d, want >= 1", schema.Properties.DisallowedTools.AdditionalProperties.Properties.Reason.MinLength)
	}
}

func TestParseSpecRejects(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "unknown top level field",
			yaml: `
bogus_field: true
disallowed_tools:
  schedule:
    reason: "not allowed"
`,
			wantErr: "field bogus_field not found",
		},
		{
			name: "unknown entry field",
			yaml: `
disallowed_tools:
  schedule:
    reason: "not allowed"
    extra_field: 123
`,
			wantErr: "field extra_field not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s Spec
			dec := yaml.NewDecoder(strings.NewReader(tt.yaml))
			dec.KnownFields(true)
			err := dec.Decode(&s)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNormalizeToolName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"schedule", "schedule"},
		{"Schedule", "schedule"},
		{"SCHEDULE", "schedule"},
		{"default_api:schedule", "schedule"},
		{"default_api:Schedule", "schedule"},
		{"cortex_step_type_schedule", "schedule"},
		{"  Schedule  ", "schedule"},
		{"run_command", "run_command"},
	}

	for _, tt := range tests {
		got := NormalizeToolName(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeToolName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCheckDisallowedTool(t *testing.T) {
	tests := []struct {
		toolName       string
		wantDisallowed bool
	}{
		{"schedule", true},
		{"Schedule", true},
		{"default_api:schedule", true},
		{"cortex_step_type_schedule", true},
		{"run_command", false},
		{"view_file", false},
		{"write_to_file", false},
		{"unknown_tool", false},
	}

	for _, tt := range tests {
		disallowed, reason := IsDisallowedTool(tt.toolName)
		if disallowed != tt.wantDisallowed {
			t.Errorf("IsDisallowedTool(%q) = %v (reason %q), want %v", tt.toolName, disallowed, reason, tt.wantDisallowed)
		}
		if disallowed && reason == "" {
			t.Errorf("IsDisallowedTool(%q) returned empty reason for disallowed tool", tt.toolName)
		}
	}
}

func TestMustLoadSpecPanicsOnInvalid(t *testing.T) {
	var s Spec
	dec := yaml.NewDecoder(bytes.NewReader([]byte("bogus: true")))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err == nil {
		t.Fatal("expected decode failure on unknown field")
	}
}

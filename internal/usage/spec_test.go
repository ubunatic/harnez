package usage

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez"
)

func TestEmbeddedUsageSpec(t *testing.T) {
	age, err := agyMeterMaxAge()
	if err != nil {
		t.Fatal(err)
	}
	if age != 60*time.Minute {
		t.Fatalf("AGY meter max age = %s, want 1h from embedded spec", age)
	}
	mode, err := DefaultUsageViewMode()
	if err != nil {
		t.Fatal(err)
	}
	if mode != "normal" {
		t.Fatalf("default usage view = %q, want normal from embedded spec", mode)
	}
}

func TestUsageSpecSchemaDeclaresAgeSetting(t *testing.T) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/usage.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Required    []string       `json:"required"`
		Properties  map[string]any `json:"properties"`
		Definitions map[string]struct {
			Properties map[string]struct {
				MinItems int `json:"minItems"`
			} `json:"properties"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	if len(schema.Required) != 7 || schema.Properties["watch_loading_label"] == nil || schema.Properties["agy_meter_max_age"] == nil || schema.Properties["collector_cadence"] == nil || schema.Properties["collector_timeout"] == nil || schema.Properties["passive_dedupe_interval"] == nil || schema.Properties["statusline_busy_timeout"] == nil || schema.Properties["view_modes"] == nil {
		t.Fatalf("usage schema does not define required agy_meter_max_age: %+v", schema)
	}
	if schema.Definitions["viewMode"].Properties["panels"].MinItems != 1 {
		t.Fatalf("usage schema must reject empty mode panel lists: %+v", schema.Definitions["viewMode"])
	}
}

func TestParseUsageSpecRejectsInvalidAge(t *testing.T) {
	valid := `agy_meter_max_age: 1h
collector_cadence: 15m
collector_timeout: 2m
passive_dedupe_interval: 30s
statusline_busy_timeout: 25ms
watch_loading_label: Collecting usage
view_modes:
  default: normal
  modes:
    normal: &mode
      panels: [claude]
      token_details: true
      title_bar: true
      status_bar: true
      hidden_list: true
      overflow_hint: true
    compact: *mode
    minimal: *mode
    dashboard:
      panels: [claude]
      token_details: true
      title_bar: true
      status_bar: true
      hidden_list: false
      overflow_hint: true
`
	invalid := []struct {
		name string
		data string
	}{
		{name: "zero age", data: strings.Replace(valid, "agy_meter_max_age: 1h", "agy_meter_max_age: 0s", 1)},
		{name: "invalid age", data: strings.Replace(valid, "agy_meter_max_age: 1h", "agy_meter_max_age: not-a-duration", 1)},
		{name: "unknown field", data: valid + "unknown: true\n"},
		{name: "zero cadence", data: strings.Replace(valid, "collector_cadence: 15m", "collector_cadence: 0s", 1)},
		{name: "missing mode field", data: strings.Replace(valid, "      token_details: true\n", "", 1)},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, _, _, _, err := parseUsageSpec([]byte(tc.data)); err == nil {
				t.Fatalf("parseUsageSpec() succeeded, want validation error")
			}
		})
	}
}

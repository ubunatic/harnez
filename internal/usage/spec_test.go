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
	if len(schema.Required) != 12 || schema.Properties["nvidia_smi_timeout"] == nil || schema.Properties["nvidia_smi_cache_ttl"] == nil || schema.Properties["export_host_key"] == nil || schema.Properties["history_snapshot_interval"] == nil || schema.Properties["watch_loading_label"] == nil || schema.Properties["agy_meter_max_age"] == nil || schema.Properties["collector_cadence"] == nil || schema.Properties["collector_timeout"] == nil || schema.Properties["passive_dedupe_interval"] == nil || schema.Properties["statusline_busy_timeout"] == nil || schema.Properties["statusline_write_timeout"] == nil || schema.Properties["view_modes"] == nil {
		t.Fatalf("usage schema does not define required agy_meter_max_age: %+v", schema)
	}
	if schema.Definitions["viewMode"].Properties["panels"].MinItems != 1 {
		t.Fatalf("usage schema must reject empty mode panel lists: %+v", schema.Definitions["viewMode"])
	}
}

func TestPassivePolicyLoadsStatuslineWriteTimeout(t *testing.T) {
	_, _, writeTimeout, err := PassivePolicy()
	if err != nil {
		t.Fatal(err)
	}
	if writeTimeout != 100*time.Millisecond {
		t.Fatalf("statusline write timeout = %s, want 100ms from embedded spec", writeTimeout)
	}
}

func TestParseUsageSpecRejectsInvalidAge(t *testing.T) {
	valid := `agy_meter_max_age: 1h
collector_cadence: 15m
collector_timeout: 2m
passive_dedupe_interval: 30s
history_snapshot_interval: 15m
export_host_key: export-host-key
statusline_busy_timeout: 25ms
statusline_write_timeout: 100ms
nvidia_smi_timeout: 1s
nvidia_smi_cache_ttl: 2s
watch_loading_label: Collecting usage
view_modes:
  default: normal
  modes:
    normal: &mode
      panels: [table]
      token_details: true
      title_bar: true
      status_bar: true
      hidden_list: true
      overflow_hint: true
    compact: *mode
    minimal: *mode
`
	if _, _, _, _, _, err := parseUsageSpec([]byte(valid)); err != nil {
		t.Fatalf("parseUsageSpec(valid) = %v, want the base fixture to parse", err)
	}
	invalid := []struct {
		name string
		data string
	}{
		{name: "zero age", data: strings.Replace(valid, "agy_meter_max_age: 1h", "agy_meter_max_age: 0s", 1)},
		{name: "invalid age", data: strings.Replace(valid, "agy_meter_max_age: 1h", "agy_meter_max_age: not-a-duration", 1)},
		{name: "unknown field", data: valid + "unknown: true\n"},
		{name: "empty export host key", data: strings.Replace(valid, "export_host_key: export-host-key", "export_host_key: ''", 1)},
		{name: "zero history snapshot interval", data: strings.Replace(valid, "history_snapshot_interval: 15m", "history_snapshot_interval: 0s", 1)},
		{name: "zero cadence", data: strings.Replace(valid, "collector_cadence: 15m", "collector_cadence: 0s", 1)},
		{name: "invalid statusline write timeout", data: strings.Replace(valid, "statusline_write_timeout: 100ms", "statusline_write_timeout: nope", 1)},
		{name: "missing statusline write timeout", data: strings.Replace(valid, "statusline_write_timeout: 100ms\n", "", 1)},
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

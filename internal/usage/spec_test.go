package usage

import (
	"encoding/json"
	"io/fs"
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
}

func TestUsageSpecSchemaDeclaresAgeSetting(t *testing.T) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/usage.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Required   []string       `json:"required"`
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	if len(schema.Required) != 3 || schema.Properties["agy_meter_max_age"] == nil || schema.Properties["collector_cadence"] == nil || schema.Properties["collector_timeout"] == nil {
		t.Fatalf("usage schema does not define required agy_meter_max_age: %+v", schema)
	}
}

func TestParseUsageSpecRejectsInvalidAge(t *testing.T) {
	for _, data := range [][]byte{
		[]byte("agy_meter_max_age: 0s\n"),
		[]byte("agy_meter_max_age: not-a-duration\n"),
		[]byte("agy_meter_max_age: 1h\ncollector_cadence: 15m\ncollector_timeout: 2m\nunknown: true\n"),
		[]byte("agy_meter_max_age: 1h\ncollector_cadence: 0s\ncollector_timeout: 2m\n"),
	} {
		if _, _, _, err := parseUsageSpec(data); err == nil {
			t.Errorf("parseUsageSpec(%q) succeeded, want validation error", data)
		}
	}
}

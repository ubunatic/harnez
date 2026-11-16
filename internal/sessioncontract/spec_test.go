package sessioncontract

import (
	"strings"
	"testing"

	"ubunatic.com/harnez"
)

func TestContractRejectsInvalidDocuments(t *testing.T) {
	data, err := harnez.DefaultFS.ReadFile("spec/session_isolation.yaml")
	if err != nil {
		t.Fatal(err)
	}
	original := string(data)
	cases := map[string]string{
		"unknown field":            original + "unknown: true\n",
		"missing operations":       strings.Replace(original, "runner_operations: [telemetry_ingest, telemetry_replay, usage_read, own_register, own_update, own_query]\n", "", 1),
		"empty operations":         strings.Replace(original, "runner_operations: [telemetry_ingest, telemetry_replay, usage_read, own_register, own_update, own_query]", "runner_operations: []", 1),
		"stripped host exclusions": strings.Replace(original, "host_only_operations: [/shutdown, merge_legacy, arbitrary_paths, arbitrary_pids, cross_session_stop, cross_session_resume, global_policy_write]", "host_only_operations: [/shutdown]", 1),
		"duplicate resource":       strings.Replace(original, "  - {resource: exec_process_records, class: runner}", "  - {resource: session_locks_history, class: broker}", 1),
		"duplicate operation":      strings.Replace(original, "telemetry_ingest, telemetry_replay", "telemetry_ingest, telemetry_ingest", 1),
		"conflicting operation":    strings.Replace(original, "global_policy_write]", "global_policy_write, usage_read]", 1),
		"invalid class":            strings.Replace(original, "class: runner", "class: unknown", 1),
	}
	if _, err := decode(data); err != nil {
		t.Fatalf("valid contract: %v", err)
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := decode([]byte(input)); err == nil {
				t.Fatal("accepted invalid contract")
			}
		})
	}
}

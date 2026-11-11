package subagent

import (
	"testing"
)

func TestToolCallLimitLookup(t *testing.T) {
	data := []byte(`
default_model: test:test
default_role: developer
role_classify_timeout_ms: 100
reply_history_limit: 10
compact_threshold_tokens: 1000
auto_compact:
  default_mode: manual
  provider_modes: {claude: manual}
  watchdog_grace_ms: 1000
  legacy_growth_threshold_tokens: 1000
  codex_on_demand_limit_tokens: 1000
  codex_status_prompt: "status"
pi_compaction:
  small_window_threshold_tokens: 1000
  reserve_fraction: 0.1
  keep_recent_fraction: 0.1
context_modes:
  default: {description: d}
  plain: {description: d}
  no-skills: {description: d}
models:
  test:test: {provider: test, name: test, tier: low}
roles:
  developer:
    description: dev
    rules: rule
external_sessions:
  test:
    root: .
    pattern: "*/*/*/rollout-*.jsonl"
    format: jsonl
    record_type_path: type
    record_type: session_meta
    max_record_bytes: 1048576
    active_match_window_seconds: 120
    name_prefix: codex
    available_status: available
    fields:
      id: payload.id
      provider_session_id: payload.session_id
      working_dir: payload.cwd
      model_provider: payload.model_provider
      source: payload.source
      created_at: payload.timestamp
tool_call_limits:
  default: 10
  models:
    test:override: {default: 20, reviewer: 30}
  native_backstop_calls: 20
  synthesis_prompt: "prompt"
jules:
  base_url: https://jules.googleapis.com/v1alpha
  config_path: ~/.jules/config.json
  poll_interval_ms: 30000
  wait_timeout_ms: 1500000
  request_timeout_ms: 30000
  max_poll_errors: 5
  completed_pr_polls: 3
  sync_remote: origin
  sync_branch: main
  sync_timeout_ms: 60000
`)
	spec, err := parseAgentSpec(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if spec.ToolCallLimits.Default != 10 {
		t.Errorf("expected default 10, got %d", spec.ToolCallLimits.Default)
	}

	check := func(model, role string) int {
		if overrides, ok := spec.ToolCallLimits.Models[model]; ok {
			if limit, ok := overrides[role]; ok {
				return limit
			}
			if defaultLimit, ok := overrides["default"]; ok {
				return defaultLimit
			}
		}
		return spec.ToolCallLimits.Default
	}

	if limit := check("unknown:model", "reviewer"); limit != 10 {
		t.Errorf("expected 10 for unknown model, got %d", limit)
	}
	if limit := check("test:override", "developer"); limit != 20 {
		t.Errorf("expected 20 for test:override developer, got %d", limit)
	}
	if limit := check("test:override", "reviewer"); limit != 30 {
		t.Errorf("expected 30 for test:override reviewer, got %d", limit)
	}
}

package subagent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolCallLimiter(t *testing.T) {
	t.Run("zero limit means off", func(t *testing.T) {
		l := NewToolCallLimiter(0)
		for i := 0; i < 5; i++ {
			if l.Observe(Event{ToolCall: true, CallID: string(rune(i))}) {
				t.Error("expected 0 limit to not block")
			}
		}
		if l.Reached() {
			t.Error("expected Reached to be false")
		}
		if l.Attempted() != 5 {
			t.Errorf("expected 5 attempted, got %d", l.Attempted())
		}
	})

	t.Run("blocks after limit", func(t *testing.T) {
		l := NewToolCallLimiter(2)
		if l.Observe(Event{ToolCall: true, CallID: "1"}) {
			t.Error("expected first call to not block")
		}
		if l.Observe(Event{ToolCall: true, CallID: "2"}) {
			t.Error("expected second call to not block")
		}
		if !l.Observe(Event{ToolCall: true, CallID: "3"}) {
			t.Error("expected third call to block")
		}
		if !l.Reached() {
			t.Error("expected Reached to be true")
		}
		if l.Attempted() != 3 {
			t.Errorf("expected 3 attempted, got %d", l.Attempted())
		}
	})

	t.Run("deduplicates call IDs", func(t *testing.T) {
		l := NewToolCallLimiter(2)
		if l.Observe(Event{ToolCall: true, CallID: "1"}) {
			t.Error("expected first call to not block")
		}
		if l.Observe(Event{ToolCall: true, CallID: "1"}) {
			t.Error("expected duplicate call ID to not block")
		}
		if l.Attempted() != 1 {
			t.Errorf("expected 1 attempted, got %d", l.Attempted())
		}
	})

	t.Run("repeated event of a blocked call stays blocked", func(t *testing.T) {
		l := NewToolCallLimiter(1)
		l.Observe(Event{ToolCall: true, CallID: "a"})
		if !l.Observe(Event{ToolCall: true, CallID: "b"}) {
			t.Fatal("expected call over the limit to block")
		}
		if !l.Observe(Event{ToolCall: true, CallID: "b"}) {
			t.Error("expected repeated event of the blocked call to block again")
		}
		if l.Observe(Event{ToolCall: true, CallID: "a"}) {
			t.Error("expected repeated event of an allowed call to stay allowed")
		}
		if l.Attempted() != 2 {
			t.Errorf("expected 2 attempted, got %d", l.Attempted())
		}
	})

	t.Run("ignores non-tool calls", func(t *testing.T) {
		l := NewToolCallLimiter(1)
		if l.Observe(Event{ToolCall: false}) {
			t.Error("expected non-tool call to not block")
		}
		if l.Attempted() != 0 {
			t.Errorf("expected 0 attempted, got %d", l.Attempted())
		}
	})
}

func TestEffectiveToolCallConfigPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	lm := Model{Provider: "lmcoder", Name: "coder", Tier: "low"}

	// Baseline from spec/agent.yaml.
	cfg, err := EffectiveToolCallConfig(lm, "reviewer", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Limit != 15 || cfg.NativeBackstopCalls <= 0 || strings.TrimSpace(cfg.SynthesisPrompt) == "" {
		t.Fatalf("unexpected spec baseline: %+v", cfg)
	}

	// ~/.harnez/config.yaml overrides spec.
	harnezDir := filepath.Join(home, ".harnez")
	if err := os.MkdirAll(harnezDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgYAML := `agent:
  tool_call_limits:
    default: 9
    native_backstop_calls: 7
    synthesis_prompt: "custom synthesis"
    models:
      lmcoder:coder:
        reviewer: 4
        default: 6
`
	if err := os.WriteFile(filepath.Join(harnezDir, "config.yaml"), []byte(cfgYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = EffectiveToolCallConfig(lm, "reviewer", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Limit != 4 || cfg.NativeBackstopCalls != 7 || cfg.SynthesisPrompt != "custom synthesis" {
		t.Fatalf("user config override = %+v", cfg)
	}

	// --max-tool-calls 0 explicitly disables even when user config sets 4.
	cfg, err = EffectiveToolCallConfig(lm, "reviewer", 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Limit != 0 {
		t.Fatalf("flag 0 override = %d, want 0", cfg.Limit)
	}

	// Negative flag rejected.
	if _, err := EffectiveToolCallConfig(lm, "reviewer", -1, true); err == nil {
		t.Fatal("expected error for negative --max-tool-calls")
	}
}

func TestPiEmptyResponseAndToolExecutionEvent(t *testing.T) {
	var events []Event
	line := `{"type":"tool_execution_start","toolCallId":"pi-tc-1","toolName":"read"}`
	piLineFunc(func(e Event) { events = append(events, e) })([]byte(line))
	if len(events) != 1 || !events[0].ToolCall || events[0].CallID != "pi-tc-1" {
		t.Fatalf("piLineFunc events = %+v", events)
	}

	raw := strings.Join([]string{
		`{"type":"session","id":"pi-sess-1"}`,
		`{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"partial finding"}],"usage":{"input":10,"output":5,"cacheRead":2,"cacheWrite":0,"totalTokens":17}}}`,
		`{"type":"message_end","message":{"role":"assistant","content":[],"stopReason":"stop","usage":{"input":12,"output":1,"cacheRead":0,"cacheWrite":0,"totalTokens":13}}}`,
		`{"type":"agent_settled"}`,
	}, "\n")
	res, err := parsePi([]byte(raw))
	if res != nil {
		t.Fatalf("expected nil TurnResult on empty final response, got %+v", res)
	}
	var emptyErr *PiEmptyResponseError
	if !errors.As(err, &emptyErr) || emptyErr.Result == nil {
		t.Fatalf("expected PiEmptyResponseError with partial result, got %v", err)
	}
	if len(emptyErr.Result.Messages) != 1 || emptyErr.Result.Messages[0] != "partial finding" || emptyErr.Result.Response != "" {
		t.Fatalf("partial messages mismatch: %+v", emptyErr.Result)
	}
}

func TestClaudeAndCodexToolCallEvents(t *testing.T) {
	// Claude emits tool_use with CallID for main thread only, excludes subagents.
	cs := &claudeStreamer{}
	claudeEvents := cs.feed([]byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"ls"}}]}}`))
	claudeEvents = append(claudeEvents, cs.feed([]byte(`{"type":"assistant","parent_tool_use_id":"toolu_parent","message":{"content":[{"type":"tool_use","id":"toolu_sub","name":"Read","input":{}}]}}`))...)
	toolCalls := 0
	for _, ev := range claudeEvents {
		if ev.ToolCall {
			toolCalls++
			if ev.CallID != "toolu_1" {
				t.Fatalf("unexpected claude CallID = %q", ev.CallID)
			}
		}
	}
	if toolCalls != 1 {
		t.Fatalf("claude tool calls = %d, want 1 (%+v)", toolCalls, claudeEvents)
	}

	// Codex maps tool item types across started+completed and deduplicates via ToolCallLimiter.
	cp := &codexParser{}
	var codexEvents []Event
	for _, rawLine := range []string{
		`{"type":"item.started","item":{"id":"cmd1","type":"command_execution","command":"pwd"}}`,
		`{"type":"item.completed","item":{"id":"cmd1","type":"command_execution","command":"pwd"}}`,
		`{"type":"item.started","item":{"id":"mcp1","type":"mcp_tool_call"}}`,
		`{"type":"item.completed","item":{"id":"fc1","type":"file_change"}}`,
		`{"type":"item.completed","item":{"id":"msg1","type":"agent_message","text":"done"}}`,
	} {
		if ev, ok := cp.feed([]byte(rawLine)); ok {
			codexEvents = append(codexEvents, ev)
		}
	}
	limiter := NewToolCallLimiter(10)
	for _, ev := range codexEvents {
		limiter.Observe(ev)
	}
	if limiter.Attempted() != 3 {
		t.Fatalf("codex unique tool calls = %d, want 3 (events=%+v)", limiter.Attempted(), codexEvents)
	}
}

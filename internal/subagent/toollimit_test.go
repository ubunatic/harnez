package subagent

import (
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

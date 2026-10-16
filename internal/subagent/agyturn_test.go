package subagent

import (
	"testing"
	"time"
)

func TestTurnStateIdleRequiresCompletedRequestAndQuietWindow(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	state := NewTurnState(func() time.Time { return now })
	quiet := time.Second

	if !state.Idle(now, quiet, 0) {
		t.Fatal("new state should be idle")
	}
	state.RequestStarted()
	if state.Idle(now.Add(time.Hour), quiet, 0) {
		t.Fatal("in-flight request should prevent idle")
	}
	state.ResponseFinished(false)
	if state.Idle(now.Add(quiet-time.Nanosecond), quiet, 0) {
		t.Fatal("state became idle before quiet window")
	}
	if !state.Idle(now.Add(quiet), quiet, 0) {
		t.Fatal("state should be idle at quiet-window boundary")
	}
}

func TestTurnStateFunctionCallRequiresFinalResponse(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	state := NewTurnState(func() time.Time { return now })
	state.RequestStarted()
	state.ResponseFinished(true)
	if state.Idle(now.Add(time.Hour), 0, 0) {
		t.Fatal("functionCall response should not end the turn")
	}

	now = now.Add(time.Second)
	state.RequestStarted()
	state.ResponseFinished(false)
	if !state.Idle(now, 0, 0) {
		t.Fatal("final response without functionCall should end the turn")
	}
}

func TestTurnStateOpenToolsPreventIdleAndRestartQuietWindow(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	state := NewTurnState(func() time.Time { return now })
	quiet := 2 * time.Second
	state.ToolStarted()
	state.ToolStarted()
	state.ToolFinished()
	if state.Idle(now.Add(time.Hour), quiet, 0) {
		t.Fatal("remaining open tool should prevent idle")
	}
	state.ToolFinished()
	if state.Idle(now.Add(quiet-time.Nanosecond), quiet, 0) {
		t.Fatal("tool completion should restart quiet window")
	}
	if !state.Idle(now.Add(quiet), quiet, 0) {
		t.Fatal("state should become idle after tools finish and quiet window elapses")
	}
}

func TestTurnStateKeystrokeTypingGuard(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	state := NewTurnState(func() time.Time { return now })
	typingGuard := 3 * time.Second
	state.Keystroke()
	if state.Idle(now.Add(typingGuard-time.Nanosecond), 0, typingGuard) {
		t.Fatal("recent keystroke should prevent idle")
	}
	if !state.Idle(now.Add(typingGuard), 0, typingGuard) {
		t.Fatal("state should be idle at typing-guard boundary")
	}
}

func TestTurnStateQuietAndTypingWindowsAreIndependent(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	state := NewTurnState(func() time.Time { return now })
	state.RequestStarted()
	state.ResponseFinished(false)
	state.Keystroke()
	if state.Idle(now.Add(time.Second), 2*time.Second, 0) {
		t.Fatal("quiet window should prevent idle even after typing guard")
	}
	if state.Idle(now.Add(2*time.Second), 0, 3*time.Second) {
		t.Fatal("typing guard should prevent idle even after quiet window")
	}
	if !state.Idle(now.Add(3*time.Second), 2*time.Second, 3*time.Second) {
		t.Fatal("state should become idle after both windows elapse")
	}
}

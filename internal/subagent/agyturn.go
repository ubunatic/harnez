package subagent

import (
	"sync"
	"time"
)

// TurnState tracks whether an interactive AGY session is safe to receive a
// queued prompt.
type TurnState struct {
	mu               sync.Mutex
	inflight         int
	openTools        int
	lastFunctionCall bool
	lastEvent        time.Time
	lastKey          time.Time
	now              func() time.Time
}

// NewTurnState creates a turn state using clock. A nil clock uses time.Now.
func NewTurnState(clock func() time.Time) *TurnState {
	if clock == nil {
		clock = time.Now
	}
	return &TurnState{now: clock}
}

func (s *TurnState) RequestStarted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inflight++
	s.lastEvent = s.now()
}

func (s *TurnState) ResponseFinished(functionCall bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inflight > 0 {
		s.inflight--
	}
	s.lastFunctionCall = functionCall
	s.lastEvent = s.now()
}

func (s *TurnState) ToolStarted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.openTools++
	s.lastEvent = s.now()
}

func (s *TurnState) ToolFinished() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.openTools > 0 {
		s.openTools--
	}
	s.lastEvent = s.now()
}

func (s *TurnState) Keystroke() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastKey = s.now()
}

// Idle reports whether the turn is complete and both quiet windows elapsed.
func (s *TurnState) Idle(now time.Time, quiet, typingGuard time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inflight > 0 || s.openTools > 0 || s.lastFunctionCall {
		return false
	}
	if !s.lastEvent.IsZero() && now.Sub(s.lastEvent) < quiet {
		return false
	}
	if !s.lastKey.IsZero() && now.Sub(s.lastKey) < typingGuard {
		return false
	}
	return true
}

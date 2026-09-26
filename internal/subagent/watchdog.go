package subagent

import (
	"context"
	"fmt"
	"syscall"
	"time"
)

// TokenMonitor extracts a live full-context token count from a provider event.
type TokenMonitor interface {
	ContextTokens(Event) (int, bool)
}

// StreamTokenMonitor uses explicit provider token-count events, never byte estimates.
type StreamTokenMonitor struct{}

func (StreamTokenMonitor) ContextTokens(event Event) (int, bool) {
	return event.ContextTokens, event.ContextTokens > 0
}

// ProcessKiller signals an agent's isolated process group.
type ProcessKiller interface {
	KillGroup(pgid int, signal syscall.Signal) error
}

type systemProcessKiller struct{}

func (systemProcessKiller) KillGroup(pgid int, signal syscall.Signal) error {
	return KillGroup(pgid, signal)
}

// WatchdogClock makes grace-period handling deterministic in tests.
type WatchdogClock interface {
	After(time.Duration) <-chan time.Time
}

type realWatchdogClock struct{}

func (realWatchdogClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// TokenWatchdog detects a live token threshold crossing and controls process-group stop.
type TokenWatchdog struct {
	Threshold int
	Grace     time.Duration
	Monitor   TokenMonitor
	Killer    ProcessKiller
	Clock     WatchdogClock
	crossed   bool
}

// Observe returns true once when an explicit streamed token count reaches the threshold.
func (w *TokenWatchdog) Observe(event Event) (int, bool) {
	if w == nil || w.crossed || w.Threshold <= 0 {
		return 0, false
	}
	monitor := w.Monitor
	if monitor == nil {
		monitor = StreamTokenMonitor{}
	}
	tokens, ok := monitor.ContextTokens(event)
	if !ok || tokens < w.Threshold {
		return tokens, false
	}
	w.crossed = true
	return tokens, true
}

// StopGroup sends an interrupt and escalates to SIGKILL if the group misses its grace period.
func (w *TokenWatchdog) StopGroup(ctx context.Context, pgid int, done <-chan struct{}) (bool, error) {
	if pgid <= 1 {
		return false, fmt.Errorf("refusing to signal invalid process group %d", pgid)
	}
	killer := w.Killer
	if killer == nil {
		killer = systemProcessKiller{}
	}
	clock := w.Clock
	if clock == nil {
		clock = realWatchdogClock{}
	}
	grace := w.Grace
	if grace <= 0 {
		grace = 20 * time.Second
	}
	if err := killer.KillGroup(pgid, syscall.SIGINT); err != nil {
		return false, fmt.Errorf("interrupt process group %d: %w", pgid, err)
	}
	select {
	case <-done:
		return false, nil
	default:
	}
	select {
	case <-done:
		return false, nil
	case <-ctx.Done():
		return false, ctx.Err()
	case <-clock.After(grace):
		if err := killer.KillGroup(pgid, syscall.SIGKILL); err != nil {
			return true, fmt.Errorf("kill process group %d after grace timeout: %w", pgid, err)
		}
		return true, nil
	}
}

// KillGroup signals a process group created with Setpgid.
func KillGroup(pgid int, signal syscall.Signal) error {
	if pgid <= 1 {
		return fmt.Errorf("refusing to signal invalid process group %d", pgid)
	}
	return syscall.Kill(-pgid, signal)
}

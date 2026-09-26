package subagent

import (
	"context"
	"reflect"
	"syscall"
	"testing"
	"time"
)

type recordingProcessKiller struct{ signals []syscall.Signal }

func (k *recordingProcessKiller) KillGroup(_ int, signal syscall.Signal) error {
	k.signals = append(k.signals, signal)
	return nil
}

type immediateWatchdogClock struct{}

func (immediateWatchdogClock) After(time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	ch <- time.Now()
	return ch
}

func TestTokenWatchdogObservesThresholdOnce(t *testing.T) {
	w := &TokenWatchdog{Threshold: 100}
	for _, event := range []Event{{ContextTokens: 99}, {ContextTokens: 100}, {ContextTokens: 200}} {
		_, crossed := w.Observe(event)
		if crossed != (event.ContextTokens == 100) {
			t.Fatalf("Observe(%d) crossed=%v", event.ContextTokens, crossed)
		}
	}
}

func TestTokenWatchdogEscalatesAfterGrace(t *testing.T) {
	killer := &recordingProcessKiller{}
	w := &TokenWatchdog{Grace: time.Second, Killer: killer, Clock: immediateWatchdogClock{}}
	killed, err := w.StopGroup(context.Background(), 42, make(chan struct{}))
	if err != nil || !killed {
		t.Fatalf("StopGroup() = (%v, %v), want (true, nil)", killed, err)
	}
	if want := []syscall.Signal{syscall.SIGINT, syscall.SIGKILL}; !reflect.DeepEqual(killer.signals, want) {
		t.Fatalf("signals = %v, want %v", killer.signals, want)
	}
}

func TestTokenWatchdogDoesNotEscalateWhenGroupStops(t *testing.T) {
	killer := &recordingProcessKiller{}
	w := &TokenWatchdog{Killer: killer, Clock: immediateWatchdogClock{}}
	done := make(chan struct{})
	close(done)
	killed, err := w.StopGroup(context.Background(), 42, done)
	if err != nil || killed {
		t.Fatalf("StopGroup() = (%v, %v), want (false, nil)", killed, err)
	}
	if want := []syscall.Signal{syscall.SIGINT}; !reflect.DeepEqual(killer.signals, want) {
		t.Fatalf("signals = %v, want %v", killer.signals, want)
	}
}

func TestKillGroupRejectsUnsafePGID(t *testing.T) {
	if err := KillGroup(1, syscall.SIGKILL); err == nil {
		t.Fatal("KillGroup(1) succeeded; want validation error")
	}
}

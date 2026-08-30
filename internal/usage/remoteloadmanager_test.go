package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// withShortRemoteLoadRetry shrinks remoteLoadRetryInterval for the duration
// of a test, restoring it afterward, so tests exercise the manager's
// fallback/retry loop in milliseconds instead of the real 10s cadence.
func withShortRemoteLoadRetry(t *testing.T, d time.Duration) {
	t.Helper()
	orig := remoteLoadRetryInterval
	remoteLoadRetryInterval = d
	t.Cleanup(func() { remoteLoadRetryInterval = orig })
}

// TestRunRemoteLoadManager_InvalidHostFallsBackWithoutLeakingStop exercises
// the manager's control flow when both the streaming and batch paths fail
// immediately (an invalid/empty host, so neither ever spawns a real ssh
// process) — it must keep retrying on remoteLoadRetryInterval, never call
// setStop with a non-nil func (nothing to clean up, since nothing ever
// connected), and return promptly once ctx is cancelled.
func TestRunRemoteLoadManager_InvalidHostFallsBackWithoutLeakingStop(t *testing.T) {
	withShortRemoteLoadRetry(t, 5*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	var last *LoadSnapshot
	redraws := 0
	var stopCalls []func()

	done := make(chan struct{})
	go func() {
		defer close(done)
		runRemoteLoadManager(ctx, "", &mu, &last, func() {
			mu.Lock()
			redraws++
			mu.Unlock()
		}, func(f func()) {
			mu.Lock()
			stopCalls = append(stopCalls, f)
			mu.Unlock()
		})
	}()

	time.Sleep(40 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runRemoteLoadManager did not return after ctx cancellation")
	}

	mu.Lock()
	defer mu.Unlock()
	if last != nil {
		t.Fatalf("expected no snapshot to ever be set for an invalid host, got %+v", last)
	}
	for i, f := range stopCalls {
		if f != nil {
			t.Fatalf("setStop call %d was non-nil for a host that never streamed successfully", i)
		}
	}
}

// TestRunRemoteLoadManager_StreamSamplesDeliveredThenFallback fakes a
// successful ControlMaster + streaming child (same mocking approach as
// loadstream_test.go) that emits one sample and exits, then verifies the
// manager (a) delivers that sample via setLast/redraw, (b) called setStop
// with a real stop func while the stream was live, (c) calls setStop(nil)
// once the stream ends, and (d) tears the ControlMaster down via exactly
// one ssh -O exit per stream instance — no leaked master across the
// fallback-then-retry cycle.
func TestRunRemoteLoadManager_StreamSamplesDeliveredThenFallback(t *testing.T) {
	withShortRemoteLoadRetry(t, 20*time.Millisecond)

	origStart := sshControlMasterStartCmd
	origExit := sshControlMasterExitCmd
	origChild := sshLoadStreamChildCmd
	defer func() {
		sshControlMasterStartCmd = origStart
		sshControlMasterExitCmd = origExit
		sshLoadStreamChildCmd = origChild
	}()

	var exitCalls int
	var exitMu sync.Mutex
	sshControlMasterStartCmd = func(ctlPath, host string) *exec.Cmd { return exec.Command("true") }
	sshControlMasterExitCmd = func(ctlPath, host string) *exec.Cmd {
		exitMu.Lock()
		exitCalls++
		exitMu.Unlock()
		return exec.Command("true")
	}

	snap := LoadSnapshot{CPU: CPULoad{NumCPU: 16, Ok: true}}
	data, _ := json.Marshal(snap)
	script := fakeScript(t, fmt.Sprintf("echo '%s'\n", string(data)))
	sshLoadStreamChildCmd = func(ctx context.Context, ctlPath, host string) *exec.Cmd {
		return exec.CommandContext(ctx, script)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	var last *LoadSnapshot
	gotSample := make(chan struct{}, 1)
	var stopSeen, nilStopSeen bool

	managerDone := make(chan struct{})
	go func() {
		defer close(managerDone)
		runRemoteLoadManager(ctx, "um760", &mu, &last, func() {
			mu.Lock()
			if last != nil {
				select {
				case gotSample <- struct{}{}:
				default:
				}
			}
			mu.Unlock()
		}, func(f func()) {
			mu.Lock()
			if f != nil {
				stopSeen = true
			} else if stopSeen {
				nilStopSeen = true
			}
			mu.Unlock()
		})
	}()

	select {
	case <-gotSample:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a streamed sample to be applied")
	}

	mu.Lock()
	if last == nil || last.CPU.NumCPU != 16 {
		t.Fatalf("unexpected snapshot: %+v", last)
	}
	mu.Unlock()

	// Give the manager time to notice the child exited (one-line script,
	// no further output) and cycle through setStop(nil) + one fallback
	// poll/retry.
	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		ok := stopSeen && nilStopSeen
		mu.Unlock()
		if ok {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for setStop(nil) after stream end")
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	select {
	case <-managerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("runRemoteLoadManager did not return after ctx cancellation")
	}

	exitMu.Lock()
	got := exitCalls
	exitMu.Unlock()
	if got != 1 {
		t.Fatalf("expected exactly 1 ssh -O exit invocation, got %d", got)
	}
}

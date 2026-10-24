package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestRunRemoteLoadManager_InvalidHostFallsBackWithoutLeakingStop exercises
// the manager's control flow when both the streaming and batch paths fail
// immediately (an invalid/empty host, so neither ever spawns a real ssh
// process) — it must keep retrying on its injected policy, never call
// setStop with a non-nil func (nothing to clean up, since nothing ever
// connected), and return promptly once ctx is cancelled.
func TestRunRemoteLoadManager_InvalidHostFallsBackWithoutLeakingStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var last *LoadSnapshot
	redraws := 0
	var stopCalls []func()
	streamAttempts := make(chan struct{}, 10)

	done := make(chan struct{})
	go func() {
		defer close(done)
		runRemoteLoadManagerWithPolicy(ctx, "", &mu, &last, func() {
			mu.Lock()
			redraws++
			mu.Unlock()
		}, func(f func()) {
			mu.Lock()
			stopCalls = append(stopCalls, f)
			mu.Unlock()
		}, func(bool) {
			select {
			case streamAttempts <- struct{}{}:
			default:
			}
		}, 5*time.Millisecond, 5*time.Millisecond, waitOrDone, CollectRemoteLoadSnapshot)
	}()

	for range 2 {
		select {
		case <-streamAttempts:
		case <-time.After(2 * time.Second):
			t.Fatal("manager did not retry stream setup")
		}
	}
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
	fallbackSeen := make(chan struct{}, 1)
	var stopSeen bool
	var streamingTrueSeen, streamingFalseAfterTrueSeen bool

	managerDone := make(chan struct{})
	go func() {
		defer close(managerDone)
		runRemoteLoadManagerWithPolicy(ctx, "isolated-host", &mu, &last, func() {
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
				select {
				case fallbackSeen <- struct{}{}:
				default:
				}
			}
			mu.Unlock()
		}, func(streaming bool) {
			mu.Lock()
			if streaming {
				streamingTrueSeen = true
			} else if streamingTrueSeen {
				streamingFalseAfterTrueSeen = true
			}
			mu.Unlock()
		}, 20*time.Millisecond, 20*time.Millisecond, waitOrDone, func(context.Context, string) (*LoadSnapshot, error) {
			return nil, fmt.Errorf("test poll failure")
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
	select {
	case <-fallbackSeen:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for setStop(nil) after stream end")
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

	mu.Lock()
	defer mu.Unlock()
	if !streamingTrueSeen {
		t.Error("expected setStreaming(true) to be called while the stream was live")
	}
	if !streamingFalseAfterTrueSeen {
		t.Error("expected setStreaming(false) to be called once the stream ended (fallback mode)")
	}
}

func TestRunRemoteLoadManager_FakeSSHRecoversWithBackoff(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	countFile := filepath.Join(t.TempDir(), "ssh-count")
	ssh := filepath.Join(bin, "ssh")
	script := `#!/bin/sh
if test "$1" = "-M"; then exit 255; fi
count=0
if test -f "$HARNEZ_SSH_COUNT"; then read count < "$HARNEZ_SSH_COUNT"; fi
count=$((count + 1))
printf '%s\n' "$count" > "$HARNEZ_SSH_COUNT"
if test "$count" -lt 5; then exit 255; fi
printf '%s\n' '{"load":{"cpu":{"NumCPU":23,"Ok":true}}}'
`
	if err := os.WriteFile(ssh, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg-data"))
	t.Setenv("PATH", bin)
	t.Setenv("HARNEZ_SSH_COUNT", countFile)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var last *LoadSnapshot
	var pollErr error
	var pollSnapshot *LoadSnapshot
	delays := make(chan time.Duration)
	release := make(chan struct{}, 1)
	wait := func(ctx context.Context, d time.Duration) {
		select {
		case delays <- d:
		case <-ctx.Done():
			return
		}
		select {
		case <-release:
		case <-ctx.Done():
		}
	}
	managerDone := make(chan struct{})
	collect := func(ctx context.Context, host string) (*LoadSnapshot, error) {
		snap, err := CollectRemoteLoadSnapshot(ctx, host)
		pollErr = err
		pollSnapshot = snap
		return snap, err
	}
	go func() {
		defer close(managerDone)
		runRemoteLoadManagerWithPolicy(ctx, "fake-host", &mu, &last, func() {}, func(func()) {}, func(bool) {},
			10*time.Millisecond, 40*time.Millisecond, wait, collect)
	}()

	wantDelays := []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond, 40 * time.Millisecond, 10 * time.Millisecond}
	for i, want := range wantDelays {
		select {
		case got := <-delays:
			if got != want {
				t.Fatalf("retry delay %d = %v, want %v", i+1, got, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for retry delay %d", i+1)
		}
		data, err := os.ReadFile(countFile)
		got := 0
		if err == nil {
			_, _ = fmt.Sscanf(string(data), "%d", &got)
		}
		wantCalls := i + 1
		if got != wantCalls {
			t.Fatalf("batch ssh call count at retry %d = %d, want %d", i+1, got, wantCalls)
		}
		if i == 4 {
			mu.Lock()
			recovered := last != nil && last.CPU.NumCPU == 23
			mu.Unlock()
			if !recovered {
				t.Fatalf("expected remote load snapshot after fake ssh recovered; poll snapshot: %+v, error: %v", pollSnapshot, pollErr)
			}
			cancel()
		}
		release <- struct{}{}
	}
	select {
	case <-managerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("manager did not stop after context cancellation")
	}
}

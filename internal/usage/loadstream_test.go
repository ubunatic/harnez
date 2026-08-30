package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// fakeSSHScript writes a small shell script to a temp file and returns its
// path, so tests can substitute it for the real `ssh` binary via
// sshControlMasterStartCmd/sshControlMasterExitCmd/sshLoadStreamChildCmd —
// exercising StartRemoteLoadStream's real parsing/lifecycle logic without a
// real SSH target (per the ticket's instruction to mock the exec.Command
// boundary the way agy.go's runAGYUsageCmdFn does).
func fakeScript(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/fake.sh"
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunLoadStream_StopsOnContextCancel(t *testing.T) {
	orig := loadStreamInterval
	loadStreamInterval = 5 * time.Millisecond
	defer func() { loadStreamInterval = orig }()

	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	in := strings.NewReader("") // already "empty"; blocks reads until EOF is hit immediately? see below

	done := make(chan error, 1)
	go func() { done <- RunLoadStream(ctx, &out, in) }()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunLoadStream returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunLoadStream did not return after context cancellation")
	}

	if out.Len() == 0 {
		t.Fatal("expected at least one sample line to have been written")
	}
	dec := json.NewDecoder(&out)
	var snap LoadSnapshot
	if err := dec.Decode(&snap); err != nil {
		t.Fatalf("failed to decode first NDJSON line: %v", err)
	}
}

// blockingReader never returns from Read until closed, simulating a live
// (still-open) ssh stdin pipe so the EOF-triggered shutdown path can be
// tested independently of context cancellation.
type blockingReader struct {
	closed chan struct{}
}

func (r *blockingReader) Read(p []byte) (int, error) {
	<-r.closed
	return 0, io.EOF
}

func TestRunLoadStream_StopsOnStdinEOF(t *testing.T) {
	orig := loadStreamInterval
	loadStreamInterval = 5 * time.Millisecond
	defer func() { loadStreamInterval = orig }()

	ctx := context.Background()
	var out bytes.Buffer
	in := &blockingReader{closed: make(chan struct{})}

	done := make(chan error, 1)
	go func() { done <- RunLoadStream(ctx, &out, in) }()

	time.Sleep(20 * time.Millisecond)
	close(in.closed)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunLoadStream returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunLoadStream did not return after stdin EOF")
	}
}

func TestNewControlMaster_InvalidHost(t *testing.T) {
	for _, host := range []string{"", "-oProxyCommand=x", "bad host"} {
		if _, err := newControlMaster(host); err == nil {
			t.Fatalf("newControlMaster(%q): expected error, got nil", host)
		}
	}
}

func TestNewControlMaster_StartFailure(t *testing.T) {
	origStart := sshControlMasterStartCmd
	defer func() { sshControlMasterStartCmd = origStart }()
	sshControlMasterStartCmd = func(ctlPath, host string) *exec.Cmd {
		return exec.Command("false")
	}

	_, err := newControlMaster("um760")
	if err == nil {
		t.Fatal("expected error when the ssh -M start command fails")
	}
}

func TestControlMaster_CloseIdempotentAndSafeOnNil(t *testing.T) {
	var cm *controlMaster
	cm.Close() // must not panic

	origStart := sshControlMasterStartCmd
	origExit := sshControlMasterExitCmd
	defer func() {
		sshControlMasterStartCmd = origStart
		sshControlMasterExitCmd = origExit
	}()
	sshControlMasterStartCmd = func(ctlPath, host string) *exec.Cmd { return exec.Command("true") }
	exitCalls := 0
	sshControlMasterExitCmd = func(ctlPath, host string) *exec.Cmd {
		exitCalls++
		return exec.Command("true")
	}

	cm2, err := newControlMaster("um760")
	if err != nil {
		t.Fatalf("newControlMaster: %v", err)
	}
	dir := cm2.dir
	if dir == "" {
		t.Fatal("expected control socket dir to be set")
	}
	if _, statErr := os.Stat(dir); statErr != nil {
		t.Fatalf("expected control dir to exist: %v", statErr)
	}

	cm2.Close()
	cm2.Close() // idempotent: must not error, must not call exit cmd twice pointlessly
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Fatalf("expected control dir to be removed after Close, stat err = %v", statErr)
	}
	if exitCalls != 1 {
		t.Fatalf("expected exactly 1 ssh -O exit invocation, got %d", exitCalls)
	}
}

// TestStartRemoteLoadStream_ParsesAndDelivers substitutes fake ssh commands
// (plain shell scripts) for both the ControlMaster start/exit and the
// streaming child, verifying StartRemoteLoadStream's NDJSON parsing,
// delivery, and idempotent shutdown without any real SSH target.
func TestStartRemoteLoadStream_ParsesAndDelivers(t *testing.T) {
	origStart := sshControlMasterStartCmd
	origExit := sshControlMasterExitCmd
	origChild := sshLoadStreamChildCmd
	defer func() {
		sshControlMasterStartCmd = origStart
		sshControlMasterExitCmd = origExit
		sshLoadStreamChildCmd = origChild
	}()

	sshControlMasterStartCmd = func(ctlPath, host string) *exec.Cmd { return exec.Command("true") }
	exitCalls := 0
	sshControlMasterExitCmd = func(ctlPath, host string) *exec.Cmd {
		exitCalls++
		return exec.Command("true")
	}

	snap1 := LoadSnapshot{CPU: CPULoad{NumCPU: 4, Ok: true}}
	snap2 := LoadSnapshot{CPU: CPULoad{NumCPU: 8, Ok: true}}
	data1, _ := json.Marshal(snap1)
	data2, _ := json.Marshal(snap2)
	script := fakeScript(t, fmt.Sprintf(
		"echo '%s'\nsleep 0.05\necho 'not valid json'\necho '%s'\nsleep 1\n",
		string(data1), string(data2)))
	sshLoadStreamChildCmd = func(ctx context.Context, ctlPath, host string) *exec.Cmd {
		return exec.CommandContext(ctx, script)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, stop, err := StartRemoteLoadStream(ctx, "um760")
	if err != nil {
		t.Fatalf("StartRemoteLoadStream: %v", err)
	}
	defer stop()

	var got []LoadSnapshot
	timeout := time.After(3 * time.Second)
collectLoop:
	for len(got) < 2 {
		select {
		case s, ok := <-ch:
			if !ok {
				break collectLoop
			}
			got = append(got, s)
		case <-timeout:
			t.Fatal("timed out waiting for streamed samples")
		}
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 valid samples (malformed line skipped), got %d: %+v", len(got), got)
	}
	if got[0].CPU.NumCPU != 4 || got[1].CPU.NumCPU != 8 {
		t.Fatalf("unexpected sample contents: %+v", got)
	}

	stop()
	stop() // idempotent
	if exitCalls != 1 {
		t.Fatalf("expected exactly 1 ssh -O exit invocation across both stop() calls, got %d", exitCalls)
	}
}

func TestStartRemoteLoadStream_ControlMasterFailurePropagates(t *testing.T) {
	origStart := sshControlMasterStartCmd
	defer func() { sshControlMasterStartCmd = origStart }()
	sshControlMasterStartCmd = func(ctlPath, host string) *exec.Cmd { return exec.Command("false") }

	ch, stop, err := StartRemoteLoadStream(context.Background(), "um760")
	defer stop()
	if err == nil {
		t.Fatal("expected error when ControlMaster start fails")
	}
	if ch != nil {
		t.Fatal("expected nil channel on failure")
	}
}

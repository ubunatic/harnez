package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"ubunatic.com/harnez/internal/procs"
	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/internal/usage"
)

func signalTestDeps(store *subagent.FileSessionStore) agentDeps {
	return agentDeps{
		store:        func() (*subagent.FileSessionStore, error) { return store, nil },
		parent:       func() string { return "" },
		quota:        func(context.Context, string, bool) usage.TurnQuotaReading { return usage.TurnQuotaReading{} },
		availability: func(string, string) usage.ProviderQuotaAvailability { return usage.ProviderQuotaAvailability{} },
		preflight:    func(context.Context) error { return nil },
		find: func(_ *cobra.Command, s *subagent.FileSessionStore, name string) (*subagent.Session, error) {
			return resolveSession(s, name, "")
		},
	}
}

func signalTestCmd(timeout string) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().Duration("timeout", 0, "")
	if timeout != "" {
		_ = cmd.Flags().Set("timeout", timeout)
	}
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	return cmd
}

// TestSignalFixture is the harnez side of TestSignalStopsProviderChild: it
// runs one real start or resume turn through withAgentTimeout against a fake
// provider CLI on PATH and exits non-zero when the turn fails.
func TestSignalFixture(t *testing.T) {
	if os.Getenv("HARNEZ_SIGNAL_FIXTURE") != "1" {
		return
	}
	t.Setenv("HARNEZ_AGENT_ROLE", "")
	t.Setenv("HARNEZ_SESSION_ID", "")
	store, err := subagent.NewSessionStore(os.Getenv("HARNEZ_SIGNAL_STORE"))
	if err != nil {
		t.Fatal(err)
	}
	provider := os.Getenv("HARNEZ_SIGNAL_PROVIDER")
	agentDriver = func(subagent.Model, string) subagent.Driver {
		if provider == "claude" {
			return subagent.ClaudeDriver{}
		}
		return subagent.CodexDriver{}
	}
	cmd := signalTestCmd("")
	json := provider == "claude" // claude: batch path; codex: streaming startProcess
	err = withAgentTimeout(cmd, 0, func(ctx context.Context) error {
		if os.Getenv("HARNEZ_SIGNAL_VERB") == "resume" {
			return runResume(ctx, cmd, signalTestDeps(store), resumeRequest{Name: "worker", Prompt: "task", JSON: json, StreamMode: streamFull})
		}
		return runStart(ctx, cmd, signalTestDeps(store), startRequest{Name: "worker", Role: "advisor", Prompt: "task", ModelSpec: "codex:luna", Dir: ".", JSON: json, StreamMode: streamFull})
	})
	fmt.Fprintf(os.Stderr, "fixture result: %v\n", err)
	if err != nil {
		os.Exit(3)
	}
}

func TestSignalStopsProviderChild(t *testing.T) {
	skipShort(t, "signals a real harnez turn with a provider process tree")
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		t.Skip("Linux process identity fixture")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 needed for fake provider")
	}
	for _, tc := range []struct {
		provider, verb string
		sig            syscall.Signal
	}{
		{"claude", "start", syscall.SIGTERM},
		{"claude", "start", syscall.SIGINT},
		{"claude", "resume", syscall.SIGTERM},
		{"codex", "start", syscall.SIGTERM},
	} {
		t.Run(fmt.Sprintf("%s/%s/%s", tc.provider, tc.verb, signalName(tc.sig)), func(t *testing.T) {
			dir := t.TempDir()
			ready := processTestListener(t)
			// The provider starts a sleeping child in its group and reports the child's PID.
			script := "#!" + python + "\n" + `import os, signal, socket, subprocess
child = subprocess.Popen(['sleep', '30'])
host, port = os.environ['HARNEZ_SIGNAL_READY'].split(':')
with socket.create_connection((host, int(port)), timeout=5) as ready:
    ready.sendall((str(child.pid)+'\n').encode())
while True:
    signal.pause()
`
			if err := os.WriteFile(filepath.Join(dir, tc.provider), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			store, err := subagent.NewSessionStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			if tc.verb == "resume" {
				now := time.Now()
				if err := store.Save(&subagent.Session{ID: "worker-id", Name: "worker", Provider: "claude", Model: "haiku", Role: "advisor", WorkingDir: dir, ProviderSessionID: "p1", ContextTokens: 100, TokenTotalsKnown: true, HarnessType: "harnez", Status: "done", CreatedAt: now, LastActiveAt: now}); err != nil {
					t.Fatal(err)
				}
			}
			harnez := exec.Command(os.Args[0], "-test.run=^TestSignalFixture$")
			harnez.Dir = dir
			harnez.Env = append(os.Environ(), "HARNEZ_SIGNAL_FIXTURE=1", "HARNEZ_SIGNAL_STORE="+dir, "HARNEZ_SIGNAL_PROVIDER="+tc.provider, "HARNEZ_SIGNAL_VERB="+tc.verb, "HARNEZ_SIGNAL_READY="+ready.Addr().String(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			var log bytes.Buffer
			harnez.Stdout, harnez.Stderr = &log, &log
			if err := harnez.Start(); err != nil {
				t.Fatal(err)
			}
			child := 0
			var exitErr error
			exited := make(chan struct{})
			go func() { exitErr = harnez.Wait(); close(exited) }()
			defer func() {
				if child > 0 {
					_ = syscall.Kill(child, syscall.SIGKILL)
				}
				_ = harnez.Process.Kill()
				select {
				case <-exited:
				case <-time.After(5 * time.Second * raceFactor):
				}
				if t.Failed() {
					t.Logf("harnez fixture output: %s", log.String())
				}
			}()
			conn := acceptProcessTestEvent(t, ready)
			if _, err := fmt.Fscanln(conn, &child); err != nil || child <= 0 {
				t.Fatalf("provider child PID = %d, %v", child, err)
			}
			_ = conn.Close()
			signalled := time.Now()
			if err := harnez.Process.Signal(tc.sig); err != nil {
				t.Fatal(err)
			}
			select {
			case <-exited:
				if exitErr == nil {
					t.Fatal("harnez exited 0 after a signal")
				}
			case <-time.After(5 * time.Second * raceFactor):
				t.Fatal("harnez did not exit after the signal")
			}
			t.Logf("harnez exited %s after %s", time.Since(signalled), signalName(tc.sig))
			if !processGone(child, 5*time.Second*raceFactor) {
				t.Fatalf("provider child %d still running after %s to harnez", child, signalName(tc.sig))
			}
			sessions, err := store.List("", true)
			if err != nil || len(sessions) != 1 {
				t.Fatalf("sessions = %d, %v", len(sessions), err)
			}
			got := sessions[0]
			if got.Status != "stopped" || got.ProcessPID != 0 || got.ProviderPID != 0 || !strings.Contains(got.LastError, signalName(tc.sig)) {
				t.Fatalf("session after signal = status %q pid %d provider %d error %q; want stopped", got.Status, got.ProcessPID, got.ProviderPID, got.LastError)
			}
			if tc.verb == "resume" && got.ResumeFailures != 0 {
				t.Fatalf("signal counted as resume failure: %d", got.ResumeFailures)
			}
		})
	}
}

// processGone reports whether pid ends (or is reaped) within limit.
func processGone(pid int, limit time.Duration) bool {
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	deadline := time.After(limit)
	for {
		if procs.ProcessStarttime(pid) == 0 {
			return true
		}
		if state, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil && bytes.Contains(state, []byte(") Z ")) {
			return true
		}
		select {
		case <-tick.C:
		case <-deadline:
			return false
		}
	}
}

// blockingTurnDriver blocks each turn until its context ends.
type blockingTurnDriver struct{ recordingAgentDriver }

func (*blockingTurnDriver) Run(ctx context.Context, _ subagent.RunOptions) (*subagent.TurnResult, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestSignalContextKeepsDoneAndTimeout(t *testing.T) {
	old := agentDriver
	t.Cleanup(func() { agentDriver = old })
	t.Run("normal turn stays completed", func(t *testing.T) {
		store, err := subagent.NewSessionStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		agentDriver = func(subagent.Model, string) subagent.Driver { return &recordingAgentDriver{} }
		cmd := signalTestCmd("")
		err = withAgentTimeout(cmd, 0, func(ctx context.Context) error {
			return runStart(ctx, cmd, signalTestDeps(store), startRequest{Name: "w", Role: "advisor", Prompt: "task", ModelSpec: "codex:luna", Dir: ".", JSON: true, StreamMode: streamFull})
		})
		if err != nil {
			t.Fatal(err)
		}
		sessions, _ := store.List("", true)
		if len(sessions) != 1 || sessions[0].Status != "completed" {
			t.Fatalf("sessions = %+v; want one completed", sessions)
		}
	})
	t.Run("timeout is not a signal", func(t *testing.T) {
		store, err := subagent.NewSessionStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		agentDriver = func(subagent.Model, string) subagent.Driver { return &blockingTurnDriver{} }
		cmd := signalTestCmd("50ms")
		err = withAgentTimeout(cmd, 50*time.Millisecond, func(ctx context.Context) error {
			return runStart(ctx, cmd, signalTestDeps(store), startRequest{Name: "w", Role: "advisor", Prompt: "task", ModelSpec: "codex:luna", Dir: ".", JSON: true, StreamMode: streamFull})
		})
		if err == nil || !strings.Contains(err.Error(), "deadline") || strings.Contains(err.Error(), "interrupted by") {
			t.Fatalf("timeout error = %v; want a deadline, not a signal", err)
		}
		sessions, _ := store.List("", true)
		if len(sessions) != 1 || sessions[0].Status == "stopped" {
			t.Fatalf("sessions = %+v; a timeout must not be recorded as a signal stop", sessions)
		}
	})
}

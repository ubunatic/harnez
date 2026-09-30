//go:build linux

package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Run the actual watch loop under a controlling PTY, including a collector
// blocked until cancellation. Checking escape sequences alone misses raw-mode
// leaks, so the child also compares the complete before/after termios state.
func TestWatchPTYLifecycle(t *testing.T) {
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("util-linux script is required for PTY integration")
	}
	for _, tc := range []struct {
		name, mode string
		key        byte
	}{
		{"startup-q", "startup", 'q'},
		{"startup-Q", "startup", 'Q'},
		{"startup-ctrl-c", "startup", 3},
		{"startup-escape", "startup", 27},
		{"refresh-q", "refresh", 'q'},
		{"refresh-ctrl-c", "refresh", 3},
		{"context-cancel", "cancel", 0},
		{"sigterm", "sigterm", 0},
		{"sigint", "sigint", 0},
		{"output-error", "output-error", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			quoted := "'" + strings.ReplaceAll(exe, "'", "'\\''") + "'"
			cmd := exec.Command("script", "-qec", quoted+" -test.run=^TestWatchPTYHelper$", "/dev/null")
			cmd.Env = append(os.Environ(), "HARNEZ_WATCH_PTY_HELPER="+tc.mode)
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			in, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			out, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stderr = cmd.Stdout
			start := time.Now()
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }()
			chunks := make(chan string, 32)
			go func() {
				defer close(chunks)
				buf := make([]byte, 8192)
				for {
					n, err := out.Read(buf)
					if n > 0 {
						chunks <- string(buf[:n])
					}
					if err != nil {
						return
					}
				}
			}()
			var transcript strings.Builder
			var quitAt time.Time
			frameSeen, refreshSent := false, false
			deadline := time.NewTimer(8 * time.Second)
			defer deadline.Stop()
		loop:
			for {
				select {
				case chunk, ok := <-chunks:
					if !ok {
						break loop
					}
					transcript.WriteString(chunk)
					s := transcript.String()
					if !frameSeen && strings.Contains(s, "Collecting usage") {
						frameSeen = true
						if elapsed := time.Since(start); elapsed >= time.Second {
							t.Errorf("first dashboard took %s, want <1s", elapsed)
						}
					}
					if tc.mode == "refresh" && !refreshSent && strings.Contains(s, "PTY-ready") {
						_, _ = in.Write([]byte{'r'}) // remote -> local, starts blocked refresh
						refreshSent = true
					}
					if tc.key != 0 && quitAt.IsZero() && strings.Contains(s, "FETCH_BLOCKED") {
						quitAt = time.Now()
						_, _ = in.Write([]byte{tc.key})
					}
				case <-deadline.C:
					_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
					_ = cmd.Wait()
					t.Fatalf("watch did not exit: %s", transcript.String())
				}
			}
			if err := cmd.Wait(); err != nil {
				t.Fatalf("watch helper: %v\n%s", err, transcript.String())
			}
			if !quitAt.IsZero() && time.Since(quitAt) >= time.Second {
				t.Errorf("quit took %s, want <1s", time.Since(quitAt))
			}
			s := transcript.String()
			for _, marker := range []string{"\x1b[?1049h", "\x1b[0m\x1b[?25h\x1b[?1049l", "TERMINAL_RESTORED"} {
				if !strings.Contains(s, marker) {
					t.Errorf("missing %q in %s", marker, s)
				}
			}
			if tc.mode != "output-error" && !frameSeen {
				t.Error("no initial loading dashboard")
			}
			if tc.key != 0 && quitAt.IsZero() {
				t.Error("collector never blocked; quit path was not exercised")
			}
		})
	}
}

type failWatchFrameWriter struct{ failed bool }

func (w *failWatchFrameWriter) Write(p []byte) (int, error) {
	if !w.failed && bytes.Contains(p, []byte("Collecting usage")) {
		w.failed = true
		return 0, io.ErrClosedPipe
	}
	return os.Stdout.Write(p)
}

func TestWatchPTYHelper(t *testing.T) {
	mode := os.Getenv("HARNEZ_WATCH_PTY_HELPER")
	if mode == "" {
		return
	}
	before, err := exec.Command("stty", "-F", "/dev/tty", "-g").Output()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	opts := WatchOptions{Compact: true}
	opts.SharedUsageCollector = func(ctx context.Context) UsageSummary {
		fmt.Fprint(os.Stdout, "FETCH_BLOCKED")
		switch mode {
		case "cancel":
			cancel()
		case "sigterm":
			_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
		case "sigint":
			_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
		}
		<-ctx.Done()
		return UsageSummary{}
	}
	if mode == "refresh" {
		// The first remote fetch succeeds; [r] then starts a local refresh.
		// This verifies cancellation after startup uses the same async path.
		dir := t.TempDir()
		summary := UsageSummary{Timestamp: time.Now(), Agents: []AgentUsage{{AgentID: "codex", Name: "PTY-ready", Installed: true, Session: &QuotaWindow{Name: "Session", UsedPercent: 42}}}}
		data, err := json.Marshal(summary)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte("#!/bin/sh\nprintf '%s\\n' '"+string(data)+"'\n"), 0o700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		opts.Host = "fixture"
	}
	var out io.Writer = os.Stdout
	if mode == "output-error" {
		out = &failWatchFrameWriter{}
	}
	err = RunWatchWithOptions(ctx, t.TempDir(), nil, out, MinWatchInterval, t.TempDir(), opts)
	if mode == "output-error" {
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("output error = %v", err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	after, err := exec.Command("stty", "-F", "/dev/tty", "-g").Output()
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("terminal state changed: before=%s after=%s error=%v", before, after, err)
	}
	fmt.Fprint(os.Stdout, "TERMINAL_RESTORED")
}

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/procs"
	"ubunatic.com/harnez/internal/subagent"
)

// This subprocess is a plain Harnez-shaped wrapper, never an agent session.
func TestStopFakeProviderWrapper(t *testing.T) {
	if os.Getenv("HARNEZ_STOP_FIXTURE") != "1" {
		return
	}
	store, err := subagent.NewSessionStore(os.Getenv("HARNEZ_STOP_STORE"))
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "fixture", Name: "fixture", Provider: os.Getenv("HARNEZ_STOP_PROVIDER"), Model: "haiku", Status: "running", ProcessPID: os.Getpid(), ProcessStarttime: procs.ProcessStarttime(os.Getpid())}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	ctx := subagent.WithProcessObserver(context.Background(), func(pid int, start uint64) error {
		sess.ProviderPID, sess.ProviderStarttime = pid, start
		return store.Save(sess)
	})
	if sess.Provider == "claude" {
		_, _ = (subagent.ClaudeDriver{}).Run(ctx, subagent.RunOptions{})
	} else if os.Getenv("HARNEZ_STOP_STREAM") == "1" {
		_, _ = (subagent.CodexDriver{}).RunStream(ctx, subagent.RunOptions{}, func(subagent.Event) {})
	} else {
		_, _ = (subagent.CodexDriver{}).Run(ctx, subagent.RunOptions{})
	}
}

func TestStopProviderTreeAndWrapper(t *testing.T) {
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		t.Skip("Linux process identity fixture")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 needed for plain-process fixture")
	}
	for _, variant := range []struct{ provider, stream, ignore string }{
		{"codex", "1", "1"}, {"codex", "0", "0"}, {"claude", "0", "1"}, {"claude", "0", "0"},
	} {
		t.Run(variant.provider+"/stream="+variant.stream+"/ignore-term="+variant.ignore, func(t *testing.T) {
			t.Setenv("HARNEZ_AGENT_ROLE", "")
			t.Setenv("HARNEZ_SESSION_ID", "")
			dir := t.TempDir()
			pidFile := filepath.Join(dir, "pids")
			// Descendants include a new session: group-only killing is insufficient.
			script := "#!" + python + "\n" + `import os, sys, signal, subprocess, time
depth = int(sys.argv[1]) if len(sys.argv) == 2 and sys.argv[1].isdigit() else 2
if os.environ['HARNEZ_STOP_IGNORE_TERM'] == '1':
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
with open(os.environ['HARNEZ_STOP_PIDS'], 'a') as f:
    f.write(str(os.getpid())+'\n')
if depth:
    subprocess.Popen([sys.executable, __file__, str(depth-1)], start_new_session=(depth == 2))
while True:
    time.sleep(.02)
`
			if err := os.WriteFile(filepath.Join(dir, variant.provider), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			store, err := subagent.NewSessionStore(dir)
			if err != nil {
				t.Fatal(err)
			}
			wrapper := exec.Command(os.Args[0], "-test.run=^TestStopFakeProviderWrapper$")
			wrapper.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			wrapper.Env = append(os.Environ(), "HARNEZ_STOP_FIXTURE=1", "HARNEZ_STOP_STORE="+dir, "HARNEZ_STOP_PROVIDER="+variant.provider, "HARNEZ_STOP_STREAM="+variant.stream, "HARNEZ_STOP_IGNORE_TERM="+variant.ignore, "HARNEZ_STOP_PIDS="+pidFile, "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			var log bytes.Buffer
			wrapper.Stdout, wrapper.Stderr = &log, &log
			if err := wrapper.Start(); err != nil {
				t.Fatal(err)
			}
			var pids []int
			defer func() {
				for _, pid := range pids {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
				_ = wrapper.Process.Kill()
				_ = wrapper.Wait()
			}()
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				data, _ := os.ReadFile(pidFile)
				pids = nil
				for _, field := range strings.Fields(string(data)) {
					pid, _ := strconv.Atoi(field)
					pids = append(pids, pid)
				}
				if len(pids) == 3 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if len(pids) != 3 {
				t.Fatalf("provider tree not ready: %v", pids)
			}
			sess, err := store.Get("fixture")
			if err != nil || sess.ProviderPID == 0 || sess.ProviderStarttime == 0 {
				t.Fatalf("provider identity = %+v, %v", sess, err)
			}
			// Numeric reproduction of the old behavior: the driver's Stop does nothing.
			var driver subagent.Driver = subagent.CodexDriver{}
			if variant.provider == "claude" {
				driver = subagent.ClaudeDriver{}
			}
			if err := driver.Stop(context.Background(), "fixture"); err != nil {
				t.Fatal(err)
			}
			live, err := sessionWriters(sess)
			if err != nil || len(live) != 4 {
				t.Fatalf("no-op baseline: %d live, %v; want wrapper + 3 providers", len(live), err)
			}
			t.Logf("baseline: %d live processes after old Stop", len(live))
			// Losing the wrapper must not make wait declare the writer dead.
			orphan := *sess
			exited := exec.Command("true")
			if err := exited.Run(); err != nil {
				t.Fatal(err)
			}
			orphan.ProcessPID, orphan.ProcessStarttime = exited.Process.Pid, 0
			if err := store.Save(&orphan); err != nil {
				t.Fatal(err)
			}
			waiting, err := waitForAgent(context.Background(), store, "fixture", 20*time.Millisecond)
			if err != nil || waiting.Status != "running" || waiting.ProviderPID != sess.ProviderPID {
				t.Fatalf("wait lost live provider after wrapper exit: %+v, %v", waiting, err)
			}
			// A stopped registry status must never hide an active writer.
			sess.Status = "stopped"
			if err := store.Save(sess); err != nil {
				t.Fatal(err)
			}
			if _, err := runWithStore(t, &recordingAgentDriver{}, dir, "resume", "--name", "fixture", "prompt"); err == nil || !strings.Contains(err.Error(), "writer is still alive") {
				t.Fatalf("resume live writer: %v", err)
			}
			cmd := newAgentCmd()
			cmd.SetArgs([]string{"stop", "--name", "fixture", "--store-dir", dir})
			var out bytes.Buffer
			cmd.SetOut(&out)
			began := time.Now()
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if time.Since(began) > 4*time.Second {
				t.Fatal("shutdown exceeded bounded grace + kill confirmation")
			}
			if !strings.Contains(out.String(), "stopped: fixture, no process left, safe to resume/delete") || !strings.Contains(out.String(), "few seconds to report exit") {
				t.Fatalf("stop output = %q", out.String())
			}
			roots := []procs.TreeRoot{{PID: wrapper.Process.Pid}}
			for _, pid := range pids {
				roots = append(roots, procs.TreeRoot{PID: pid})
			}
			live, err = procs.TreeMembers(roots, make(map[int]procs.ProcInfo), nil)
			if err != nil || len(live) != 0 {
				t.Fatalf("reported success with live tree: %+v, %v", live, err)
			}
			stored, err := store.Get("fixture")
			if err != nil || stored.Status != "stopped" || stored.ProcessPID != 0 || stored.ProviderPID != 0 {
				t.Fatalf("stored stop state = %+v, %v", stored, err)
			}
			t.Logf("fixed: 0 live processes, confirmed in %s", time.Since(began))
		})
	}
}

func TestStopReportStillExitingPreservesWriter(t *testing.T) {
	store, err := subagent.NewSessionStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess := &subagent.Session{ID: "sid", Name: "worker", Status: "running", ProcessPID: 1234}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	for _, failure := range []error{nil, fmt.Errorf("permission denied")} {
		cmd := &cobra.Command{}
		var out bytes.Buffer
		cmd.SetOut(&out)
		err := reportSessionStop(cmd, store, sess, []procs.ProcInfo{{PID: 1234}}, failure)
		if err == nil || !strings.Contains(err.Error(), "stop requested: 1234 still exiting; run `harnez agent wait --name worker`") || out.Len() != 0 {
			t.Fatalf("false success: output %q, error %v", out.String(), err)
		}
		stored, _ := store.Get("sid")
		if stored.Status != "running" || stored.ProcessPID != 1234 {
			t.Fatalf("lost writer tracking: %+v", stored)
		}
	}
}

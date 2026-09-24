package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestProcessGroupStoppedWalksKnownDescendants(t *testing.T) {
	procRoot := t.TempDir()
	writeProcFixture := func(pid int, name, state string, parent, pgid int, taskChildren map[int]string) {
		t.Helper()
		procDir := filepath.Join(procRoot, strconv.Itoa(pid))
		if err := os.MkdirAll(filepath.Join(procDir, "task"), 0o700); err != nil {
			t.Fatal(err)
		}
		stat := fmt.Sprintf("%d (%s) %s %d %d 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n", pid, name, state, parent, pgid)
		if err := os.WriteFile(filepath.Join(procDir, "stat"), []byte(stat), 0o600); err != nil {
			t.Fatal(err)
		}
		for tid, children := range taskChildren {
			taskDir := filepath.Join(procDir, "task", strconv.Itoa(tid))
			if err := os.MkdirAll(taskDir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(taskDir, "children"), []byte(children), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	writeProcFixture(321, "outer shell", "S", 1, 321, map[int]string{321: "400"})
	writeProcFixture(400, "middle shell", "S", 321, 321, map[int]string{400: "401"})
	writeProcFixture(401, "stopped leaf", "T", 400, 321, map[int]string{401: ""})
	writeProcFixture(999, "unrelated stopped", "T", 1, 999, map[int]string{999: ""})
	stopped, err := processGroupStoppedAt(procRoot, 321)
	if err != nil || !stopped {
		t.Fatalf("processGroupStoppedAt() = (%t, %v), want stopped grandchild in pgid 321", stopped, err)
	}

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate stopped.go")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(testFile), "stopped.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(source), "LinuxProcReader{}.List") {
		t.Fatal("stopped-group monitor must not use the full-/proc lister per tick")
	}
}

func TestMonitorStoppedGroupContinuesOnceThenKills(t *testing.T) {
	done := make(chan struct{})
	var signals []syscall.Signal
	outcome := monitorStoppedGroup(done, 321, 5*time.Millisecond, time.Millisecond,
		func(int) (bool, error) { return true, nil },
		func(_ int, signal syscall.Signal) error {
			signals = append(signals, signal)
			return nil
		})
	if !outcome.Detected || !outcome.Killed || !reflect.DeepEqual(signals, []syscall.Signal{syscall.SIGCONT, syscall.SIGKILL}) {
		t.Fatalf("monitor outcome=%+v, signals=%v; want one CONT then KILL", outcome, signals)
	}
}

func TestMonitorStoppedGroupReturnsAfterSuccessfulContinue(t *testing.T) {
	done := make(chan struct{})
	var reads atomic.Int32
	var signals []syscall.Signal
	outcome := monitorStoppedGroup(done, 322, 5*time.Millisecond, time.Millisecond,
		func(int) (bool, error) { return reads.Add(1) <= 7, nil },
		func(_ int, signal syscall.Signal) error {
			signals = append(signals, signal)
			return nil
		})
	if !outcome.Detected || outcome.Killed || !reflect.DeepEqual(signals, []syscall.Signal{syscall.SIGCONT}) {
		t.Fatalf("monitor outcome=%+v, signals=%v; want continue only", outcome, signals)
	}
}

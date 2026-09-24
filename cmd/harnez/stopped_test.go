package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestProcessGroupStoppedReadsOnlyLeaderStat(t *testing.T) {
	procRoot := t.TempDir()
	pidDir := filepath.Join(procRoot, "321")
	if err := os.Mkdir(pidDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "stat"), []byte("321 (child) T 0 321 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stopped, err := processGroupStoppedAt(procRoot, 321)
	if err != nil || !stopped {
		t.Fatalf("processGroupStoppedAt() = (%t, %v), want (true, nil)", stopped, err)
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

package main

import (
	"reflect"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

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

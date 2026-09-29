package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHookSound_DefaultReturnsQuicklyWithSlowFakePlayer(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "detached.log")
	fakeScript := filepath.Join(tempDir, "fake-harnez")

	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" > %q\nsleep 2\n", logFile)
	if err := os.WriteFile(fakeScript, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake executable: %v", err)
	}

	old := hookSoundExecutable
	hookSoundExecutable = func() (string, error) { return fakeScript, nil }
	defer func() { hookSoundExecutable = old }()

	start := time.Now()
	err := runHookSound(context.Background(), hookSoundOptions{})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("runHookSound default mode failed: %v", err)
	}

	if elapsed > 500*time.Millisecond {
		t.Errorf("runHookSound took %v, want < 500ms (fast return with detached slow player)", elapsed)
	}

	// Verify detached process executed in background and logged its arguments
	var logged []byte
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		logged, err = os.ReadFile(logFile)
		if err == nil && len(logged) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err != nil || len(logged) == 0 {
		t.Fatalf("detached process did not execute or log: %v", err)
	}

	gotArgs := strings.TrimSpace(string(logged))
	wantArgs := "hook sound --sync"
	if gotArgs != wantArgs {
		t.Errorf("detached process args = %q, want %q", gotArgs, wantArgs)
	}
}

func TestHookSound_CobraCommandDefaultMode(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "cobra_detached.log")
	fakeScript := filepath.Join(tempDir, "fake-harnez-cmd")

	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" > %q\nsleep 2\n", logFile)
	if err := os.WriteFile(fakeScript, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake executable: %v", err)
	}

	old := hookSoundExecutable
	hookSoundExecutable = func() (string, error) { return fakeScript, nil }
	defer func() { hookSoundExecutable = old }()

	cmd := newHookSoundCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	start := time.Now()
	err := cmd.Execute()
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("cmd.Execute() failed: %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("cmd.Execute() took %v, want < 500ms", elapsed)
	}
}

func TestHookSound_SyncModeCallsPlay(t *testing.T) {
	called := false
	playErr := errors.New("simulated play error")

	err := runHookSound(context.Background(), hookSoundOptions{
		Sync: true,
		Play: func(ctx context.Context) error {
			called = true
			return playErr
		},
	})

	if !called {
		t.Errorf("Play was not called in sync mode")
	}
	if !errors.Is(err, playErr) {
		t.Errorf("err = %v, want %v", err, playErr)
	}
}

func TestHookSound_ExecutableLookupFailure(t *testing.T) {
	lookupErr := errors.New("lookup failed")
	err := runHookSound(context.Background(), hookSoundOptions{
		Executable: func() (string, error) {
			return "", lookupErr
		},
	})
	if !errors.Is(err, lookupErr) {
		t.Errorf("err = %v, want %v", err, lookupErr)
	}
}

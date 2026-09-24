package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ubunatic.com/harnez/internal/quota1"
)

func TestCleanQuota1DryRunAndReleaseRequireDeadGroup(t *testing.T) {
	dir := t.TempDir()
	stateFile, _, err := quota1.ResolveStateFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	finished := time.Now().UTC()
	state := "{\"started\":\"" + finished.Add(-time.Minute).Format(time.RFC3339Nano) + "\",\"pgid\":2147483647,\"pid_starttime\":123,\"finished\":\"" + finished.Format(time.RFC3339Nano) + "\",\"exit\":null}"
	if err := os.MkdirAll(filepath.Dir(stateFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stateFile, []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}

	dryRun, err := cleanQuota1(dir, false)
	if err != nil || dryRun.Status != "would-release" {
		t.Fatalf("dry-run = %+v, %v; want would-release", dryRun, err)
	}
	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("dry-run removed quota state: %v", err)
	}

	released, err := cleanQuota1(dir, true)
	if err != nil || released.Status != "released" {
		t.Fatalf("release = %+v, %v; want released", released, err)
	}
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Fatalf("quota state remains after release, stat err = %v", err)
	}
}

func TestCleanQuota1KeepsNormalExit(t *testing.T) {
	dir := t.TempDir()
	stateFile, _, err := quota1.ResolveStateFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	state := "{\"started\":\"2026-09-24T12:00:00Z\",\"pgid\":2147483647,\"pid_starttime\":123,\"finished\":\"2026-09-24T12:01:00Z\",\"exit\":137}"
	if err := os.MkdirAll(filepath.Dir(stateFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stateFile, []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := cleanQuota1(dir, true)
	if err != nil || result.Status != "kept" || !strings.Contains(result.Reason, "modify source") {
		t.Fatalf("normal exit cleanup = %+v, %v; want keep with source guidance", result, err)
	}
	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("normal-exit quota state was removed: %v", err)
	}
}

func TestCleanQuota1KeepsLiveIncompleteGroup(t *testing.T) {
	dir := t.TempDir()
	stateFile, _, err := quota1.ResolveStateFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	state := `{"started":"2026-09-24T12:00:00Z","pgid":4321,"pid_starttime":123,"finished":"2026-09-24T12:01:00Z","exit":null}`
	if err := os.MkdirAll(filepath.Dir(stateFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stateFile, []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := cleanQuota1WithGroupCheck(dir, true, func(pgid int) (bool, error) {
		if pgid != 4321 {
			t.Fatalf("checked pgid = %d, want 4321", pgid)
		}
		return false, nil
	})
	if err != nil || result.Status != "kept" || !strings.Contains(result.Reason, "still alive") {
		t.Fatalf("live incomplete group = %+v, %v; want kept", result, err)
	}
	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("live incomplete quota state was removed: %v", err)
	}
}

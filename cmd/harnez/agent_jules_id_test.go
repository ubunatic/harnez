package main

import (
	"context"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
)

const julesIDTestSession = "5832791262383075829"

// julesIDDriver stands in for the Jules API: it counts reads and records each
// prompt that resume sends.
type julesIDDriver struct {
	recordingAgentDriver
	reads   int
	resumes []string
}

func (d *julesIDDriver) InspectJulesSession(_ context.Context, id string) (subagent.JulesSessionStatus, error) {
	d.reads++
	return subagent.JulesSessionStatus{ID: id, State: "AWAITING_USER_FEEDBACK"}, nil
}

func (d *julesIDDriver) Resume(_ context.Context, id, prompt string, _ subagent.Model) (*subagent.TurnResult, error) {
	d.resumes = append(d.resumes, prompt)
	return &subagent.TurnResult{SessionID: id, Response: "waiting"}, nil
}

func julesRecordCount(t *testing.T, dir string) int {
	t.Helper()
	store, err := subagent.NewSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := store.List("", true)
	if err != nil {
		t.Fatal(err)
	}
	return len(sessions)
}

func TestJulesIDRefusesNonJulesModelBeforeAnyRead(t *testing.T) {
	dir := t.TempDir()
	drv := &julesIDDriver{}
	if _, err := runWithStore(t, drv, dir, "resume", "--model", "claude:sonnet:low", "--id", julesIDTestSession, "wait"); err == nil || !strings.Contains(err.Error(), "not jules") {
		t.Errorf("--model claude with --id: err = %v", err)
	}
	if _, err := runWithStore(t, drv, dir, "status", "--model", "claude:sonnet:low", "--id", julesIDTestSession); err == nil || !strings.Contains(err.Error(), "not jules") {
		t.Errorf("status --model claude --id: err = %v", err)
	}
	if _, err := runWithStore(t, drv, dir, "resume", "--id", julesIDTestSession, "wait"); err == nil || !strings.Contains(err.Error(), "--model jules") {
		t.Errorf("--id without --model: err = %v", err)
	}
	if drv.reads != 0 || len(drv.resumes) != 0 || julesRecordCount(t, dir) != 0 {
		t.Errorf("refused commands reached the driver: reads %d resumes %v records %d", drv.reads, drv.resumes, julesRecordCount(t, dir))
	}
}

func TestJulesIDAdoptsOnceThenResumesOnTheRecord(t *testing.T) {
	dir := t.TempDir()
	drv := &julesIDDriver{}
	if _, err := runWithStore(t, drv, dir, "resume", "--model", "jules", "--id", julesIDTestSession, "wait"); err != nil {
		t.Fatalf("first adopt and wait: %v", err)
	}
	if n := julesRecordCount(t, dir); n != 1 {
		t.Fatalf("records after first adopt = %d, want 1", n)
	}
	if _, err := runWithStore(t, drv, dir, "resume", "--model", "jules", "--id", julesIDTestSession, "wait"); err != nil {
		t.Fatalf("second adopt and wait: %v", err)
	}
	if n := julesRecordCount(t, dir); n != 1 {
		t.Errorf("records after second adopt = %d, want 1", n)
	}
	if drv.reads != 1 {
		t.Errorf("reads = %d, want 1: the repeat adopt reuses the record", drv.reads)
	}
	if len(drv.resumes) != 2 || drv.resumes[0] != "wait" || drv.resumes[1] != "wait" {
		t.Errorf("resume prompts = %q, want two waits", drv.resumes)
	}
}

func TestStatusJulesIDReadsAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	drv := &julesIDDriver{}
	out, err := runWithStore(t, drv, dir, "status", "--model", "jules", "--id", julesIDTestSession)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "AWAITING_USER_FEEDBACK") {
		t.Errorf("status output lacks the state:\n%s", out)
	}
	if drv.reads != 1 || len(drv.resumes) != 0 || julesRecordCount(t, dir) != 0 {
		t.Errorf("status --id: reads %d resumes %v records %d, want one read and no write", drv.reads, drv.resumes, julesRecordCount(t, dir))
	}
}

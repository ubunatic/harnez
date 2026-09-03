package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/feedback"
)

// TestFeedbackPromoteCrossProject reproduces issue 203: an entry logged
// while working in one project (e.g. smarthome, observing a harnez bug)
// must still be promotable straight into a *different* project's issues/
// (e.g. harnez itself) via `harnez feedback promote <id> -d <other-dir>`,
// without manually reading the raw JSONL log.
func TestFeedbackPromoteCrossProject(t *testing.T) {
	feedbackHome := t.TempDir()
	t.Setenv("HOME", feedbackHome)

	originProject := filepath.Join(t.TempDir(), "smarthome")
	targetProject := filepath.Join(t.TempDir(), "harnez")
	if err := os.MkdirAll(targetProject, 0o755); err != nil {
		t.Fatalf("mkdir target project: %v", err)
	}

	// Log the entry as if an agent working in originProject called
	// `harnez feedback issue` while describing a bug in targetProject.
	var issueOut bytes.Buffer
	if err := runFeedbackIssue(&issueOut, "harnez apply drops a section on rerun", "bug", originProject, false); err != nil {
		t.Fatalf("runFeedbackIssue: %v", err)
	}
	id := strings.TrimSpace(strings.TrimPrefix(issueOut.String(), "logged feedback entry "))
	if id == "" {
		t.Fatalf("expected an id from runFeedbackIssue output, got %q", issueOut.String())
	}

	// Promoting scoped to the origin project's own -d target works (sanity
	// check that we didn't break the common case)... but the point of this
	// test is promoting into the *other* project without ever touching
	// originProject again.
	var promoteOut bytes.Buffer
	if err := runFeedbackPromote(&promoteOut, targetProject, id); err != nil {
		t.Fatalf("runFeedbackPromote into a different project than the entry was logged from: %v", err)
	}

	out := promoteOut.String()
	if !strings.Contains(out, "filed ") {
		t.Fatalf("expected promote output to report a filed ticket, got %q", out)
	}

	// The ticket must land under targetProject/issues/, not originProject.
	entries, err := os.ReadDir(filepath.Join(targetProject, "issues"))
	if err != nil {
		t.Fatalf("read target issues dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one ticket file in target issues dir, got %d: %+v", len(entries), entries)
	}
	name := entries[0].Name()
	if !strings.HasPrefix(name, "001-") {
		t.Errorf("expected ticket numbered 001, got %q", name)
	}
	if !strings.Contains(name, "harnez-apply-drops-a-section-on-rerun") {
		t.Errorf("expected ticket filename to slugify the description, got %q", name)
	}

	content, err := os.ReadFile(filepath.Join(targetProject, "issues", name))
	if err != nil {
		t.Fatalf("read ticket file: %v", err)
	}
	if !strings.Contains(string(content), "harnez apply drops a section on rerun") {
		t.Errorf("expected ticket body to contain the original description, got:\n%s", content)
	}

	// The origin project's log must now show the entry as promoted too
	// (the fallback path updates the log it actually found the entry in,
	// not one freshly created for targetProject).
	originEntries, err := feedback.Load(feedback.DefaultDir(), originProject)
	if err != nil {
		t.Fatalf("Load origin log: %v", err)
	}
	if len(originEntries) != 1 || originEntries[0].Status != feedback.StatusPromoted {
		t.Fatalf("expected origin log to show the entry as promoted, got %+v", originEntries)
	}

	// No stray log file should have been created for targetProject.
	targetLogPath, err := feedback.Path(feedback.DefaultDir(), targetProject)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if _, err := os.Stat(targetLogPath); !os.IsNotExist(err) {
		t.Errorf("expected no feedback log to be created for the target project, but found one at %s", targetLogPath)
	}
}

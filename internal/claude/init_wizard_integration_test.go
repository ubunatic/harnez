package claude

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInitWizardPreservesRecordedDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "issues", "README.md"), []byte("<!-- managed by harnez index -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	current := detectRepoConfig(dir, nil)
	current.Issues.Tracker = "none"
	current.Issues.MergeDriver = false
	if err := writeRepoConfig(dir, current); err != nil {
		t.Fatal(err)
	}
	if err := RunInitWithVariant(dir, nil, InitOptions{Force: true, WizardInput: strings.NewReader(""), WizardOutput: io.Discard}); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadRepoConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Issues.Tracker != "none" || loaded.Issues.MergeDriver {
		t.Fatalf("wizard replaced recorded defaults: %+v", loaded)
	}
}

func TestRunInitAssumeYesSkipsWizardReader(t *testing.T) {
	dir := t.TempDir()
	if err := RunInitWithVariant(dir, nil, InitOptions{Force: true, AssumeYes: true, WizardInput: wizardFailReader{}, WizardOutput: io.Discard}); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRepoConfig(dir); err != nil {
		t.Fatal(err)
	}
}

func TestRunInitWizardTrackerOptOutSurvivesAssumeYes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "issues", "README.md"), []byte("<!-- managed by harnez index -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RunInitWithVariant(dir, nil, InitOptions{Force: true, WizardInput: strings.NewReader("no\n\n\n\n\n\n"), WizardOutput: io.Discard}); err != nil {
		t.Fatal(err)
	}
	if err := RunInitWithVariant(dir, nil, InitOptions{Force: true, AssumeYes: true}); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadRepoConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Issues.Tracker != "none" {
		t.Fatalf("noninteractive init re-enabled tracker: %+v", loaded.Issues)
	}
}

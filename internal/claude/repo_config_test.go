package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoConfigDetectionAndValidation(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "issues"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "issues", "README.md"), []byte("# Local tracker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "docs", "studies"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte("all:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := detectRepoConfig(dir, nil)
	if got.Issues.Tracker != "none" || got.Issues.MergeDriver || !got.Docs.Studies || !got.Make.ContainerTest {
		t.Fatalf("unexpected detection: %+v", got)
	}
	if err := writeRepoConfig(dir, got); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadRepoConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Issues.Tracker != "none" || !loaded.Docs.Studies {
		t.Fatalf("loaded config differs: %+v", loaded)
	}
	if err := os.WriteFile(repoConfigPath(dir), []byte("version: 1\nunknown: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRepoConfig(dir); err == nil {
		t.Fatal("unknown key accepted")
	}
}

func TestRepoConfigRecognizesHarnezTracker(t *testing.T) {
	dir := t.TempDir()
	issues := filepath.Join(dir, "issues")
	if err := os.MkdirAll(issues, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(issues, "README.md"), []byte("<!-- managed by harnez index -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := detectRepoConfig(dir, nil)
	if got.Issues.Tracker != "harnez" || !got.Issues.MergeDriver {
		t.Fatalf("tracker not recognized: %+v", got.Issues)
	}
}

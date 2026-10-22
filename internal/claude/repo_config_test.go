package claude

import (
	"os"
	"path/filepath"
	"strings"
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
	if got.Issues.Tracker != "none" || got.Issues.MergeDriver || !got.Docs.Studies || got.Make.ContainerTest {
		t.Fatalf("unexpected detection: %+v", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte("container-test:\n\ntest-q1:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = detectRepoConfig(dir, nil)
	if !got.Make.ContainerTest || !got.Rules.Quota {
		t.Fatalf("existing opt-in targets not detected: %+v", got)
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

func TestRunInitGatesContainerTargetAndWritesNotices(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte("all:\n\t@true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RunInitWithVariant(dir, LoadTestConfig(t), InitOptions{AssumeYes: true}); err != nil {
		t.Fatal(err)
	}
	makefile, err := os.ReadFile(filepath.Join(dir, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(makefile), "container-test:") {
		t.Fatalf("container-test added without opt-in:\n%s", makefile)
	}
	repo, err := LoadRepoConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if repo.Issues.Tracker != "none" || repo.Make.ContainerTest {
		t.Fatalf("unexpected repo config: %+v", repo)
	}
	index, err := os.ReadFile(filepath.Join(dir, ".harnez", "rules", "Index.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, notice := range []string{"This repo does not use file-based issues; skip those steps and use the repo's own tracker.", "This repo does not use indexed studies; skip those steps."} {
		if !strings.Contains(string(index), notice) {
			t.Errorf("Index.md missing notice %q:\n%s", notice, index)
		}
	}
}

func TestRunInitPreservesExistingRepoConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte("all:\n\t@true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := &RepoConfig{Version: 1}
	config.Issues.Tracker = "none"
	config.Docs.Copies = []string{"bash"}
	if err := writeRepoConfig(dir, config); err != nil {
		t.Fatal(err)
	}
	path := repoConfigPath(dir)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := RunInitWithVariant(dir, LoadTestConfig(t), InitOptions{AssumeYes: true}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("repo.yaml rewritten without override:\nbefore:\n%s\nafter:\n%s", before, after)
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

func TestWriteRulesAcceptsNilRepoConfig(t *testing.T) {
	dir := t.TempDir()
	changes, err := writeRules(dir, RulesConfig{Index: "# Harnez Rules\n"}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if changes != 1 {
		t.Fatalf("writeRules changes=%d, want 1", changes)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".harnez", "rules", "Index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "does not use") {
		t.Fatalf("nil repo config emitted notice: %s", data)
	}
}

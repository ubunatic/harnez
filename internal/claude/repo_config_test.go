package claude

import (
	"io"
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
	if got.Issues.Tracker != "none" || got.Issues.MergeDriver || !got.Docs.Studies || !got.Make.Scaffold || got.Make.ContainerTest {
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
	if loaded.Issues.Tracker != "none" || !loaded.Docs.Studies || !loaded.Make.Scaffold {
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

func TestRunInitMakeScaffoldOptOutAndLegacyDefault(t *testing.T) {
	tests := []struct {
		name           string
		scaffoldConfig string
		wantScaffold   bool
	}{
		{name: "opt out", scaffoldConfig: "  scaffold: false\n", wantScaffold: false},
		{name: "missing key defaults on", wantScaffold: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			initial := "all:\n\t@echo original\n"
			if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(initial), 0o644); err != nil {
				t.Fatal(err)
			}
			repoYAML := "version: 1\nissues:\n  tracker: none\n  merge_driver: false\ndocs:\n  studies: false\n  copies: [make]\nrules:\n  quota: false\nmake:\n" + tc.scaffoldConfig + "  container_test: false\n"
			writeRepoYAML(t, dir, repoYAML)
			loaded, err := LoadRepoConfig(dir)
			if err != nil {
				t.Fatal(err)
			}
			if loaded.Make.Scaffold != tc.wantScaffold {
				t.Fatalf("make.scaffold=%t, want %t", loaded.Make.Scaffold, tc.wantScaffold)
			}
			if err := RunInitWithVariant(dir, LoadTestConfig(t), InitOptions{AssumeYes: true}); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(dir, "Makefile"))
			if err != nil {
				t.Fatal(err)
			}
			if !tc.wantScaffold {
				if string(got) != initial {
					t.Fatalf("Makefile changed with scaffold disabled:\n%s", got)
				}
			} else if !strings.Contains(string(got), ".PHONY:") ||
				!strings.Contains(string(got), "_prim :=") ||
				!strings.Contains(string(got), "_rst  :=") ||
				!strings.Contains(string(got), "help:") {
				t.Fatalf("default Makefile scaffold missing:\n%s", got)
			}
		})
	}
}

func TestRunInitDocsCopiesIsAuthoritativeAndHintsDetectedDocs(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		wantHint   string
		missingDoc string
	}{
		{
			name:       "Make doc omitted from copies",
			files:      map[string]string{"Makefile": "all:\n\t@true\n"},
			wantHint:   "detected make docs; add with harnez init --docs make",
			missingDoc: "Make.md",
		},
		{
			name:       "new language detected",
			files:      map[string]string{"Cargo.toml": "[package]\nname = \"sample\"\n"},
			wantHint:   "detected rust docs; add with harnez init --docs rust",
			missingDoc: "Rust.md",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			writeRepoYAML(t, dir, "version: 1\nissues:\n  tracker: none\n  merge_driver: false\ndocs:\n  studies: false\n  copies: []\nrules:\n  quota: false\nmake:\n  container_test: false\n")
			out, err := captureInitStdout(t, func() error {
				return RunInitWithVariant(dir, LoadTestConfig(t), InitOptions{AssumeYes: true})
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, tc.wantHint) {
				t.Fatalf("init output missing hint %q:\n%s", tc.wantHint, out)
			}
			if _, err := os.Stat(filepath.Join(dir, "docs", tc.missingDoc)); !os.IsNotExist(err) {
				t.Fatalf("unlisted doc %s was copied (stat err=%v)", tc.missingDoc, err)
			}
		})
	}
}

func writeRepoYAML(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(repoConfigPath(dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repoConfigPath(dir), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func captureInitStdout(t *testing.T, run func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	runErr := run()
	_ = w.Close()
	os.Stdout = oldStdout
	output, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	return string(output), runErr
}

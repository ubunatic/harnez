package release

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInferBumpFromTagList(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		wantBump string
	}{
		{
			name:     "empty history defaults to patch",
			tags:     nil,
			wantBump: "patch",
		},
		{
			name:     "single tag defaults to patch",
			tags:     []string{"v0.1.0"},
			wantBump: "patch",
		},
		{
			name:     "minor sequence",
			tags:     []string{"v0.1.0", "v0.2.0", "v0.3.0"},
			wantBump: "minor",
		},
		{
			name:     "patch sequence",
			tags:     []string{"v0.1.0", "v0.1.1", "v0.1.2"},
			wantBump: "patch",
		},
		{
			name:     "minor then patch shift tie-breaks to newest (patch)",
			tags:     []string{"v0.1.0", "v0.2.0", "v0.2.1"},
			wantBump: "patch",
		},
		{
			name:     "patch then minor shift tie-breaks to newest (minor)",
			tags:     []string{"v0.1.0", "v0.1.1", "v0.2.0"},
			wantBump: "minor",
		},
		{
			name:     "non-semver and prerelease noise ignored",
			tags:     []string{"release-notes", "v0.1.0", "v0.2.0-rc.1", "v0.2.0", "v0.3.0-alpha.1", "v0.3.0", "not-a-tag"},
			wantBump: "minor",
		},
		{
			name:     "only prerelease tags defaults to patch",
			tags:     []string{"v0.1.0-alpha.1", "v0.2.0-beta.1"},
			wantBump: "patch",
		},
		{
			name:     "major jump never inferred as major",
			tags:     []string{"v0.9.0", "v1.0.0"},
			wantBump: "patch",
		},
		{
			name:     "only latest 4 stable tags inspected",
			tags:     []string{"v0.1.0", "v0.1.1", "v0.1.2", "v0.1.3", "v0.2.0", "v0.3.0", "v0.4.0"},
			wantBump: "minor",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := inferBumpFromTagList(tc.tags)
			if got != tc.wantBump {
				t.Errorf("inferBumpFromTagList(%v) = %q (%s), want %q", tc.tags, got, reason, tc.wantBump)
			}
			if reason == "" {
				t.Errorf("expected non-empty reason")
			}
		})
	}
}

func TestRunInferredBumpAndExplicitOverride(t *testing.T) {
	dir := setupGitRepoWithVersion(t, "0.1.0", false)
	if err := os.WriteFile(filepath.Join(dir, "version.yaml"), []byte("version: 0.2.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, dir, "add", "version.yaml")
	runGitCmd(t, dir, "commit", "-m", "feat: step 2")
	runGitCmd(t, dir, "tag", "-a", "v0.2.0", "-m", "v0.2.0")
	if err := os.WriteFile(filepath.Join(dir, "work.txt"), []byte("change 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, dir, "add", "work.txt")
	runGitCmd(t, dir, "commit", "-m", "feat: step 3")

	// Empty Bump infers minor from v0.1.0 -> v0.2.0 and logs inference.
	var out bytes.Buffer
	err := Run(Options{
		Dir:         dir,
		Bump:        "",
		DryRun:      true,
		SkipBuild:   true,
		SkipSign:    true,
		SkipPublish: true,
		SkipPush:    true,
		Out:         &out,
	})
	if err != nil {
		t.Fatalf("dry-run inferred release: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "[version] Inferred bump: minor") {
		t.Errorf("expected inferred minor bump log, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "0.2.0 -> 0.3.0") {
		t.Errorf("expected bump 0.2.0 -> 0.3.0, got:\n%s", out.String())
	}

	// Explicit Bump overrides inference silently.
	out.Reset()
	err = Run(Options{
		Dir:         dir,
		Bump:        "patch",
		DryRun:      true,
		SkipBuild:   true,
		SkipSign:    true,
		SkipPublish: true,
		SkipPush:    true,
		Out:         &out,
	})
	if err != nil {
		t.Fatalf("dry-run explicit release: %v\n%s", err, out.String())
	}
	if strings.Contains(out.String(), "Inferred bump:") {
		t.Errorf("explicit bump should not log inference, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "0.2.0 -> 0.2.1") {
		t.Errorf("expected bump 0.2.0 -> 0.2.1, got:\n%s", out.String())
	}
}

func TestRunSkipBuildVsMissingBuildConfig(t *testing.T) {
	dir := setupGitRepoWithVersion(t, "0.1.0", false)

	var skippedOut bytes.Buffer
	err := Run(Options{
		Dir:         dir,
		Bump:        "patch",
		DryRun:      true,
		Force:       true,
		SkipBuild:   true,
		SkipSign:    true,
		SkipPublish: true,
		SkipPush:    true,
		Out:         &skippedOut,
	})
	if err != nil {
		t.Fatalf("skip-build run: %v\n%s", err, skippedOut.String())
	}
	if !strings.Contains(skippedOut.String(), "[build] Build skipped (--skip-build)") {
		t.Errorf("expected explicit skip-build message, got:\n%s", skippedOut.String())
	}
	if strings.Contains(skippedOut.String(), "No build configuration detected") {
		t.Errorf("skip-build should not report missing build config, got:\n%s", skippedOut.String())
	}

	stubs := t.TempDir()
	if err := os.WriteFile(filepath.Join(stubs, "minisign"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	dummyKey := filepath.Join(stubs, "dummy.key")
	if err := os.WriteFile(dummyKey, []byte("untrusted comment: dummy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", stubs+string(os.PathListSeparator)+os.Getenv("PATH"))

	var missingOut bytes.Buffer
	err = Run(Options{
		Dir:         dir,
		Bump:        "patch",
		DryRun:      true,
		Force:       true,
		SignKey:     dummyKey,
		SkipBuild:   false,
		SkipSign:    false,
		SkipPublish: true,
		SkipPush:    true,
		Out:         &missingOut,
	})
	if err != nil {
		t.Fatalf("missing-build run: %v\n%s", err, missingOut.String())
	}
	wantBuild := "[build] No build configuration detected (no build_cmd in version.yaml, no .goreleaser.yaml, no make dist/build target); proceeding without artifact compilation."
	if !strings.Contains(missingOut.String(), wantBuild) {
		t.Errorf("expected %q, got:\n%s", wantBuild, missingOut.String())
	}
	if !strings.Contains(missingOut.String(), "No build artifacts or dist/SHA256SUMS found in dist/") {
		t.Errorf("expected clear signing notice when dist/ is empty, got:\n%s", missingOut.String())
	}
}

func TestRunMissingAndMalformedVersionSpec(t *testing.T) {
	// Missing version.yaml emits tag-only fallback warning.
	dir := t.TempDir()
	runGitCmd(t, dir, "init")
	runGitCmd(t, dir, "config", "user.email", "test@example.com")
	runGitCmd(t, dir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, dir, "add", "README.md")
	runGitCmd(t, dir, "commit", "-m", "init")

	var out bytes.Buffer
	err := Run(Options{
		Dir:         dir,
		Bump:        "patch",
		DryRun:      true,
		Force:       true,
		SkipBuild:   true,
		SkipSign:    true,
		SkipPublish: true,
		SkipPush:    true,
		Out:         &out,
	})
	if err != nil {
		t.Fatalf("missing version.yaml dry-run: %v\n%s", err, out.String())
	}
	wantWarn := "[warning] No version.yaml found in " + dir + "; release is running in tag-only fallback mode without a declared release spec."
	if !strings.Contains(out.String(), wantWarn) {
		t.Errorf("expected missing version.yaml warning %q, got:\n%s", wantWarn, out.String())
	}

	// Malformed version.yaml fails cleanly without overwriting the file.
	brokenContent := "version: [unterminated\n"
	specPath := filepath.Join(dir, "version.yaml")
	if err := os.WriteFile(specPath, []byte(brokenContent), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, dir, "add", "version.yaml")
	runGitCmd(t, dir, "commit", "-m", "add broken version.yaml")

	out.Reset()
	err = Run(Options{
		Dir:         dir,
		Bump:        "patch",
		DryRun:      false,
		Force:       true,
		SkipBuild:   true,
		SkipSign:    true,
		SkipPublish: true,
		SkipPush:    true,
		Out:         &out,
	})
	if err == nil {
		t.Fatalf("expected error for malformed version.yaml, got nil\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "load version spec") {
		t.Errorf("expected load version spec error, got: %v", err)
	}
	gotContent, readErr := os.ReadFile(specPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(gotContent) != brokenContent {
		t.Errorf("malformed version.yaml was modified: got %q, want %q", string(gotContent), brokenContent)
	}
}

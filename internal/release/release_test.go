package release

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSemverAndBump(t *testing.T) {
	tests := []struct {
		input    string
		bump     string
		expected string
	}{
		{"0.1.0", "patch", "0.1.1"},
		{"0.1.0", "minor", "0.2.0"},
		{"0.1.0", "major", "1.0.0"},
		{"v1.2.3", "patch", "1.2.4"},
		{"1.2.3", "2.0.0", "2.0.0"},
		{"1.0.0-beta.1", "patch", "1.0.1"},
	}

	for _, tc := range tests {
		bumped, err := BumpVersion(tc.input, tc.bump)
		if err != nil {
			t.Fatalf("BumpVersion(%q, %q) error: %v", tc.input, tc.bump, err)
		}
		if bumped.String() != tc.expected {
			t.Errorf("BumpVersion(%q, %q) = %q, expected %q", tc.input, tc.bump, bumped.String(), tc.expected)
		}
	}
}

func TestVersionSpecLoadSave(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "version.yaml")

	spec := &VersionSpec{
		Version: "1.2.3",
		Files:   []string{"version.go", "build.zig.zon"},
	}

	if err := SaveVersionSpec(specPath, spec); err != nil {
		t.Fatalf("SaveVersionSpec error: %v", err)
	}

	loaded, loadedPath, err := LoadVersionSpec(tmpDir)
	if err != nil {
		t.Fatalf("LoadVersionSpec error: %v", err)
	}

	if loadedPath != specPath {
		t.Errorf("loadedPath = %q, want %q", loadedPath, specPath)
	}
	if loaded.Version != "1.2.3" {
		t.Errorf("loaded.Version = %q, want 1.2.3", loaded.Version)
	}
	if len(loaded.Files) != 2 {
		t.Errorf("loaded.Files len = %d, want 2", len(loaded.Files))
	}
}

func TestSyncLanguageFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Go version.go
	goFile := filepath.Join(tmpDir, "version.go")
	if err := os.WriteFile(goFile, []byte("package foo\n\nvar Version = \"0.1.0\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Python __version__.py
	pyFile := filepath.Join(tmpDir, "__version__.py")
	if err := os.WriteFile(pyFile, []byte("__version__ = \"0.1.0\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Zig build.zig.zon
	zigZonFile := filepath.Join(tmpDir, "build.zig.zon")
	if err := os.WriteFile(zigZonFile, []byte(".{\n    .name = \"test\",\n    .version = \"0.1.0\",\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// 4. Zig version.zig
	zigVerFile := filepath.Join(tmpDir, "version.zig")
	if err := os.WriteFile(zigVerFile, []byte("pub const version = \"0.1.0\";\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// 5. Rust Cargo.toml
	cargoFile := filepath.Join(tmpDir, "Cargo.toml")
	if err := os.WriteFile(cargoFile, []byte("[package]\nname = \"test\"\nversion = \"0.1.0\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Sync to 0.2.0
	res, err := SyncLanguageFiles(tmpDir, "0.2.0", nil)
	if err != nil {
		t.Fatalf("SyncLanguageFiles error: %v", err)
	}

	if len(res.UpdatedFiles) != 5 {
		t.Errorf("expected 5 updated files, got %d: %v", len(res.UpdatedFiles), res.UpdatedFiles)
	}

	// Verify file contents
	goContent, _ := os.ReadFile(goFile)
	if !strings.Contains(string(goContent), `var Version = "0.2.0"`) {
		t.Errorf("Go file not updated: %s", string(goContent))
	}

	pyContent, _ := os.ReadFile(pyFile)
	if !strings.Contains(string(pyContent), `__version__ = "0.2.0"`) {
		t.Errorf("Python file not updated: %s", string(pyContent))
	}

	zigZonContent, _ := os.ReadFile(zigZonFile)
	if !strings.Contains(string(zigZonContent), `.version = "0.2.0"`) {
		t.Errorf("Zig zon file not updated: %s", string(zigZonContent))
	}

	zigVerContent, _ := os.ReadFile(zigVerFile)
	if !strings.Contains(string(zigVerContent), `pub const version = "0.2.0";`) {
		t.Errorf("Zig version file not updated: %s", string(zigVerContent))
	}

	cargoContent, _ := os.ReadFile(cargoFile)
	if !strings.Contains(string(cargoContent), `version = "0.2.0"`) {
		t.Errorf("Cargo.toml not updated: %s", string(cargoContent))
	}
}

func TestAutoDetectCurrentVersion(t *testing.T) {
	tmpDir := t.TempDir()

	// Initially default
	v, _ := AutoDetectCurrentVersion(tmpDir)
	if v != "0.1.0" {
		t.Errorf("expected default 0.1.0, got %s", v)
	}

	// When version.go exists
	_ = os.WriteFile(filepath.Join(tmpDir, "version.go"), []byte("package main\n\nconst Version = \"1.5.0\"\n"), 0644)
	v, err := AutoDetectCurrentVersion(tmpDir)
	if err != nil || v != "1.5.0" {
		t.Errorf("expected 1.5.0 from version.go, got %s (err: %v)", v, err)
	}

	// When version.yaml exists (takes precedence)
	_ = os.WriteFile(filepath.Join(tmpDir, "version.yaml"), []byte("version: 2.0.1\n"), 0644)
	v, err = AutoDetectCurrentVersion(tmpDir)
	if err != nil || v != "2.0.1" {
		t.Errorf("expected 2.0.1 from version.yaml, got %s (err: %v)", v, err)
	}
}

func TestResolveMinisignKey(t *testing.T) {
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "test.key")
	_ = os.WriteFile(keyFile, []byte("minisign secret key"), 0600)

	// Explicit key
	k, err := ResolveMinisignKey(tmpDir, keyFile)
	if err != nil || k != keyFile {
		t.Errorf("ResolveMinisignKey explicit = %s, err: %v", k, err)
	}

	// Local project key
	localKey := filepath.Join(tmpDir, ".minisign.key")
	_ = os.WriteFile(localKey, []byte("key"), 0600)
	k, err = ResolveMinisignKey(tmpDir, "")
	if err != nil || k != localKey {
		t.Errorf("ResolveMinisignKey local = %s, err: %v", k, err)
	}
}

func TestDryRunRelease(t *testing.T) {
	tmpDir := t.TempDir()
	var buf bytes.Buffer

	// Create dummy key
	keyFile := filepath.Join(tmpDir, ".minisign.key")
	_ = os.WriteFile(keyFile, []byte("dummy-key"), 0600)

	// Create dummy version.go
	_ = os.WriteFile(filepath.Join(tmpDir, "version.go"), []byte("package main\n\nvar Version = \"0.1.0\"\n"), 0644)

	opt := Options{
		Dir:         tmpDir,
		Bump:        "patch",
		DryRun:      true,
		SignKey:     keyFile,
		SkipBuild:   true,
		SkipPublish: true,
		SkipPush:    true,
		Out:         &buf,
	}

	if err := Run(opt); err != nil {
		t.Fatalf("DryRun Run error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "0.1.0 -> 0.1.1") {
		t.Errorf("expected dry-run bump output in %s", out)
	}
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("expected [dry-run] marker in %s", out)
	}
}

func TestDryRunReleaseContinue(t *testing.T) {
	tmpDir := t.TempDir()
	var buf bytes.Buffer

	keyFile := filepath.Join(tmpDir, ".minisign.key")
	_ = os.WriteFile(keyFile, []byte("dummy-key"), 0600)
	_ = os.WriteFile(filepath.Join(tmpDir, "version.yaml"), []byte("version: 0.1.5\n"), 0644)

	opt := Options{
		Dir:         tmpDir,
		Continue:    true,
		DryRun:      true,
		SignKey:     keyFile,
		SkipBuild:   true,
		SkipPublish: true,
		SkipPush:    true,
		Out:         &buf,
	}

	if err := Run(opt); err != nil {
		t.Fatalf("DryRun Run continue error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Continuing release for version: 0.1.5 (tag: v0.1.5)") {
		t.Errorf("expected continue log in %s", out)
	}
}

func TestSemverFormatting(t *testing.T) {
	sv := Semver{
		Major:      1,
		Minor:      2,
		Patch:      3,
		Prerelease: "alpha.1",
		Build:      "20260829",
	}

	if sv.String() != "1.2.3-alpha.1+20260829" {
		t.Errorf("sv.String() = %q, want 1.2.3-alpha.1+20260829", sv.String())
	}
	if sv.TagName() != "v1.2.3-alpha.1+20260829" {
		t.Errorf("sv.TagName() = %q, want v1.2.3-alpha.1+20260829", sv.TagName())
	}
}


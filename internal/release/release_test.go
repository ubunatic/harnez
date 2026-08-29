package release

import (
	"bytes"
	"os"
	"os/exec"
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

func setupTestGitRepo(t *testing.T, remoteURL string) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v (%s)", err, string(out))
	}
	if remoteURL != "" {
		cmd = exec.Command("git", "remote", "add", "origin", remoteURL)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git remote add origin failed: %v (%s)", err, string(out))
		}
	}
	return dir
}

func TestDetectForgeInfo(t *testing.T) {
	tests := []struct {
		name      string
		remoteURL string
		wantHost  string
		wantOwner string
		wantRepo  string
		wantErr   string
	}{
		{
			name:      "Valid Codeberg SSH",
			remoteURL: "git@codeberg.org:myuser/myproject.git",
			wantHost:  "codeberg.org",
			wantOwner: "myuser",
			wantRepo:  "myproject",
		},
		{
			name:      "Valid Codeberg HTTPS",
			remoteURL: "https://codeberg.org/myuser/myproject.git",
			wantHost:  "codeberg.org",
			wantOwner: "myuser",
			wantRepo:  "myproject",
		},
		{
			name:      "Valid GitHub SSH",
			remoteURL: "git@github.com:myorg/myproject.git",
			wantHost:  "github.com",
			wantOwner: "myorg",
			wantRepo:  "myproject",
		},
		{
			name:      "Valid GitHub HTTPS",
			remoteURL: "https://github.com/myorg/myproject",
			wantHost:  "github.com",
			wantOwner: "myorg",
			wantRepo:  "myproject",
		},
		{
			name:      "Valid Codeberg uppercase case-insensitive",
			remoteURL: "https://CodeBerg.Org/MyUser/MyProject.git",
			wantHost:  "CodeBerg.Org",
			wantOwner: "MyUser",
			wantRepo:  "MyProject",
		},
		{
			name:      "Valid GitHub uppercase case-insensitive",
			remoteURL: "git@GITHUB.COM:MyOrg/MyProject.git",
			wantHost:  "GITHUB.COM",
			wantOwner: "MyOrg",
			wantRepo:  "MyProject",
		},
		{
			name:      "Unsupported host GitLab SSH",
			remoteURL: "git@gitlab.com:myuser/myproject.git",
			wantErr:   `unsupported or missing forge remote host "gitlab.com" for origin (must be on codeberg.org or github.com to publish releases)`,
		},
		{
			name:      "Unsupported host custom HTTPS",
			remoteURL: "https://forge.internal.lan/myuser/myproject.git",
			wantErr:   `unsupported or missing forge remote host "forge.internal.lan" for origin (must be on codeberg.org or github.com to publish releases)`,
		},
		{
			name:      "Local directory path",
			remoteURL: "/tmp/local-mirror",
			wantErr:   `unsupported or missing forge remote host "" for origin (must be on codeberg.org or github.com to publish releases)`,
		},
		{
			name:      "Local file URI",
			remoteURL: "file:///tmp/local-mirror",
			wantErr:   `unsupported or missing forge remote host "" for origin (must be on codeberg.org or github.com to publish releases)`,
		},
		{
			name:      "Missing remote origin",
			remoteURL: "",
			wantErr:   `unsupported or missing forge remote host "" for origin (must be on codeberg.org or github.com to publish releases)`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupTestGitRepo(t, tc.remoteURL)
			info, err := DetectForgeInfo(dir)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info == nil {
				t.Fatal("expected non-nil ForgeInfo")
			}
			if info.Host != tc.wantHost {
				t.Errorf("Host = %q, want %q", info.Host, tc.wantHost)
			}
			if info.Owner != tc.wantOwner {
				t.Errorf("Owner = %q, want %q", info.Owner, tc.wantOwner)
			}
			if info.Repo != tc.wantRepo {
				t.Errorf("Repo = %q, want %q", info.Repo, tc.wantRepo)
			}
		})
	}
}

func TestPreflightForgeValidation(t *testing.T) {
	// Missing remote origin
	dir := setupTestGitRepo(t, "")
	keyFile := filepath.Join(dir, ".minisign.key")
	_ = os.WriteFile(keyFile, []byte("dummy-key"), 0600)
	_ = os.WriteFile(filepath.Join(dir, "version.go"), []byte("package main\n\nvar Version = \"0.1.0\"\n"), 0644)

	var buf bytes.Buffer
	opt := Options{
		Dir:         dir,
		Bump:        "patch",
		DryRun:      true,
		SignKey:     keyFile,
		SkipBuild:   true,
		SkipPublish: false,
		SkipPush:    false,
		Out:         &buf,
	}

	err := Run(opt)
	if err == nil {
		t.Fatal("expected Run() to fail when remote origin is missing")
	}
	expectedErr := `unsupported or missing forge remote host "" for origin (must be on codeberg.org or github.com to publish releases)`
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}

	// Unsupported remote host
	dirUnsupported := setupTestGitRepo(t, "https://gitlab.com/owner/repo.git")
	keyFileUnsupported := filepath.Join(dirUnsupported, ".minisign.key")
	_ = os.WriteFile(keyFileUnsupported, []byte("dummy-key"), 0600)
	_ = os.WriteFile(filepath.Join(dirUnsupported, "version.go"), []byte("package main\n\nvar Version = \"0.1.0\"\n"), 0644)

	opt.Dir = dirUnsupported
	opt.SignKey = keyFileUnsupported
	err = Run(opt)
	if err == nil {
		t.Fatal("expected Run() to fail when remote origin is on an unsupported host")
	}
	expectedErrHost := `unsupported or missing forge remote host "gitlab.com" for origin (must be on codeberg.org or github.com to publish releases)`
	if !strings.Contains(err.Error(), expectedErrHost) {
		t.Errorf("expected error %q, got %q", expectedErrHost, err.Error())
	}
}



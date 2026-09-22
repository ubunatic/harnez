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
		{"1.0", "patch", "1.0.1"},
		{"1.0", "minor", "1.1.0"},
		{"1.0", "major", "2.0.0"},
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

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
	}
}

// setupGitRepoWithVersion creates a temp git repo with a committed
// version.yaml at the given version, optionally tagged v<version>.
func setupGitRepoWithVersion(t *testing.T, version string, skipTag bool) string {
	t.Helper()
	dir := t.TempDir()
	runGitCmd(t, dir, "init")
	runGitCmd(t, dir, "config", "user.email", "test@example.com")
	runGitCmd(t, dir, "config", "user.name", "test")
	_ = os.WriteFile(filepath.Join(dir, "version.yaml"), []byte("version: "+version+"\n"), 0644)
	runGitCmd(t, dir, "add", "version.yaml")
	runGitCmd(t, dir, "commit", "-m", "init")
	if !skipTag {
		runGitCmd(t, dir, "tag", "-a", "v"+version, "-m", "v"+version)
	}
	return dir
}

func TestTagExistsAndHasDiffSinceTag(t *testing.T) {
	dir := setupGitRepoWithVersion(t, "1.0.0", false)

	exists, err := tagExists(dir, "v1.0.0")
	if err != nil || !exists {
		t.Fatalf("tagExists(v1.0.0) = %v, %v; want true, nil", exists, err)
	}

	exists, err = tagExists(dir, "v9.9.9")
	if err != nil || exists {
		t.Fatalf("tagExists(v9.9.9) = %v, %v; want false, nil", exists, err)
	}

	hasDiff, err := hasDiffSinceTag(dir, "v1.0.0")
	if err != nil || hasDiff {
		t.Fatalf("hasDiffSinceTag before new commit = %v, %v; want false, nil", hasDiff, err)
	}

	_ = os.WriteFile(filepath.Join(dir, "CHANGES.txt"), []byte("more work\n"), 0644)
	runGitCmd(t, dir, "add", "CHANGES.txt")
	runGitCmd(t, dir, "commit", "-m", "more work")

	hasDiff, err = hasDiffSinceTag(dir, "v1.0.0")
	if err != nil || !hasDiff {
		t.Fatalf("hasDiffSinceTag after new commit = %v, %v; want true, nil", hasDiff, err)
	}
}

func TestReleaseSkipWhenNoDiffSincePrevTag(t *testing.T) {
	tests := []struct {
		name           string
		addExtraCommit bool
		skipTag        bool
		force          bool
		wantSkip       bool
	}{
		{name: "no diff skips release", wantSkip: true},
		{name: "diff present proceeds", addExtraCommit: true, wantSkip: false},
		{name: "force flag overrides no-diff skip", force: true, wantSkip: false},
		{name: "no prior tag proceeds", skipTag: true, wantSkip: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupGitRepoWithVersion(t, "1.0.0", tc.skipTag)
			if tc.addExtraCommit {
				_ = os.WriteFile(filepath.Join(dir, "CHANGES.txt"), []byte("more work\n"), 0644)
				runGitCmd(t, dir, "add", "CHANGES.txt")
				runGitCmd(t, dir, "commit", "-m", "more work")
			}

			var buf bytes.Buffer
			opt := Options{
				Dir:         dir,
				Bump:        "patch",
				DryRun:      true,
				Force:       tc.force,
				SkipBuild:   true,
				SkipSign:    true,
				SkipPublish: true,
				SkipPush:    true,
				Out:         &buf,
			}

			if err := Run(opt); err != nil {
				t.Fatalf("Run error: %v", err)
			}

			out := buf.String()
			skipped := strings.Contains(out, "nothing to release")
			if skipped != tc.wantSkip {
				t.Errorf("skipped = %v, want %v; output=%s", skipped, tc.wantSkip, out)
			}
			if !skipped && !strings.Contains(out, "1.0.0 -> 1.0.1") {
				t.Errorf("expected bump output when not skipped, got: %s", out)
			}
		})
	}
}

type testRemote struct {
	name string
	url  string
}

func setupTestGitRepoWithRemotes(t *testing.T, remotes []testRemote) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v (%s)", err, string(out))
	}
	for _, r := range remotes {
		if r.url != "" {
			cmd = exec.Command("git", "remote", "add", r.name, r.url)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git remote add %s failed: %v (%s)", r.name, err, string(out))
			}
		}
	}
	return dir
}

func setupTestGitRepo(t *testing.T, remoteURL string) string {
	return setupTestGitRepoWithRemotes(t, []testRemote{{"origin", remoteURL}})
}

func TestDetectForgeInfo(t *testing.T) {
	tests := []struct {
		name       string
		remotes    []testRemote
		wantRemote string
		wantHost   string
		wantOwner  string
		wantRepo   string
		wantErr    string
	}{
		{
			name:       "Valid Codeberg SSH origin",
			remotes:    []testRemote{{"origin", "git@codeberg.org:myuser/myproject.git"}},
			wantRemote: "origin",
			wantHost:   "codeberg.org",
			wantOwner:  "myuser",
			wantRepo:   "myproject",
		},
		{
			name:       "Valid Codeberg HTTPS origin",
			remotes:    []testRemote{{"origin", "https://codeberg.org/myuser/myproject.git"}},
			wantRemote: "origin",
			wantHost:   "codeberg.org",
			wantOwner:  "myuser",
			wantRepo:   "myproject",
		},
		{
			name:       "Valid GitHub SSH origin",
			remotes:    []testRemote{{"origin", "git@github.com:myorg/myproject.git"}},
			wantRemote: "origin",
			wantHost:   "github.com",
			wantOwner:  "myorg",
			wantRepo:   "myproject",
		},
		{
			name:       "Valid GitHub HTTPS origin",
			remotes:    []testRemote{{"origin", "https://github.com/myorg/myproject"}},
			wantRemote: "origin",
			wantHost:   "github.com",
			wantOwner:  "myorg",
			wantRepo:   "myproject",
		},
		{
			name:       "Valid Codeberg uppercase case-insensitive",
			remotes:    []testRemote{{"origin", "https://CodeBerg.Org/MyUser/MyProject.git"}},
			wantRemote: "origin",
			wantHost:   "CodeBerg.Org",
			wantOwner:  "MyUser",
			wantRepo:   "MyProject",
		},
		{
			name:       "Valid GitHub uppercase case-insensitive",
			remotes:    []testRemote{{"origin", "git@GITHUB.COM:MyOrg/MyProject.git"}},
			wantRemote: "origin",
			wantHost:   "GITHUB.COM",
			wantOwner:  "MyOrg",
			wantRepo:   "MyProject",
		},
		{
			name: "Local origin with secondary codeberg remote",
			remotes: []testRemote{
				{"origin", "/tmp/local-mirror"},
				{"codeberg", "git@codeberg.org:myuser/myproject.git"},
			},
			wantRemote: "codeberg",
			wantHost:   "codeberg.org",
			wantOwner:  "myuser",
			wantRepo:   "myproject",
		},
		{
			name: "Local origin with secondary github remote",
			remotes: []testRemote{
				{"origin", "/tmp/local-mirror"},
				{"github", "https://github.com/myorg/myproject.git"},
			},
			wantRemote: "github",
			wantHost:   "github.com",
			wantOwner:  "myorg",
			wantRepo:   "myproject",
		},
		{
			name: "Local origin with both github and codeberg remotes (picks codeberg per Priority 2)",
			remotes: []testRemote{
				{"origin", "/tmp/local-mirror"},
				{"github", "https://github.com/myorg/myproject.git"},
				{"codeberg", "git@codeberg.org:myuser/myproject.git"},
			},
			wantRemote: "codeberg",
			wantHost:   "codeberg.org",
			wantOwner:  "myuser",
			wantRepo:   "myproject",
		},
		{
			name: "Priority 1 origin github over secondary codeberg remote",
			remotes: []testRemote{
				{"origin", "git@github.com:myorg/myproject.git"},
				{"codeberg", "git@codeberg.org:myuser/myproject.git"},
			},
			wantRemote: "origin",
			wantHost:   "github.com",
			wantOwner:  "myorg",
			wantRepo:   "myproject",
		},
		{
			name: "Custom named remote forgejo on codeberg",
			remotes: []testRemote{
				{"upstream", "/var/git/local-repo"},
				{"forgejo", "https://codeberg.org/customuser/customrepo.git"},
			},
			wantRemote: "forgejo",
			wantHost:   "codeberg.org",
			wantOwner:  "customuser",
			wantRepo:   "customrepo",
		},
		{
			name:    "Unsupported host GitLab SSH",
			remotes: []testRemote{{"origin", "git@gitlab.com:myuser/myproject.git"}},
			wantErr: `no supported forge remote found (must have at least one remote on codeberg.org or github.com to publish releases)`,
		},
		{
			name:    "Unsupported host custom HTTPS",
			remotes: []testRemote{{"origin", "https://forge.internal.lan/myuser/myproject.git"}},
			wantErr: `no supported forge remote found (must have at least one remote on codeberg.org or github.com to publish releases)`,
		},
		{
			name:    "Local directory path",
			remotes: []testRemote{{"origin", "/tmp/local-mirror"}},
			wantErr: `no supported forge remote found (must have at least one remote on codeberg.org or github.com to publish releases)`,
		},
		{
			name:    "Local file URI",
			remotes: []testRemote{{"origin", "file:///tmp/local-mirror"}},
			wantErr: `no supported forge remote found (must have at least one remote on codeberg.org or github.com to publish releases)`,
		},
		{
			name:    "Missing remotes",
			remotes: nil,
			wantErr: `no supported forge remote found (must have at least one remote on codeberg.org or github.com to publish releases)`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := setupTestGitRepoWithRemotes(t, tc.remotes)
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
			if info.RemoteName != tc.wantRemote {
				t.Errorf("RemoteName = %q, want %q", info.RemoteName, tc.wantRemote)
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
	expectedErr := `no supported forge remote found (must have at least one remote on codeberg.org or github.com to publish releases)`
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
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}

	// Secondary codeberg remote with local origin passes preflight
	dirSecondary := setupTestGitRepoWithRemotes(t, []testRemote{
		{"origin", "/tmp/local-mirror"},
		{"codeberg", "git@codeberg.org:owner/repo.git"},
	})
	keyFileSecondary := filepath.Join(dirSecondary, ".minisign.key")
	_ = os.WriteFile(keyFileSecondary, []byte("dummy-key"), 0600)
	_ = os.WriteFile(filepath.Join(dirSecondary, "version.go"), []byte("package main\n\nvar Version = \"0.1.0\"\n"), 0644)

	opt.Dir = dirSecondary
	opt.SignKey = keyFileSecondary
	opt.SkipPublish = true
	buf.Reset()
	err = Run(opt)
	if err != nil {
		t.Fatalf("expected Run() to succeed with secondary codeberg remote in dry-run, got: %v", err)
	}
	if !strings.Contains(buf.String(), "Would push branch and tag v0.1.1 to codeberg") {
		t.Errorf("expected dry-run push to codeberg, got %s", buf.String())
	}
}

func TestAutoDetectCurrentVersion_ManifestAndPackageJSON(t *testing.T) {
	// 1. Manifest V3 / V2 JSON detection
	tmpDir1 := t.TempDir()
	manifestContent := `{\n  "manifest_version": 3,\n  "name": "LinkLit",\n  "version": "1.0",\n  "description": "test"\n}\n`
	_ = os.WriteFile(filepath.Join(tmpDir1, "manifest.json"), []byte(manifestContent), 0644)

	v, err := AutoDetectCurrentVersion(tmpDir1)
	if err != nil || v != "1.0" {
		t.Errorf("AutoDetectCurrentVersion manifest.json = %q (err: %v), want %q", v, err, "1.0")
	}

	// 2. package.json detection
	tmpDir2 := t.TempDir()
	pkgContent := `{\n  "name": "my-tool",\n  "version": "2.4.1",\n  "main": "index.js"\n}\n`
	_ = os.WriteFile(filepath.Join(tmpDir2, "package.json"), []byte(pkgContent), 0644)

	v, err = AutoDetectCurrentVersion(tmpDir2)
	if err != nil || v != "2.4.1" {
		t.Errorf("AutoDetectCurrentVersion package.json = %q (err: %v), want %q", v, err, "2.4.1")
	}
}

func TestSyncLanguageFiles_ManifestAndPackageJSON(t *testing.T) {
	tmpDir := t.TempDir()

	manifestPath := filepath.Join(tmpDir, "manifest.json")
	manifestOriginal := "{\n  \"manifest_version\": 3,\n  \"name\": \"LinkLit\",\n  \"version\": \"1.0\",\n  \"description\": \"test\"\n}\n"
	_ = os.WriteFile(manifestPath, []byte(manifestOriginal), 0644)

	pkgPath := filepath.Join(tmpDir, "package.json")
	pkgOriginal := "{\n  \"name\": \"test-pkg\",\n  \"version\": \"1.2.0\",\n  \"dependencies\": {}\n}\n"
	_ = os.WriteFile(pkgPath, []byte(pkgOriginal), 0644)

	res, err := SyncLanguageFiles(tmpDir, "1.2.1", nil)
	if err != nil {
		t.Fatalf("SyncLanguageFiles error: %v", err)
	}

	if len(res.UpdatedFiles) != 2 {
		t.Errorf("expected 2 updated files, got %d: %v", len(res.UpdatedFiles), res.UpdatedFiles)
	}

	manifestUpdated, _ := os.ReadFile(manifestPath)
	expectedManifest := "{\n  \"manifest_version\": 3,\n  \"name\": \"LinkLit\",\n  \"version\": \"1.2.1\",\n  \"description\": \"test\"\n}\n"
	if string(manifestUpdated) != expectedManifest {
		t.Errorf("manifest.json mismatch:\ngot:\n%s\nwant:\n%s", string(manifestUpdated), expectedManifest)
	}

	pkgUpdated, _ := os.ReadFile(pkgPath)
	expectedPkg := "{\n  \"name\": \"test-pkg\",\n  \"version\": \"1.2.1\",\n  \"dependencies\": {}\n}\n"
	if string(pkgUpdated) != expectedPkg {
		t.Errorf("package.json mismatch:\ngot:\n%s\nwant:\n%s", string(pkgUpdated), expectedPkg)
	}
}

func TestTagPrefixFormatting(t *testing.T) {
	tests := []struct {
		prefix   string
		version  string
		expected string
	}{
		{"linklit-v", "1.0.1", "linklit-v1.0.1"},
		{"v", "1.0.1", "v1.0.1"},
		{"", "1.0.1", "1.0.1"},
		{"subpkg/v", "2.0.0", "subpkg/v2.0.0"},
	}

	for _, tc := range tests {
		got := FormatTag(tc.prefix, tc.version)
		if got != tc.expected {
			t.Errorf("FormatTag(%q, %q) = %q, want %q", tc.prefix, tc.version, got, tc.expected)
		}
	}

	// Test dry-run with tag_prefix from version.yaml
	tmpDir := t.TempDir()
	var buf bytes.Buffer
	keyFile := filepath.Join(tmpDir, ".minisign.key")
	_ = os.WriteFile(keyFile, []byte("dummy-key"), 0600)
	_ = os.WriteFile(filepath.Join(tmpDir, "version.yaml"), []byte("version: 1.0.0\ntag_prefix: linklit-v\n"), 0644)

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
		t.Fatalf("Run error with tag_prefix: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "tag: linklit-v1.0.1") {
		t.Errorf("expected tag linklit-v1.0.1 in output, got: %s", out)
	}
	if !strings.Contains(out, "Would commit version bump and create tag linklit-v1.0.1") {
		t.Errorf("expected dry-run tag creation for linklit-v1.0.1, got: %s", out)
	}
}

func TestBuildCmdMakefileFallback(t *testing.T) {
	// 1. Fallback to make pack
	tmpDirPack := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDirPack, "Makefile"), []byte("pack:\n\t@echo 'packing'\n"), 0644)
	target := detectMakefileBuildTarget(tmpDirPack)
	if target != "pack" {
		t.Errorf("detectMakefileBuildTarget = %q, want 'pack'", target)
	}

	// 2. Fallback to make dist
	tmpDirDist := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDirDist, "Makefile"), []byte("dist:\n\t@echo 'disting'\n"), 0644)
	target = detectMakefileBuildTarget(tmpDirDist)
	if target != "dist" {
		t.Errorf("detectMakefileBuildTarget = %q, want 'dist'", target)
	}

	// 3. Dry-run execution with pack target
	var buf bytes.Buffer
	keyFile := filepath.Join(tmpDirPack, ".minisign.key")
	_ = os.WriteFile(keyFile, []byte("dummy-key"), 0600)
	_ = os.WriteFile(filepath.Join(tmpDirPack, "version.yaml"), []byte("version: 1.0.0\n"), 0644)

	opt := Options{
		Dir:         tmpDirPack,
		Bump:        "patch",
		DryRun:      true,
		SignKey:     keyFile,
		SkipPublish: true,
		SkipPush:    true,
		Out:         &buf,
	}

	if err := Run(opt); err != nil {
		t.Fatalf("Run error with make pack fallback: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Running make target: pack") {
		t.Errorf("expected output to mention running make target: pack, got: %s", out)
	}
}

func TestBuildEnvGOWORK(t *testing.T) {
	t.Setenv("GOWORK", "/somewhere/go.work")

	env := buildEnv(false)
	found := false
	for _, e := range env {
		if e == "GOWORK=off" {
			found = true
		}
		if strings.HasPrefix(e, "GOWORK=") && e != "GOWORK=off" {
			t.Errorf("buildEnv(false) leaked non-off GOWORK entry: %s", e)
		}
	}
	if !found {
		t.Error("buildEnv(false) expected GOWORK=off in the resulting env")
	}

	env = buildEnv(true)
	for _, e := range env {
		if e == "GOWORK=off" {
			t.Error("buildEnv(true) should not force GOWORK=off")
		}
	}
}

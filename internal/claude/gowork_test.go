package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoWorkspacePolicy(t *testing.T) {
	t.Setenv("GOWORK", "")

	t.Run("default init skips when no go.work exists", func(t *testing.T) {
		target := t.TempDir()
		writeGoModule(t, target, "example.com/target")

		changed, err := reconcileGoWorkspace(target, false)
		if err != nil {
			t.Fatal(err)
		}
		if changed {
			t.Fatal("expected no change on default init without --gowork")
		}
		assertNotExists(t, filepath.Join(target, "go.work"))
		assertNotExists(t, filepath.Join(target, "go.work.example"))
	})

	t.Run("opt-in with --gowork creates go.work.example and symlink", func(t *testing.T) {
		target := t.TempDir()
		writeGoModule(t, target, "example.com/target")

		changed, err := reconcileGoWorkspace(target, true)
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Fatal("expected changed=true on opt-in")
		}

		examplePath := filepath.Join(target, "go.work.example")
		workPath := filepath.Join(target, "go.work")
		gitignorePath := filepath.Join(target, ".gitignore")

		exampleData, err := os.ReadFile(examplePath)
		if err != nil {
			t.Fatalf("read go.work.example: %v", err)
		}
		if !strings.Contains(string(exampleData), managedWorkspaceMarker) {
			t.Fatalf("go.work.example missing managed marker: %s", exampleData)
		}

		fi, err := os.Lstat(workPath)
		if err != nil {
			t.Fatalf("stat go.work: %v", err)
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("go.work is not a symlink")
		}
		linkTarget, err := os.Readlink(workPath)
		if err != nil || linkTarget != "go.work.example" {
			t.Fatalf("go.work symlink target = %q, want go.work.example", linkTarget)
		}

		gitignoreData, err := os.ReadFile(gitignorePath)
		if err != nil {
			t.Fatalf("read .gitignore: %v", err)
		}
		if !strings.Contains(string(gitignoreData), "/go.work") {
			t.Fatalf(".gitignore missing /go.work: %s", gitignoreData)
		}
	})

	t.Run("default init maintains existing managed workspace", func(t *testing.T) {
		target := t.TempDir()
		writeGoModule(t, target, "example.com/target")

		examplePath := filepath.Join(target, "go.work.example")
		if err := os.WriteFile(examplePath, []byte(managedWorkspaceMarker+"\ngo 1.23\nuse .\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		changed, err := reconcileGoWorkspace(target, false)
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Fatal("expected symlink and gitignore creation to report change")
		}

		workPath := filepath.Join(target, "go.work")
		fi, err := os.Lstat(workPath)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("go.work not created as symlink: %v", err)
		}
	})

	t.Run("unmanaged local go.work is skipped without opt-in", func(t *testing.T) {
		target := t.TempDir()
		writeGoModule(t, target, "example.com/target")

		workPath := filepath.Join(target, "go.work")
		original := []byte("go 1.23\nuse .\n")
		if err := os.WriteFile(workPath, original, 0o644); err != nil {
			t.Fatal(err)
		}

		changed, err := reconcileGoWorkspace(target, false)
		if err != nil {
			t.Fatal(err)
		}
		if changed {
			t.Fatal("expected no change on unmanaged go.work without --gowork")
		}
		assertNotExists(t, filepath.Join(target, "go.work.example"))
		data, _ := os.ReadFile(workPath)
		if string(data) != string(original) {
			t.Fatalf("go.work modified: %s", data)
		}
	})

	t.Run("opt-in adopts unmanaged go.work into go.work.example and symlinks", func(t *testing.T) {
		target := t.TempDir()
		writeGoModule(t, target, "example.com/target")

		workPath := filepath.Join(target, "go.work")
		original := "go 1.23\nuse .\n"
		if err := os.WriteFile(workPath, []byte(original), 0o644); err != nil {
			t.Fatal(err)
		}

		changed, err := reconcileGoWorkspace(target, true)
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Fatal("expected change on adopting unmanaged go.work")
		}

		examplePath := filepath.Join(target, "go.work.example")
		exampleData, err := os.ReadFile(examplePath)
		if err != nil {
			t.Fatalf("read go.work.example: %v", err)
		}
		if !strings.Contains(string(exampleData), managedWorkspaceMarker) || !strings.Contains(string(exampleData), original) {
			t.Fatalf("unexpected go.work.example content: %s", exampleData)
		}

		fi, err := os.Lstat(workPath)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("go.work not converted to symlink: %v", err)
		}
	})
}

func writeGoModule(t *testing.T, dir, module string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "module " + module + "\n\ngo 1.23\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s not to exist, stat error: %v", path, err)
	}
}

func TestFindGoModules_IgnoresDotDirsAndTestData(t *testing.T) {
	root := t.TempDir()
	writeGoModule(t, root, "example.com/root")
	writeGoModule(t, filepath.Join(root, ".cache", "mod1"), "example.com/cache")
	writeGoModule(t, filepath.Join(root, ".git", "mod2"), "example.com/git")
	writeGoModule(t, filepath.Join(root, "testdata", "fixture1"), "example.com/fixture")
	writeGoModule(t, filepath.Join(root, "subpkg"), "example.com/subpkg")

	dirs, args, err := findGoModules(root)
	if err != nil {
		t.Fatalf("findGoModules failed: %v", err)
	}

	if len(dirs) != 2 {
		t.Fatalf("expected 2 discovered modules (root and subpkg), got %d: %v", len(dirs), dirs)
	}
	if len(args) != 2 || args[0] != "." || args[1] != "./subpkg" {
		t.Fatalf("unexpected module args: %v", args)
	}
}

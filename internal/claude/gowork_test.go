package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoWorkspacePolicy(t *testing.T) {
	t.Setenv("GOWORK", "")

	t.Run("unrelated parent workspace", func(t *testing.T) {
		parent := t.TempDir()
		member := filepath.Join(parent, "member")
		target := filepath.Join(parent, "target")
		writeGoModule(t, member, "example.com/member")
		writeGoModule(t, target, "example.com/target")
		mustRunGo(t, parent, "work", "init", "./member")

		if output, err := runGoCommand(target, "list", "./..."); err == nil {
			t.Fatalf("expected ambient workspace to reject unrelated module, got success: %s", output)
		}
		plan, err := planGoWorkspace(target, runGoCommand)
		if err != nil {
			t.Fatal(err)
		}
		if plan.action != goWorkCreate {
			t.Fatalf("action = %v, want create; reason: %s", plan.action, plan.reason)
		}
		if !strings.Contains(plan.reason, "omits") {
			t.Fatalf("decision does not explain parent omission: %q", plan.reason)
		}

		changed, err := reconcileGoWorkspace(target)
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Fatal("reconcileGoWorkspace reported no change")
		}
		assertWorkspaceUses(t, filepath.Join(target, "go.work"), target)
		mustRunGo(t, target, "list", "-m")
	})

	t.Run("project already listed in parent", func(t *testing.T) {
		parent := t.TempDir()
		target := filepath.Join(parent, "target")
		writeGoModule(t, target, "example.com/target")
		mustRunGo(t, parent, "work", "init", "./target")

		plan, err := planGoWorkspace(target, runGoCommand)
		if err != nil {
			t.Fatal(err)
		}
		if plan.action != goWorkSkip || !strings.Contains(plan.reason, "already included") {
			t.Fatalf("unexpected decision: action=%v reason=%q", plan.action, plan.reason)
		}
		assertWorkspaceUnchanged(t, target)
		assertNotExists(t, filepath.Join(target, "go.work"))
	})

	t.Run("no parent workspace", func(t *testing.T) {
		target := t.TempDir()
		writeGoModule(t, target, "example.com/target")

		plan, err := planGoWorkspace(target, runGoCommand)
		if err != nil {
			t.Fatal(err)
		}
		if plan.action != goWorkSkip || plan.reason != "Go reports no active workspace" {
			t.Fatalf("unexpected decision: action=%v reason=%q", plan.action, plan.reason)
		}
		assertWorkspaceUnchanged(t, target)
		assertNotExists(t, filepath.Join(target, "go.work"))
	})

	t.Run("no Go module", func(t *testing.T) {
		parent := t.TempDir()
		member := filepath.Join(parent, "member")
		target := filepath.Join(parent, "target")
		writeGoModule(t, member, "example.com/member")
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatal(err)
		}
		mustRunGo(t, parent, "work", "init", "./member")

		plan, err := planGoWorkspace(target, runGoCommand)
		if err != nil {
			t.Fatal(err)
		}
		if plan.action != goWorkSkip || plan.reason != "no Go modules found" {
			t.Fatalf("unexpected decision: action=%v reason=%q", plan.action, plan.reason)
		}
		assertWorkspaceUnchanged(t, target)
		assertNotExists(t, filepath.Join(target, "go.work"))
	})

	t.Run("existing local go.work", func(t *testing.T) {
		target := t.TempDir()
		workFile := filepath.Join(target, "go.work")
		original := []byte("custom workspace content\n")
		if err := os.WriteFile(workFile, original, 0o644); err != nil {
			t.Fatal(err)
		}

		changed, err := reconcileGoWorkspace(target)
		if err != nil {
			t.Fatal(err)
		}
		if changed {
			t.Fatal("existing local go.work was reported as changed")
		}
		got, err := os.ReadFile(workFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(original) {
			t.Fatalf("existing go.work changed:\ngot:  %q\nwant: %q", got, original)
		}
	})

	t.Run("multiple modules with one omitted by parent", func(t *testing.T) {
		parent := t.TempDir()
		target := filepath.Join(parent, "target")
		tools := filepath.Join(target, "tools")
		writeGoModule(t, target, "example.com/target")
		writeGoModule(t, tools, "example.com/target/tools")
		mustRunGo(t, parent, "work", "init", "./target")

		changed, err := reconcileGoWorkspace(target)
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Fatal("expected local workspace for omitted nested module")
		}
		assertWorkspaceUses(t, filepath.Join(target, "go.work"), target, tools)
	})

	t.Run("GOWORK off", func(t *testing.T) {
		t.Setenv("GOWORK", "off")
		target := t.TempDir()
		writeGoModule(t, target, "example.com/target")

		plan, err := planGoWorkspace(target, runGoCommand)
		if err != nil {
			t.Fatal(err)
		}
		if plan.action != goWorkSkip || !strings.Contains(plan.reason, "GOWORK=off") {
			t.Fatalf("unexpected decision: action=%v reason=%q", plan.action, plan.reason)
		}
		assertWorkspaceUnchanged(t, target)
		assertNotExists(t, filepath.Join(target, "go.work"))
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

func mustRunGo(t *testing.T, dir string, args ...string) string {
	t.Helper()
	output, err := runGoCommand(dir, args...)
	if err != nil {
		t.Fatalf("go %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return output
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s not to exist, stat error: %v", path, err)
	}
}

func assertWorkspaceUnchanged(t *testing.T, dir string) {
	t.Helper()
	changed, err := reconcileGoWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("workspace was unexpectedly changed")
	}
}

func assertWorkspaceUses(t *testing.T, workFile string, want ...string) {
	t.Helper()
	data := mustRunGo(t, filepath.Dir(workFile), "work", "edit", "-json")
	var parsed struct {
		Use []struct {
			DiskPath string
		}
	}
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		t.Fatal(err)
	}
	got := make(map[string]bool, len(parsed.Use))
	for _, use := range parsed.Use {
		path := use.DiskPath
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(workFile), path)
		}
		got[cleanExistingPath(path)] = true
	}
	for _, path := range want {
		if !got[cleanExistingPath(path)] {
			t.Errorf("workspace %s does not use %s", workFile, path)
		}
	}
}

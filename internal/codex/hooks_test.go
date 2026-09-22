package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "config.toml")

	changed, err := Apply(path)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on first apply")
	}

	installed, drifted := Status(path)
	if !installed || drifted {
		t.Fatalf("Status = installed=%v drifted=%v, want true/false", installed, drifted)
	}

	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "hooks = true") {
		t.Fatalf("expected Codex lifecycle hooks feature enabled, got:\n%s", data)
	}

	// idempotent: second apply is a no-op.
	changed, err = Apply(path)
	if err != nil {
		t.Fatalf("Apply (2nd): %v", err)
	}
	if changed {
		t.Fatal("expected changed=false on repeat apply")
	}
}

func TestSummaryIncludesLifecycleHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if _, err := Apply(path); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := "enabled (PreToolUse 1, PostToolUse 1, PreCompact 1, PostCompact 1, SessionStart 1, Stop 1, SessionEnd 1)"
	if got := Summary(path); got != want {
		t.Fatalf("Summary = %q, want %q", got, want)
	}
}

func TestApplyPreservesOtherHooksAndKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	initial := `unrelatedTopLevelKey = "keep-me"

[hooks.someone-elses-hook]
enabled = true
`
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Apply(path); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "someone-elses-hook") {
		t.Errorf("expected other named hook preserved, got:\n%s", got)
	}
	if !strings.Contains(got, "unrelatedTopLevelKey") {
		t.Errorf("expected unrelated top-level key preserved, got:\n%s", got)
	}
	if !strings.Contains(got, HookName) {
		t.Errorf("expected harnez hook entry present, got:\n%s", got)
	}
}

func TestStatusDetectsDrift(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if _, err := Apply(path); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Hand-edit the harnez group (matcher changed, still harnez-owned).
	drifted := `[features]
hooks = true

[[hooks.PreToolUse]]
matcher = "Other"

[[hooks.PreToolUse.hooks]]
type = "command"
command = "harnez codex-hook"
`
	if err := os.WriteFile(path, []byte(drifted), 0644); err != nil {
		t.Fatal(err)
	}

	installed, isDrifted := Status(path)
	if !installed {
		t.Fatal("expected installed=true")
	}
	if !isDrifted {
		t.Fatal("expected drifted=true after hand-editing the harnez entry")
	}
}

func TestApplyPreservesOtherFeatures(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("[features]\napps = false\nplugins = false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{"apps = false", "plugins = false", "hooks = true"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestStatusMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist", "config.toml")
	installed, drifted := Status(path)
	if installed || drifted {
		t.Fatalf("Status on missing file = installed=%v drifted=%v, want false/false", installed, drifted)
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	initial := `unrelatedTopLevelKey = "keep-me"

[hooks.someone-elses-hook]
enabled = true

[hooks.harnez]
enabled = true
`
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	changed, err := Remove(path)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true")
	}

	installed, _ := Status(path)
	if installed {
		t.Fatal("expected harnez entry removed")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "someone-elses-hook") || !strings.Contains(got, "unrelatedTopLevelKey") {
		t.Errorf("expected unrelated content preserved after Remove, got:\n%s", got)
	}

	// second remove is a no-op
	changed, err = Remove(path)
	if err != nil {
		t.Fatalf("Remove (2nd): %v", err)
	}
	if changed {
		t.Fatal("expected changed=false on repeat remove")
	}
}

func TestRemoveDeletesEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if _, err := Apply(path); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if _, err := Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file removed once empty, stat err=%v", err)
	}
}

const userGroupTOML = `
[[hooks.PreToolUse]]
matcher = "Edit"

[[hooks.PreToolUse.hooks]]
type = "command"
command = "my-lint"
`

func TestApplyPreservesUserGroupsOnHarnezEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(userGroupTOML), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(path); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "my-lint") {
		t.Fatalf("user PreToolUse group lost:\n%s", data)
	}
	if installed, drifted := Status(path); !installed || drifted {
		t.Errorf("Status = installed %v, drifted %v; want true, false with a user group present", installed, drifted)
	}
	changed, err := Apply(path)
	if err != nil {
		t.Fatalf("Apply (2nd): %v", err)
	}
	if changed {
		t.Error("second Apply changed the file; want idempotent")
	}
	if n := strings.Count(mustRead(t, path), "harnez codex-hook"); n != 1 {
		t.Errorf("harnez codex-hook appears %d times, want 1", n)
	}
}

func TestRemoveKeepsUserGroupsAndFeature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(userGroupTOML), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(path); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	changed, err := Remove(path)
	if err != nil || !changed {
		t.Fatalf("Remove = %v, %v", changed, err)
	}
	got := mustRead(t, path)
	if strings.Contains(got, "harnez ") {
		t.Errorf("harnez groups left:\n%s", got)
	}
	if !strings.Contains(got, "my-lint") {
		t.Errorf("user group removed:\n%s", got)
	}
	if !strings.Contains(got, "hooks = true") {
		t.Errorf("features.hooks dropped although user hooks remain:\n%s", got)
	}
	if installed, _ := Status(path); installed {
		t.Error("Status reports harnez installed after Remove")
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

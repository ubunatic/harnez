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

	// idempotent: second apply is a no-op.
	changed, err = Apply(path)
	if err != nil {
		t.Fatalf("Apply (2nd): %v", err)
	}
	if changed {
		t.Fatal("expected changed=false on repeat apply")
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

	drifted := `[hooks.harnez]
enabled = false
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

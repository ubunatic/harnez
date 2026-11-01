package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// adapterOwnershipSetup isolates ApplyAllVariant to a throwaway home: every
// target lives under it, Pi's runner is disabled, and only the distill adapter
// paths are of interest to the caller (issue 1061 M1).
func adapterOwnershipSetup(t *testing.T) (target string, cfg *Config, piPath, openCodePath string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PI_CODING_AGENT_DIR", "")
	cfg = loadTestConfig(t)
	cfg.ClaudeSkillsTarget = filepath.Join(home, "claude-skills")
	cfg.SkillsTarget = filepath.Join(home, "gemini-skills")
	cfg.CodexSkillsTarget = filepath.Join(home, "codex-skills")
	cfg.PrimeAgentTarget = ""
	cfg.PiAgentTarget = ""
	cfg.CodexHooksTarget = ""
	cfg.AgyHooksTarget = ""
	cfg.DistillAutopipe = DistillAutopipe{
		PiExtensionTarget:    "~/.pi/agent/extensions/harnez-distill.ts",
		OpenCodePluginTarget: "~/.config/opencode/plugins/harnez-distill.ts",
	}
	piPath = filepath.Join(home, ".pi", "agent", "extensions", "harnez-distill.ts")
	openCodePath = filepath.Join(home, ".config", "opencode", "plugins", "harnez-distill.ts")
	target = filepath.Join(home, ".claude")
	return target, cfg, piPath, openCodePath
}

func writeAdapterTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

const userPiExtension = "// My own Pi extension, not written by harnez.\nexport default function () {}\n"

// A user-authored file at the Pi adapter path survives install, and apply
// reports it as kept instead of overwriting it.
func TestDistillAdapterUserFileKeptOnInstall(t *testing.T) {
	target, cfg, piPath, openCodePath := adapterOwnershipSetup(t)
	writeAdapterTestFile(t, piPath, userPiExtension)

	out, err := CaptureTestStdout(func() error {
		return ApplyAllVariant(target, cfg, ApplyOptions{})
	})
	if err != nil {
		t.Fatalf("apply must succeed when an adapter path holds a user file: %v", err)
	}
	if got, _ := os.ReadFile(piPath); string(got) != userPiExtension {
		t.Fatalf("user-authored Pi adapter was overwritten:\n%s", got)
	}
	if !strings.Contains(out, "kept "+piPath+": not harnez-managed") {
		t.Fatalf("apply must report the kept adapter, got:\n%s", out)
	}
	if got, _ := os.ReadFile(openCodePath); string(got) != openCodeDistillAdapter {
		t.Fatalf("missing OpenCode adapter must install normally, got:\n%s", got)
	}
}

// The same user file must survive a selection that drops telemetry, which
// today deletes it without warning.
func TestDistillAdapterUserFileKeptOnRemove(t *testing.T) {
	target, cfg, piPath, _ := adapterOwnershipSetup(t)
	writeAdapterTestFile(t, piPath, userPiExtension)

	out, err := CaptureTestStdout(func() error {
		return ApplyAllVariant(target, cfg, ApplyOptions{Selection: Set{Skills: true}})
	})
	if err != nil {
		t.Fatalf("apply without telemetry must succeed: %v", err)
	}
	if got, _ := os.ReadFile(piPath); string(got) != userPiExtension {
		t.Fatalf("user-authored Pi adapter was removed or changed:\n%s", got)
	}
	if !strings.Contains(out, "kept "+piPath+": not harnez-managed") {
		t.Fatalf("apply must report the kept adapter on removal, got:\n%s", out)
	}
}

// A symlink at an adapter path is never followed or replaced, on either path.
func TestDistillAdapterSymlinkKept(t *testing.T) {
	target, cfg, piPath, _ := adapterOwnershipSetup(t)
	elsewhere := filepath.Join(t.TempDir(), "elsewhere.ts")
	writeAdapterTestFile(t, elsewhere, userPiExtension)
	if err := os.MkdirAll(filepath.Dir(piPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, piPath); err != nil {
		t.Fatal(err)
	}

	if _, err := CaptureTestStdout(func() error {
		return ApplyAllVariant(target, cfg, ApplyOptions{})
	}); err != nil {
		t.Fatalf("install over a symlink must succeed: %v", err)
	}
	if got, _ := os.ReadFile(elsewhere); string(got) != userPiExtension {
		t.Fatalf("apply wrote through a symlink into %s:\n%s", elsewhere, got)
	}
	if _, err := CaptureTestStdout(func() error {
		return ApplyAllVariant(target, cfg, ApplyOptions{Selection: Set{Skills: true}})
	}); err != nil {
		t.Fatalf("removal over a symlink must succeed: %v", err)
	}
	fi, err := os.Lstat(piPath)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink at adapter path was removed: fi=%v err=%v", fi, err)
	}
}

// A symlink whose target carries the harnez header is still a symlink: apply
// must not classify it as managed, so it never writes through it and never
// removes it (issue 1061 review item 3; the regular-file guard in
// inspectDistillAdapter is what this pins).
func TestDistillAdapterSymlinkToManagedHeaderKept(t *testing.T) {
	target, cfg, piPath, _ := adapterOwnershipSetup(t)
	elsewhere := filepath.Join(t.TempDir(), "elsewhere.ts")
	managed := distillAdapterHeader + "\n// harnez content behind a symlink\n"
	writeAdapterTestFile(t, elsewhere, managed)
	if err := os.MkdirAll(filepath.Dir(piPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, piPath); err != nil {
		t.Fatal(err)
	}

	if owner, err := inspectDistillAdapter(piPath); err != nil || owner != adapterKept {
		t.Fatalf("symlink to a managed-header file: owner=%v err=%v, want adapterKept", owner, err)
	}
	if _, err := CaptureTestStdout(func() error {
		return ApplyAllVariant(target, cfg, ApplyOptions{})
	}); err != nil {
		t.Fatalf("full apply over a symlink: %v", err)
	}
	if got, _ := os.ReadFile(elsewhere); string(got) != managed {
		t.Fatalf("apply wrote through the symlink:\n%s", got)
	}
	prune, err := Resolve([]string{"skills"}, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CaptureTestStdout(func() error {
		return ApplyAllVariant(target, cfg, ApplyOptions{Selection: prune})
	}); err != nil {
		t.Fatalf("pruned apply over a symlink: %v", err)
	}
	fi, err := os.Lstat(piPath)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink at adapter path was removed: fi=%v err=%v", fi, err)
	}
	if got, _ := os.ReadFile(elsewhere); string(got) != managed {
		t.Fatalf("prune changed the symlink target:\n%s", got)
	}
}

// Managed files, including ones written by older harnez versions that share
// the header line, are updated on install and removed on removal.
func TestDistillAdapterManagedFileUpdatedAndRemoved(t *testing.T) {
	target, cfg, piPath, _ := adapterOwnershipSetup(t)
	olderManaged := "// Managed by harnez.\n// older generated body\nexport {};\n"
	writeAdapterTestFile(t, piPath, olderManaged)

	if _, err := CaptureTestStdout(func() error {
		return ApplyAllVariant(target, cfg, ApplyOptions{})
	}); err != nil {
		t.Fatalf("install over an older managed adapter: %v", err)
	}
	if got, _ := os.ReadFile(piPath); string(got) != piDistillAdapter {
		t.Fatalf("older managed adapter was not updated to the current content:\n%s", got)
	}

	if _, err := CaptureTestStdout(func() error {
		return ApplyAllVariant(target, cfg, ApplyOptions{Selection: Set{Skills: true}})
	}); err != nil {
		t.Fatalf("removal of managed adapter: %v", err)
	}
	if _, err := os.Lstat(piPath); !os.IsNotExist(err) {
		t.Fatalf("managed adapter should be removed without telemetry, stat err=%v", err)
	}
}

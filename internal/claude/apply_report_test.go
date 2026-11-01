package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// reportSetup extends adapterOwnershipSetup with Codex and Antigravity hook
// files under the same throwaway home (issue 1061 M2).
func reportSetup(t *testing.T) (target string, cfg *Config, piPath, codexPath, agyPath string) {
	t.Helper()
	target, cfg, piPath, _ = adapterOwnershipSetup(t)
	home := filepath.Dir(target)
	codexPath = filepath.Join(home, ".codex", "config.toml")
	agyPath = filepath.Join(home, ".gemini", "config", "hooks.json")
	if err := os.MkdirAll(filepath.Join(home, ".gemini"), 0755); err != nil {
		t.Fatal(err)
	}
	cfg.CodexHooksTarget = codexPath
	cfg.AgyHooksTarget = agyPath
	return target, cfg, piPath, codexPath, agyPath
}

// Removing Codex and Antigravity hooks edits their files; apply must say so
// and must not claim it removed the file, and the run ends with a summary
// naming the removal and its reason.
func TestApplyReportsHookRemovalTruthfully(t *testing.T) {
	target, cfg, _, codexPath, agyPath := reportSetup(t)
	// A user key keeps each file from becoming empty, so the removal edits it.
	writeAdapterTestFile(t, codexPath, "keep_me = true\n")
	writeAdapterTestFile(t, agyPath, "{\"userSetting\": true}\n")
	if _, err := CaptureTestStdout(func() error {
		return ApplyAllVariant(target, cfg, ApplyOptions{})
	}); err != nil {
		t.Fatalf("full apply: %v", err)
	}

	out, err := CaptureTestStdout(func() error {
		return ApplyAllVariant(target, cfg, ApplyOptions{Selection: Set{Skills: true}})
	})
	if err != nil {
		t.Fatalf("skills-only apply: %v", err)
	}
	for _, p := range []string{codexPath, agyPath} {
		if !strings.Contains(out, "removed harnez hooks from "+p) {
			t.Errorf("apply must name the harnez hooks removed from %s, got:\n%s", p, out)
		}
		if strings.Contains(out, "removed "+p+"\n") {
			t.Errorf("apply claims it removed the file %s, which it only edited:\n%s", p, out)
		}
	}
	if !strings.Contains(out, "removed:") || !strings.Contains(out, "(telemetry not selected)") {
		t.Errorf("apply must end with a removal summary naming the reason, got:\n%s", out)
	}
	if _, err := os.Stat(codexPath); err != nil {
		t.Errorf("codex config must survive the hook removal: %v", err)
	}
}

// diff names every adapter, hook and skill that apply would remove, and
// reports drift for them, before apply runs.
func TestDiffNamesEveryRemovalApplyWouldMake(t *testing.T) {
	target, cfg, piPath, codexPath, agyPath := reportSetup(t)
	if _, err := CaptureTestStdout(func() error {
		return ApplyAllVariant(target, cfg, ApplyOptions{})
	}); err != nil {
		t.Fatalf("full apply: %v", err)
	}

	skillsOnly := Set{Docs: true, Skills: true, Agents: true, Usage: true}
	var changed bool
	out, err := CaptureTestStdout(func() error {
		var dErr error
		changed, dErr = DiffAll(target, cfg, skillsOnly)
		return dErr
	})
	if err != nil {
		t.Fatalf("DiffAll: %v", err)
	}
	if !changed {
		t.Fatalf("DiffAll must report drift when apply would remove hooks, adapters and skills")
	}
	wants := []string{
		"would remove harnez hooks from " + codexPath + " (telemetry not selected)",
		"would remove harnez hooks from " + agyPath + " (telemetry not selected)",
		"would remove " + piPath + " (telemetry not selected)",
		"would remove " + filepath.Join(filepath.Dir(target), "claude-skills", "tool-feedback-protocol", "SKILL.md") + " (requires telemetry not selected)",
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("diff output lacks %q:\n%s", want, out)
		}
	}
}

// With only a managed adapter left to remove, diff reports drift; that is
// what makes `diff --exit-code` fail.
func TestDiffFailsWhenOnlyAdapterWouldChange(t *testing.T) {
	target, cfg, piPath, _, _ := reportSetup(t)
	cfg.Skills = nil
	cfg.Hooks = nil
	cfg.CodexHooksTarget = ""
	cfg.AgyHooksTarget = ""
	if _, err := CaptureTestStdout(func() error {
		return ApplyAllVariant(target, cfg, ApplyOptions{})
	}); err != nil {
		t.Fatalf("full apply: %v", err)
	}
	var changed bool
	out, err := CaptureTestStdout(func() error {
		var dErr error
		changed, dErr = DiffAll(target, cfg, Set{Docs: true, Skills: true, Agents: true, Usage: true})
		return dErr
	})
	if err != nil {
		t.Fatalf("DiffAll: %v", err)
	}
	if !changed || !strings.Contains(out, "would remove "+piPath) {
		t.Fatalf("diff must fail on an adapter-only removal (changed=%v), got:\n%s", changed, out)
	}
}

// A user-authored adapter is named as kept by diff but is not drift: apply
// would change nothing, so diff --exit-code stays clean.
func TestDiffKeepsUserAdapterWithoutDrift(t *testing.T) {
	target, cfg, piPath, _, _ := reportSetup(t)
	cfg.Skills = nil
	cfg.Hooks = nil
	cfg.CodexHooksTarget = ""
	cfg.AgyHooksTarget = ""
	cfg.DistillAutopipe.OpenCodePluginTarget = ""
	if _, err := CaptureTestStdout(func() error {
		return ApplyAllVariant(target, cfg, ApplyOptions{})
	}); err != nil {
		t.Fatalf("full apply: %v", err)
	}
	writeAdapterTestFile(t, piPath, userPiExtension)

	var changed bool
	out, err := CaptureTestStdout(func() error {
		var dErr error
		changed, dErr = DiffAll(target, cfg, Set{Docs: true, Skills: true, Agents: true, Usage: true})
		return dErr
	})
	if err != nil {
		t.Fatalf("DiffAll: %v", err)
	}
	if changed {
		t.Fatalf("a kept user adapter is not drift, got changed=true:\n%s", out)
	}
	if !strings.Contains(out, "kept "+piPath+": not harnez-managed") {
		t.Fatalf("diff must name the kept adapter, got:\n%s", out)
	}
}

package claude

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/agy"
	"ubunatic.com/harnez/internal/codex"
	"ubunatic.com/harnez/internal/fsutil"
)

// scopedFixture is one throwaway home with every managed component installed:
// Claude settings, Codex and Antigravity hooks, both distill adapters, the
// managed skills and the Pi background-task runner (issue 1061 M3).
func scopedFixture(t *testing.T) (target string, cfg *Config) {
	t.Helper()
	target, cfg, _, _, _ = reportSetup(t)
	cfg.PiAgentTarget = filepath.Join(filepath.Dir(target), ".pi", "agent")
	return target, cfg
}

// scopedSnapshot maps every file under the managed locations to its bytes, so
// a test can say "nothing changed" without listing paths by hand.
func scopedSnapshot(t *testing.T, target string, cfg *Config) map[string]string {
	t.Helper()
	home := filepath.Dir(target)
	snap := map[string]string{}
	for _, root := range []string{target, cfg.ClaudeSkillsTarget, cfg.PiAgentTarget, filepath.Join(home, ".codex"), filepath.Join(home, ".gemini")} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			snap[path] = string(data)
			return nil
		})
		if err != nil {
			t.Fatalf("snapshot %s: %v", root, err)
		}
	}
	return snap
}

// diffSnapshots names the paths that differ between two snapshots.
func diffSnapshots(want, got map[string]string) []string {
	var out []string
	for path := range maps.Keys(want) {
		if g, ok := got[path]; !ok {
			out = append(out, "removed "+path)
		} else if g != want[path] {
			out = append(out, "changed "+path)
		}
	}
	for path := range maps.Keys(got) {
		if _, ok := want[path]; !ok {
			out = append(out, "added "+path)
		}
	}
	slices.Sort(out)
	return out
}

// TestScopedApplyLeavesUnselectedComponentsUntouched is the acceptance table
// for issue 1061: from a full apply, a flag selection without --prune changes
// nothing outside it and reports no drift (D1).
func TestScopedApplyLeavesUnselectedComponentsUntouched(t *testing.T) {
	target, cfg := scopedFixture(t)
	if err := ApplyAllVariant(target, cfg, ApplyOptions{}); err != nil {
		t.Fatalf("full apply: %v", err)
	}
	baseline := scopedSnapshot(t, target, cfg)

	for _, names := range [][]string{{"skills"}, {"usage"}, {"docs"}, {"telemetry"}, {"agents"}} {
		t.Run(strings.Join(names, ","), func(t *testing.T) {
			set, err := Resolve(names, nil, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := ApplyAllVariant(target, cfg, ApplyOptions{Selection: set}); err != nil {
				t.Fatalf("apply: %v", err)
			}
			if d := diffSnapshots(baseline, scopedSnapshot(t, target, cfg)); len(d) > 0 {
				t.Errorf("unselected components changed: %v", d)
			}
			if changed, err := DiffAll(target, cfg, set); err != nil || changed {
				t.Errorf("scoped diff after scoped apply: changed=%v err=%v, want clean", changed, err)
			}
		})
	}
}

// --prune (desired state, D1) still removes what the selection leaves out, and
// a plain apply afterwards restores the full state byte for byte.
func TestPruneRemovesUnselectedAndPlainApplyRestores(t *testing.T) {
	target, cfg := scopedFixture(t)
	if err := ApplyAllVariant(target, cfg, ApplyOptions{}); err != nil {
		t.Fatalf("full apply: %v", err)
	}
	baseline := scopedSnapshot(t, target, cfg)

	home := filepath.Dir(target)
	piAdapter := fsutil.ExpandHome(cfg.DistillAutopipe.PiExtensionTarget)
	rateSkill := filepath.Join(cfg.ClaudeSkillsTarget, "tool-feedback-protocol", "SKILL.md")
	codexPath := filepath.Join(home, ".codex", "config.toml")
	agyPath := filepath.Join(home, ".gemini", "config", "hooks.json")
	// Baseline: each thing the prune must remove is present after the full apply,
	// so an absent file afterwards means the prune removed it.
	if installed, _ := codex.Status(codexPath); !installed {
		t.Fatal("baseline: codex hook not installed after full apply")
	}
	if installed, _ := agy.Status(agyPath); !installed {
		t.Fatal("baseline: agy hook not installed after full apply")
	}
	for _, p := range []string{piAdapter, rateSkill} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("baseline: %s missing after full apply: %v", p, err)
		}
	}

	set, err := Resolve([]string{"skills"}, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyAllVariant(target, cfg, ApplyOptions{Selection: set}); err != nil {
		t.Fatalf("pruned apply: %v", err)
	}
	if installed, _ := codex.Status(codexPath); installed {
		t.Error("desired-state run left the Codex hook installed")
	}
	if installed, _ := agy.Status(agyPath); installed {
		t.Error("desired-state run left the Antigravity hook installed")
	}
	if _, err := os.Stat(piAdapter); err == nil {
		t.Error("desired-state run left the Pi distill adapter")
	}
	if _, err := os.Stat(rateSkill); err == nil {
		t.Error("desired-state run left the requires-telemetry skill")
	}

	if err := ApplyAllVariant(target, cfg, ApplyOptions{}); err != nil {
		t.Fatalf("plain apply after prune: %v", err)
	}
	if d := diffSnapshots(baseline, scopedSnapshot(t, target, cfg)); len(d) > 0 {
		t.Errorf("plain apply after prune does not restore the full state: %v", d)
	}
}

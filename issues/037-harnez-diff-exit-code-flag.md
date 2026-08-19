# 037 — `harnez diff --exit-code` for Drift Detection & CI Pipelines

**Status**: Closed  
**Category**: Feature / CLI Contract  
**Related**: [Issue 004: diff exit code 2 swallowed](004-diff-exit-code-swallowed.md), [CLIDesign.md](../docs/CLIDesign.md)

---

## 1. Problem & Context

Issue 004 established correct error propagation in `internal/markdown` and `internal/claude` so that command execution failures (e.g. exit code ≥ 2 or missing `diff` utility) return errors instead of falsely claiming clean status.

However, standard execution of `harnez diff` always terminates with exit code `0` when differences are detected and printed to stdout. While this is standard interactive CLI behavior (consistent with default `git diff`), automated workflows such as CI checks, pre-commit hooks, and drift-detection scripts (e.g. `scripts/smoke-test.sh`) require a programmatic signal when files have drifted from declared harness configuration without having to parse stdout text.

## 2. Proposed CLI Contract

Add the `--exit-code` (`-e`) flag to `harnez diff`:

| Invocation | Drift Present | Execution Error | Exit Code | Stdout Output |
|---|---|---|---|---|
| `harnez diff` | No | No | `0` | `"No changes."` |
| `harnez diff` | Yes | No | `0` | Unified diffs |
| `harnez diff` | Any | Yes (e.g. missing binary, bad YAML) | `1` (Cobra) | Error message on stderr |
| `harnez diff --exit-code` | No | No | `0` | `"No changes."` |
| `harnez diff --exit-code` | Yes | No | `1` | Unified diffs |
| `harnez diff --exit-code` | Any | Yes (e.g. missing binary, bad YAML) | `1` (Cobra) / `>1` | Error message on stderr |

### Design Notes & Invariants
- **Clean exit on drift**: When `--exit-code` is active and differences are printed, the process terminates with status `1` directly via `os.Exit(1)` after diffs are rendered, without printing extraneous Cobra error prefixes to stderr.
- **Backwards compatibility**: Default invocation without `-e`/`--exit-code` retains exit status `0`.
- **Flag short option**: `-e` does not conflict with any existing flag on `diff` (which only uses `-c` for `--config` and `-t` for `--target`).

---

## 3. Technical Changes & Exact Line References

### A. `internal/claude/apply.go`
Update [`DiffAll`](../internal/claude/apply.go) to return `(bool, error)`:

```go
// DiffAll diffs the config and reports whether changes/drift were detected.
func DiffAll(target string, cfg *Config) (bool, error) {
	anyChanged := false
	report := func(changed bool, err error) error {
		if err != nil {
			return err
		}
		if changed {
			anyChanged = true
		}
		return nil
	}

	if err := report(diffSettingsJSON(filepath.Join(target, "settings.json"), buildSettingsDoc(cfg))); err != nil {
		return false, fmt.Errorf("settings: %w", err)
	}
	if g := cfg.AgentsMD.Global; len(g.Sections) > 0 {
		ruleTargets := []string{fsutil.ExpandHome(g.Target)}
		if root := primeAgentRoot(cfg); root != "" {
			ruleTargets = appendUniquePath(ruleTargets, filepath.Join(root, "AGENTS.md"))
		}
		for _, ruleTarget := range ruleTargets {
			for _, s := range g.Sections {
				if err := report(diffSectionMD(ruleTarget, s.Name, s.Content)); err != nil {
					return false, fmt.Errorf("agents_md.global %s [%s]: %w", ruleTarget, s.Name, err)
				}
			}
		}
	}
	if l := cfg.AgentsMD.Local; len(l.Sections) > 0 {
		for _, s := range l.Sections {
			if err := report(diffSectionMD(l.Target, s.Name, s.Content)); err != nil {
				return false, fmt.Errorf("agents_md.local [%s]: %w", s.Name, err)
			}
		}
	}

	if !anyChanged {
		fmt.Println("No changes.")
	}
	return anyChanged, nil
}
```

### B. `cmd/harnez/main.go`
Update [`diff` command definition](../cmd/harnez/main.go):

```go
	var diffExitCode bool
	diff := &cobra.Command{
		Use:   "diff",
		Short: "Show what apply would change in managed blocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := claude.OpenConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			t := claude.ExpandTarget(target, cfg.TargetDir)
			changed, err := claude.DiffAll(t, cfg)
			if err != nil {
				return err
			}
			if diffExitCode && changed {
				os.Exit(1)
			}
			return nil
		},
	}
	diff.Flags().StringVarP(&configPath, "config", "c", "", "path to config YAML file (default: embedded)")
	diff.Flags().StringVarP(&target, "target", "t", "", "Claude config directory (default: ~/.claude)")
	diff.Flags().BoolVarP(&diffExitCode, "exit-code", "e", false, "exit with status 1 if drift/changes are found")
```

---

## 4. Test & Verification Plan

### A. Go Integration Tests ([`internal/claude/integration_test.go`](../internal/claude/integration_test.go))
1. **Aligned State (Step 5)**:
   - Verify `changed, err := claude.DiffAll(targetDir, cfg)` returns `changed == false` and `err == nil`.
2. **Drifted State (Step 7)**:
   - Verify `changed, err := claude.DiffAll(targetDir, cfg)` returns `changed == true` and `err == nil`.
3. **Execution Failure ([`TestDiffAll_ExecError`](../internal/claude/integration_test.go))**:
   - Update call signature to `_, err = claude.DiffAll(targetDir, cfg)` and verify `err != nil`.

### B. Smoke Test Script Integration ([`scripts/smoke-test.sh`](../scripts/smoke-test.sh))
Add drift detection assertion steps following project Bash style (no `;`, newline before `then`):

```bash
echo ""
echo "=== simulate drift: drop git permission ==="
scripts/drop-perm.sh "Bash.git"

echo ""
echo "=== diff under drift: assert exit code 1 with --exit-code ==="
if "$bin" diff --exit-code >/dev/null 2>&1
then
    fail "diff --exit-code exited 0 despite drift"
else
    pass "diff --exit-code returned non-zero on drift"
fi

echo ""
echo "=== diff under drift: assert exit code 0 without flag ==="
if "$bin" diff >/dev/null 2>&1
then
    pass "diff without flag exited 0 under drift"
else
    fail "diff without flag returned non-zero under drift"
fi

echo ""
echo "=== apply after drift: must restore ==="
...

echo ""
echo "=== diff after repair: assert exit code 0 with --exit-code ==="
if "$bin" diff --exit-code >/dev/null 2>&1
then
    pass "diff --exit-code exited 0 after repair"
else
    fail "diff --exit-code exited non-zero after repair"
fi
```


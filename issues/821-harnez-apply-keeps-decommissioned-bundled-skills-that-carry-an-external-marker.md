# 821 — harnez apply keeps decommissioned bundled skills that carry an external marker

**Status**: Closed — Decommissioning removes harnez-owned bundled and unmarked skills across all five apply roots while preserving user registry installs; verified by regression tests and required checks.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: d77fb771 (removed the mattpocock bundles); harnez.org issue 032

---

## 1. Problem

On 2026-10-18 the mattpocock skills were installed again under `~/.claude/skills` and
`~/.codex/skills`: `grilling`, `grill-with-docs` and `to-spec`. They were dated 2026-10-10,
four days after d77fb771 removed them. Each copy had a `.harnez-external` marker with
`bundled: true` and `url: https://github.com/mattpocock/skills`. Unmarked copies of
`domain-modeling` were also left in `~/.claude`, `~/.codex`, `~/.gemini` and
`~/.prime/agent`.

`config.yaml` lists all four names under `decommissioned.skills`, but
`removeDecommissionedArtifacts` (`internal/claude/apply.go`) skips any directory where
`externalSkillClash` matches. So a copy that harnez's own bundle installer wrote, and
marked `bundled: true`, survives every `harnez apply`. Something reinstalled them on
2026-10-10, probably an older harnez binary. Nothing cleaned that up afterwards.

The copies were removed by hand on 2026-10-18. `harnez apply` (v0.1.30) did not reinstall
them.

## 2. Fix

- Decommissioning should remove marked copies whose marker says `bundled: true`, because
  harnez owns those. It should keep skipping real registry installs (`harnez skill
  install`) that the user chose.
- Check why `domain-modeling` survived in the `.codex`, `.gemini` and `.prime` skill roots.
  It has no marker, so `skillTargets` may not cover every agent root.
- Add a test: a decommissioned name with a `bundled: true` marker is removed, and one with
  a user registry marker is kept.

## 3. Delivery

- Implementation: `1da39d7d` reuses `skillreg.IsBundled` during decommissioning;
  former bundles are removed while user registry installs remain protected.
- Root audit: `skillTargets` already covers Claude, Codex, Gemini, Prime and Pi,
  including Pi's environment override. Native skill writes use that helper, and
  bundled writes use the matching `SkillTargetsByAgent` roots. OpenCode receives
  a distill plugin, not skills; there is no missing apply skill root to add.
  Tests confirm unmarked copies are removed from all five configured roots.
  The historical leftovers cannot be attributed to a missing root in current code;
  the older binary/configuration that left them was not available for verification.
- Temp-home regression tests cover bundled, registry, explicitly non-bundled and
  unmarked copies in each root, preserved registry contents and cleanup idempotency.
  Before the fix, bundled removal failed in all five roots (0 removed, expected 5).
- Verification: `make lint`, `go test ./internal/claude/ ./internal/skillreg/ ./cmd/harnez/`,
  `make test-short` and `make install` passed. The first short-suite run hit an
  unrelated `TestCodexOverThresholdResumeUsesProviderAutoCompact` TempDir cleanup
  failure (`directory not empty`); the complete short suite passed on rerun.

# 821 — harnez apply keeps decommissioned bundled skills that carry an external marker

**Status**: Open
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

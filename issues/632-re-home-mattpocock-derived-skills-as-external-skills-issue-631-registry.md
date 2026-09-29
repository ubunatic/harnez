# 632 — Re-home mattpocock-derived skills as external skills (issue 631 registry)

**Status**: Closed
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [[631-external-skills-canary-then-user-registry-b-then-vendored-skills-a]]

---

/goal Decide per skill whether the mattpocock-derived workflows become external skills (631
registry, pinned upstream) or stay harnez-owned adaptations, apply it, and remove the manual
drift checker; or stop and report when blocked on a user decision or on 631 not being built yet.

## 1. Problem & Motivation
Several workflows were hand-adapted from https://github.com/mattpocock/skills and are kept in
sync manually. Commit dd78359d moved them to `docs/proposed/` and `config.yaml`
`decommissioned.commands` removes them from agents, so today they are neither installed nor
tracked against upstream. Once 631's external registry exists, they fit its model better.

## 2. Findings (2026-09-29)
- Parked in `docs/proposed/`: `domain-modeling.md` (from upstream domain-modeling + CONTEXT-FORMAT
  + ADR-FORMAT), `grilling.md`, `grill-with-docs.md`, `make-spec.md` (from `to-prd`, has an
  `upstream:` header), and `sync-mattpocock-skills.md` (a manual drift checker listing the upstream URLs).
- Local changes to upstream: target `issues/` instead of an external tracker; drop references to
  `setup-matt-pocock-skills` and `to-issues`. External install would lose these unless kept as a
  thin local overlay, which is the main open question.
- No other third-party skills found installed: `~/.claude/skills`, `~/.codex/skills`,
  `~/.gemini/skills`, `~/.prime/agent/skills` hold only harnez-managed skills. `~/.claude/skills/synced`
  is Claude's own synced-skills bucket, not harnez; out of scope.

## 3. Decision (2026-09-29)
Install upstream unmodified as external skills, pinned. The harnez deltas (tickets go to
`issues/`) are already stated by repo rules (`docs/IssueTracking.md`, AGENTS.md), which override
skill defaults, so no overlay is needed. `sync-mattpocock-skills.md` is replaced by
`harnez skill update`.

## 4. Plan
1. Wait for 631 step B (`harnez skill install`).
2. Per skill: install upstream pinned as external, or keep the harnez adaptation; record the choice.
3. If external: delete the `docs/proposed/` copy and `sync-mattpocock-skills.md` (the registry's
   pinned ref plus an update check replaces it). Verify the agents list the skills.

## 5. Result (2026-09-29)
Installed pinned at upstream 272f99b (2026-07-04, when we adapted them), then ran the update
path to c55ee46 (2026-09-18): `grilling`, `grill-with-docs`, `domain-modeling` updated;
`to-prd` (our `make-spec`) reported gone, candidates `to-spec, to-tickets, ...`; replaced with
`to-spec`. All explicit-only. Deleted the `docs/proposed/` copies and `sync-mattpocock-skills.md`.
`update` gained diff summaries, moved/gone detection, rename hints, new-skill listing,
`--latest`, `--dry-run`, `--diff`. Fixed: install failed when `/tmp` is another filesystem;
decommissioned skill names no longer reserved.

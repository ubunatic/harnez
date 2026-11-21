# 1255 — issues index skips the issue indexes under tracker: none while lint and status verbs regenerate them

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 148, 615, 232

---

## 1. Problem & Motivation
Reported 2026-11-21 from the webman session (harnez 0.5.10+5413). webman's `.harnez/repo.yaml` sets `issues.tracker: none`, but `issues/` holds numbered tickets and a generated `issues/README.md`, so the issue verbs treat it as the local tracker.

After a new ticket file is added by hand:
- `harnez issues lint` fails: `issues/README.md is generated index drift; run 'harnez index'`.
- `harnez index` prints only `docs/README.md up to date`, and `harnez index --check` exits 0. Per its help, it runs issue indexes only with `issues.tracker: harnez`.
- `harnez issues open <n>` (any status verb) does regenerate `issues/README.md`.

The lint error tells the agent to run a command that does nothing in this setup. It cost about five retries over two tickets (050, 051) before a status verb was used as the workaround.

## 2. Technical Specification / Findings
- `harnez index` decides with `issues.tracker: harnez`. The issue verbs and lint use the tracker detection that also accepts a local `issues/` under `tracker: none` (numbered files, `archive/` or `issues/README.md`).
- Unverified: whether webman should simply set `tracker: harnez`. Either way, the two commands should not disagree.

## 3. Implementation & Verification Plan
/goal `harnez index` and `harnez issues lint` agree on when the issue indexes exist, so following lint's advice clears the drift. Stop and report if `tracker: none` with a local `issues/` is meant to be unsupported; then lint should say that and name the config fix instead.

- Use one tracker detection for `index`, `index --check`, lint and the status verbs, or make lint's message name a command that works.
- Test: a temp repo with `tracker: none`, numbered tickets and a stale `issues/README.md`. `harnez index` regenerates it, `--check` exits 1 before, and lint passes after.

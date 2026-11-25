# 1294 — per-repo push route in .harnez/repo.yaml; generic push wording: main if possible, AGit if protected, else branches

**Status**: Closed — implemented in 4585067a: git.push_route and git.agit_topic, generic push wording
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: issues/1293-harnez-land-description-in-config-yaml-still-names-the-old-push-order.md, docs/lang/Git.md, docs/commands/HarnezLand.md

---

## 0. Goal
`/goal` Let a repo declare its push route in `.harnez/repo.yaml`, and make the generic push wording say: push to `main` if possible; use AGit if `main` is protected and the forge supports it; otherwise use branches and PRs. Done when the rendered Repo Setup and the bundled Git and push docs match the chosen route, and `make test-q1` is green.

## 1. Problem & Motivation
The bundled `docs/Git.md` hard-codes "team, AGit by default". A repo cannot change that without editing a managed file, and the generic wording pushes agents toward AGit even when `main` accepts a direct push. Reported by lmcoder (a peer session, relayed by the owner as the owner's request).

## 2. Technical Specification / Findings
- Keys live in `spec/schemas/repo.schema.json` (`git` object), defaults in `spec/git.yaml`, and render keys in `spec/repo-config.yaml`.
- The team text is selected by `git.mode` and `protected_main` through `repo_modes` in `config.yaml` (`team`, `team_protected`).
- `internal/gitflow` loads `git.*`; `internal/repocfg/get.go` serves `harnez config`.
- Generic push wording: `docs/lang/Git.md` (copyable source, root copy `docs/Git.md`), `config.yaml` agents_md Tools rules text (~line 855).

## 3. Implementation & Verification Plan
Design (decided in decide mode):
1. Add `git.push_route` with enum `main-first` (default: main when allowed, else AGit, else branch and PR), `agit-only` (AGit, else branch and PR; never a direct push to main), and `branch-only` (topic branch and PR only).
2. Add optional `git.agit_topic` (string). When unset, the route falls back to `git config remote.origin.push`, then the forge username, as `docs/commands/HarnezLand.md` already says.
3. The route only changes rendered guidance (repo modes and the push order in Repo Setup). Enforcement stays with the pre-push guard and `protected_main`; no new hook.
4. Add `git.push_route` to `render_keys` so a change prints `run: harnez init -d <repo>`.
5. Change the generic wording in `docs/lang/Git.md` (and the `docs/Git.md` copy) and the Tools push text to: "push to main if possible; use AGit if main is protected and the forge supports it; otherwise use branches and PRs".
6. Verify: schema and `harnez config` tests for the new keys; golden tests for each route; `harnez init -d .` leaves no drift; `HTO=0 make test-q1` green.

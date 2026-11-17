# 1237 — Forge-tracker friction: duplicate search blocked, issues next fails, labels not applied on fj

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: 194 (local next-number command), 1138 (forge 404 hint), 1195 (forge create via fj/tea/gh), 1228 (fj wrapper host)

---

## 1. Problem & Motivation

Filing a ticket for taia (tracker `none`, tickets on Codeberg) hit three harnez friction points. Each one blocked or degraded a required step of the issue skill.

1. **Duplicate search is blocked for private forge repos.** `harnez find -d . issues "<terms>"` on a private repo fails with `HTTP 404 ... a private repo needs a token: run harnez forge token import or harnez forge token create`. The issue skill forbids debugging token setup, so an agent cannot run the required duplicate search and has no clear status to report. `fj` was logged in and could create issues on the same repo, but `harnez find` did not use that login.
2. **`harnez find -d <repo> issues next` fails on forge trackers.** With `issues.tracker: none` it errors with `needs a local issues/ dir`. There is no way to get the next free number for a forge-tracked repo.
3. **Inbox fallback does not say that labels are lost.** `harnez issues new "<title>" --label enhancement` with no forge CLI installed warned `no forge CLI found ... install one of tea, fj (fj cannot set labels)` and wrote `issues/inbox/<slug>.md`. The draft kept the label (`**Labels**: enhancement`). The message does not say the label will not be applied when the draft is later filed through fj.

## 2. Technical Specification / Findings

- Observed on 2026-11-17 in taia (`issues.tracker: none`, forge `codeberg.org/harnez/taia`).
- Duplicate search: `harnez find -d . issues "fj"` in this repo works because it is local. The failing case is the private taia forge repo with no harnez read token (`404`, token hint).
- `issues next`: same error text as above, reproduced in taia.
- Fallback: the first `harnez issues new` in taia, with no `fj` or `tea` installed, wrote the inbox draft. A later run with `fj` installed filed #295 through `fj issue create` (dry run showed no label flag).
- Not verified: whether `fj` actually drops labels. The note in harnez's own warning says `fj cannot set labels`.
- Related but distinct: 1138 covers the 404 hint text; 1195 covers forge creation; 1228 covers a fj wrapper; 194 covers the local next-number command.

## 3. Implementation & Verification Plan

- `harnez find issues` on a private forge repo: either use the configured forge login (`fj`) when present, or return a status an agent can report without debugging tokens.
- `harnez find issues next` for forge trackers: support the next-number lookup, or report the forge limitation with a non-error status.
- Inbox fallback: when a label cannot be applied on the target CLI, say so in the fallback warning and in the draft.
- Verify with a private forge repo that has no `harnez` read token but a working `fj` login: `find issues` and `find issues next` both return results or a clear status, and `issues new --label` with `fj` prints a warning that labels are not applied.

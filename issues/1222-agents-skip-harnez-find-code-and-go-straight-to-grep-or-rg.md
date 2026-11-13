# 1222 — Agents skip harnez find code and go straight to grep or rg

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 1221

---

## 1. Problem & Motivation

`.harnez/rules/Tools.md` says: before broad shell searches use `harnez find code|docs`, then
`rg`, and say which fallback you used. In the rule-following sessions of 1221, one session in
nine did that (`luna:high`). The other eight used `grep -rn`, `git grep` or `rg` directly, on
the old docs and on the trimmed ones. Five of them named the skipped rule themselves when asked
afterwards, so they had read it.

A rule that is read and not followed costs context in every session and teaches agents that the
rules are optional.

## 2. Technical Specification / Findings

- Task in the sessions: "find where the Go code decides between the lite and the full doc
  variant". Agents judged this a targeted search and reached for the tool they know.
- `luna:low` ran `harnez find docs` but `rg` for code.
- Not measured: whether `harnez find code` gives a better or cheaper answer than `rg` for such a
  question. Without that, the rule has no stated benefit.

## 3. Implementation & Verification Plan

1. Measure first: run the same three code questions through `harnez find code` and `rg` and
   compare turns, output bytes and correctness.
2. If `harnez find code` wins: say when it applies ("symbol or concept lookups") and what it
   saves, in one line of `Tools.md`; consider a hook hint on a first broad `grep -r`/`rg`.
3. If it does not win: drop the rule from `Tools.md` and keep it for docs only.
4. Verify with the 1221 task on `haiku:low` and `luna:low`, three runs each.

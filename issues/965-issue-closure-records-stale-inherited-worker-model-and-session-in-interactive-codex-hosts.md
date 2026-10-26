# 965 — Issue closure records stale inherited worker model and session in interactive Codex hosts

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: #953, #915; [OrchestratedAgentFlow](../docs/OrchestratedAgentFlow.md)

---

## Goal

Record the actual closing host's known identity, preserving explicit flag
precedence and genuine worker attribution. Verify regression cases, or stop and
report when blocked on a user decision, denied permission or lack of progress.

## 1. Problem & Motivation

After the interactive-host fix in #953, closing that ticket still produced
`**Closed-By**: codex:luna:low, session c943`. Those values were inherited from
an unrelated worker; the verified interactive host thread was
`01a17932-4b30-70c1-a470-248960b5b8b4`. This makes the historical attribution
misleading even though host dispatch now works.

## 2. Technical Specification / Findings
Observed on the live closure recorded by commit `4b2f3f6e`. The host environment
contained `HARNEZ_AGENT_MODEL=codex:luna:low` and `HARNEZ_SESSION_ID=c943`.
#915 deliberately introduced those environment defaults; #953 corrected caller
identity for agent operations but did not change issue-closure attribution.
Do not infer the current model from the stale environment. Unknown identity
parts should remain omitted rather than being guessed.

## 3. Implementation & Verification Plan
Re-verify current closure behavior before implementation. Resolve trustworthy
interactive-host identity while retaining `--model`/`--session` precedence,
worker defaults and omission of unknown fields. Cover stale interactive
environment, genuine workers and explicit overrides; verify through the real
close command in an isolated tracker. Decide whether to correct #953's historical
line explicitly, without reopening it or rewriting history.

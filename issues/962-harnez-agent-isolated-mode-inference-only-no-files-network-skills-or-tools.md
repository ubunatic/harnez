# 962 — harnez agent isolated mode: inference only, no files, network, skills or tools

**Status**: Backlog — grooming 1083 (2026-11-03): active list focused on taia support, P1s and open bugs; promote when needed
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: tokburn 001 (T6 AI deciders are blocked on this)

---

## 1. Problem & Motivation

tokburn lets AI models play a game by sending them a batch of task text and reading back one tier
per task. The models must not see the answers, which are in the repo and the binary. Today
`harnez agent` runs every provider with permissions bypassed: `codexRunArgs`
(`internal/subagent/codex.go`) passes `--dangerously-bypass-approvals-and-sandbox`, and
`claudeRunArgs` (`internal/subagent/claude.go`) passes `--dangerously-skip-permissions`. No flag
turns off file, network, skill or tool access, so a prompt can only ask a model to behave.
tokburn uses an empty `-d` temp dir as a stopgap for golden-tier rating (T4), which is not enough for
scored AI players (T6).

/goal `harnez agent` offers an isolated mode (e.g. `--isolated`) that runs a prompt with inference
only (no file, network, skill, MCP or tool access) for codex, claude and agy, enforced by each
provider's own controls, and fails loudly for a provider that cannot enforce it; or stop and report
when blocked on an owner decision or a provider limit. Circuit breaker: on a loop or no progress,
clear the goal and write a failure report here.

## 2. Technical Specification / Findings

- Re-check live code and provider CLIs first; flags change often.
- Per provider, find the strongest enforced option (sandbox mode, tool allowlist set to none,
  disabled MCP/skills, no network) and document what each one actually blocks.
- Open: whether a provider without enforced isolation should be refused or allowed with a warning.

## 3. Implementation & Verification Plan

A canary per provider: an isolated prompt asking the model to read a file, list a directory and
fetch a URL must fail each attempt and still return a text answer.

## Audit 2026-11-11

Jules audit (PR GH#179, report `reports/jules/962.md`, checked against 703ce7ff, 3 of 3 references held).
Still valid: every driver starts with permissions skipped, codex with
`--dangerously-bypass-approvals-and-sandbox` (`internal/subagent/codex.go:125`), claude with
`--dangerously-skip-permissions` (`internal/subagent/claude.go:55`), agy with
`--dangerously-skip-permissions --sandbox` (`internal/subagent/agy.go:37`), and no isolated mode exists
(the only "isolated" hit outside tests is a process-group comment in `internal/subagent/watchdog.go:22`).
Related: 1216 (tool-only mode) also needs a per-model switch on these same run-arg builders.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| audit | jules-audit-962 | 8313455151916030535 | github #179 (closed) | done: still valid, 3/3 refs hold |

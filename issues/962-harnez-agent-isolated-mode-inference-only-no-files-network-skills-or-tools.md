# 962 — harnez agent isolated mode: inference only, no files, network, skills or tools

**Status**: Open
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

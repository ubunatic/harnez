# 548 — Require direct harnez agent commands

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: None

---

## 1. Problem & Motivation

Agents may invoke `harnez agent` through heredoc or Python wrappers. Those wrappers obscure the actual command, make it harder for people to review, and prevent `harnez` from assessing the invocation itself.

## 2. Goal

Ensure agents invoke sessions directly with `harnez agent`, without heredoc or Python wrappers obscuring the command.

## 3. Implementation & Verification Plan

Update agent instructions or launch tooling as appropriate, and verify the supported invocation path is directly visible as `harnez agent` to both reviewers and `harnez` assessment.

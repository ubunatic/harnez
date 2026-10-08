# 763 — Study DeepSeek Harness vs Pi comparison video and adopt useful coding-agent harness practices

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: 302 (plugin systems for pi and DeepSeek Harness), 473 (MiniMax CLI harness study); video https://www.youtube.com/watch?v=gGqi-wcyHc4

---

## Problem & Motivation

A YouTube video compares the DeepSeek Harness and Pi as free coding agents. The user wants to
know how these harnesses work and which of their practices Harnez should lean on or adopt.
Only the page title was retrievable when this ticket was filed ("DeepSeek Harness vs Pi: Which
Free Coding Agent Wins"); the transcript, speaker and claims are unknown. Claims in the video are
not evidence until checked against the harnesses themselves.

## Goal

Produce an evidence-based comparison of the DeepSeek Harness and Pi and decide, per practice,
whether Harnez adopts it. The work is done when each practice has a fit decision (adopt, adapt or
reject) with a one-line rationale, and each adopted practice has a follow-up ticket or a landed,
tested change. Stop and report when blocked on a user decision or a denied permission, such as
access to a harness that needs an account or paid key.

## Milestones

- M1: Obtain the video transcript or chapter notes; if the transcript is unavailable, record that and
  use the harnesses' own docs and source instead.
- M2: Inspect how the DeepSeek Harness and Pi handle sessions, tools, plugins, instruction files and
  headless runs; note which Harnez already covers (see 302, 473 and `docs/AgenticLoop.md`).
- M3: Write the fit decisions and file follow-up tickets for adopted practices.

## Findings to Validate

- Which claims in the video hold when checked against the harnesses' current docs or a small canary task.
- Whether the "free" cost claim holds for each harness, including model and token cost per task.
- Which task-loop, context-handling or tool-permission practices are transferable to Harnez.

## Implementation & Verification Plan

Check each claim against primary sources before adopting it. Adopt only practices that fit
Harnez's spec-driven design (see `docs/Spec.md`). Verify any code change with focused tests and
`make install`. Plugin-system questions stay in 302, not here.

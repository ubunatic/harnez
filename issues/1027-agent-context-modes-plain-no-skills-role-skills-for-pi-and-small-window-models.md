# 1027 — Agent context modes: plain, no-skills, role-skills for Pi and small-window models

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1023, 1024, 1026, 486, 348, lmcoder 263

---

## 1. Problem & Motivation
A simple review on `lmcoder:coder` (1024) filled its 32k window and looped "read, compact, forget, read again"
for 14 minutes (13 compactions, lmcoder 263). Part of every prompt is fixed overhead: Pi's system prompt, the
role preamble, rules, skills and tool descriptions that a simple task does not need. Smaller context leaves more
room for the task itself, on local models and on paid ones.

Owner's request: for simple reviews, start the Pi agent in a "plain" mode, without skills and rules. More
generally, a context mode chooses what the agent gets, so it has only the tools, skills and rules the task needs
and everything else is off.

## 2. Modes (owner's draft, to refine)
- `default`: today's behavior.
- no skills/MCP: rules and tools, but no skills or MCP servers.
- `plain`: no skills, no rules, only the tools the task needs (for a review: read, grep, git).
- `<role>-skills`: only the skills that belong to the role (e.g. reviewer).
- More modes as needed ("etc.").

## 3. Open questions
- What Pi can switch off per run, verified on Pi 1.4.0: skills, extensions (`--no-extensions` does not disable
  `-e` ones, per 1023), context files, built-in tools; and what harnez itself adds (role preamble, rules).
- How a mode is selected: a `--mode` flag on `harnez agent start`, a per-role default in `spec/agent.yaml`, or a
  default by model window. Mode names and their contents go in `spec/` with a schema, not Go defaults.
- Which providers it covers: Pi first; Claude and Codex later if the same switches exist.

## 4. Goal
/goal Add selectable context modes for agent runs, starting with Pi, so a simple task gets only the tools, skills
and rules it needs. Done when `plain` runs exist, the fixed prompt size per mode is measured on Pi (tokens before
the user prompt), and the 1024 review job on `lmcoder:coder` is rerun in `plain` mode with its compaction count
recorded. Stop and report when a mode needs an owner decision (names, defaults per role) or Pi cannot switch a
part off. Circuit breaker: on a loop or no progress, clear the goal and write a failure report.
Re-check live code and Pi's version before starting; this ticket was written on 2026-10-29.

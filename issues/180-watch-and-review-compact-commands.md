# 180 — Harnez Should Watch `/compact` Events and Assess Before/After Quality

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [issues/178](178-distill-smart-mode-error-pattern-preservation.md) (output distillation, adjacent context-loss concern)

---

## 1. Problem & Motivation

`/compact` (context summarization) is one of the biggest single points of information loss in a
long agent session — a bad compaction can silently drop the one detail (a file path, an exact
error string, a decision rationale) that made the rest of the session coherent. Today harnez has
no visibility into when compaction happens or whether it preserved what mattered. Agents and users
only discover a bad compact after the fact, when the agent starts asking questions it should
already know the answer to.

## 2. Technical Specification / Findings

Needs research before implementation — this is filed as a feature ticket with an open design, not
a spec-complete one. Open questions:

- What signal is available that a `/compact` happened? For Claude Code, `/compact` runs in the
  harness itself, not as a tool call harnez can hook (unlike `harnez rate` after tool calls). May
  need to look at session transcript files/hooks (e.g. `PreCompact`/`PostCompact`-style hook
  points if the host exposes them) rather than active polling.
- "Before" state: capture (or reference) the transcript content immediately prior to compaction.
  "After" state: the generated summary. A useful assessment needs both.
- What does "assessment" mean concretely? Candidate cheap heuristics: byte/line reduction ratio,
  whether specific proper nouns/file paths/identifiers present pre-compact still appear
  post-compact, whether open TODOs or pending-task markers survived.
- Where does the assessment surface? Likely a `harnez rate`-style log entry or a note surfaced via
  the "session state" mechanism proposed in issue 183, rather than a new blocking UI.

## 3. Implementation & Verification Plan

1. Research harness hook points (Claude Code hooks config, Codex equivalents) for pre/post-compact
   signals; document findings before writing code.
2. Prototype a lightweight diff/heuristic comparator (no LLM call required for v1) that flags
   likely information loss (e.g. an identifier mentioned N times pre-compact and 0 times
   post-compact).
3. Log the assessment through harnez's existing event/telemetry path; verify with a synthetic
   before/after pair.
4. If a host-native hook isn't available, document that constraint and downgrade scope to a
   manual/opt-in `harnez compact-check <before> <after>` command instead of automatic watching.

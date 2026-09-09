# 285 — Agent must not claim it "noted" something without writing it to a durable file

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Bug (silent-correctness risk / trust)
**Category**: Agentic Ergonomics
**Related**: [issues/185](185-instruct-agents-to-submit-feedback-on-bugs-and-bad-instructions.md) (precedent: narrow, event-triggered instruction wired into `config.yaml`'s `agents_md.global.sections`), [docs/practices/AgenticLoop.md §1.5 In-Repository Single Source of Truth](../docs/practices/AgenticLoop.md)

---

## 1. Problem & Motivation

Live incident, 2026-09-08, `voxi` project (Claude Code session): the user
suggested a future feature ("we can later add a detailed view that always
shows loudness even when we're not recording"). The agent replied "Sounds
good — noted as a future enhancement, no action needed now." — but never
wrote that anywhere: not to a ticket, not to memory, not to any file.
When the user asked "'noted' where?", the agent had to admit nothing was
persisted.

This is a correctness gap in the same family `docs/practices/AgenticLoop.md`
§1.5 already names for session context/learnings ("must be committed to
the repository rather than abandoned in ephemeral agent chat contexts"),
but that existing instruction is scoped to end-of-session retrospectives
and friction logs — it doesn't cover the much more common, smaller case of
an agent using conversational acknowledgment language ("noted", "got it",
"I'll keep that in mind", "duly noted") mid-conversation as if it were a
durable action, when it is actually just an unpersisted chat reply that
evaporates the moment the session ends or context is compacted.

## 2. Technical Specification / Findings

- Add a narrow, always-applicable instruction (candidate home: a new
  `config.yaml` `agents_md.global.sections` entry, mirroring issue 185's
  mechanism, or a tightened addition to the existing §1.5 bullet in
  `docs/practices/AgenticLoop.md`) stating: an agent must never say it
  "noted"/"logged"/"will remember" something unless it actually wrote
  that thing to a durable file in the same turn — a ticket, a memory
  file, a TODO/tracking doc already in the repo. If the agent has not
  (yet) persisted it, it must say so plainly and, where a tracker exists
  (harnez-tracked project, `issues/` present), offer or ask to file it —
  not claim the acknowledgment as if it were storage.
- Scope this to the *language* the agent uses, not to a blanket
  "always file everything the user says" mandate — issue 181's
  hard-won lesson (narrow, event-triggered, not a chatty blanket rule)
  applies here just as it did to issue 185. The trigger is specifically:
  agent output contains an acknowledgment-of-persistence phrase with no
  corresponding file write in that turn.
- Two sub-cases worth distinguishing during implementation:
  1. Harnez-tracked project (`issues/` present): the instruction should
     point at filing a ticket (`harnez issues new`) or writing to memory,
     whichever fits the content (see `docs/practices/AgenticLoop.md`
     memory-vs-ticket distinction already documented elsewhere in this
     project's guidance).
  2. Non-harnez project or content that isn't ticket-shaped (e.g. a
     preference, not a task): should fall back to the agent's memory
     system if one is configured, or otherwise say explicitly "I have
     nothing to persist this in right now" instead of implying it did.

## 3. Implementation & Verification Plan

1. Draft the instruction text; keep it short per issue 181/185's
   word-budget discipline (~50-70 words), and word it as an event
   trigger ("before using 'noted'/'logged'/'I'll remember' language,
   either write the file first or say you haven't"), not a blanket
   "always file things" mandate.
2. Add it to `config.yaml`'s `agents_md.global.sections` (new section) or
   fold into the existing §1.5 bullet in
   `docs/practices/AgenticLoop.md` — decide during implementation which
   surface fits better; a new standalone section is likely clearer since
   this is about a specific phrase pattern, not the broader in-repo
   single-source-of-truth principle §1.5 already covers.
3. Verify with `go test ./...`; run `make apply` to confirm the live
   `~/.claude/CLAUDE.md` reflects the new instruction; update
   `issues/README.md`.
4. Live-verify in a real session: prompt an agent into a situation where
   it would naturally say "noted", and confirm it now either writes the
   file in the same turn or explicitly declines to claim persistence —
   a `go test` pass alone would not catch this (it's a prompt-content
   change, not code logic).

## 4. Non-Goals (for now)

- Not building detection/enforcement logic that scans agent output for
  the trigger phrase and blocks/flags it programmatically — this is an
  instruction-text change, matching how issue 185 shipped as a static
  instruction rather than new code.
- Not attempting to retroactively audit past sessions for this pattern.

# 293 — Make roadmap synthesis recoverable across quota interruptions

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [234 — Add a /roadmap skill](234-add-a-roadmap-skill-product-manager-style-roadmap-creation-update-from-open-issues.md),
`commands/roadmap.md`, `docs/Roadmap.md`, `docs/AgenticLoop.md` (context discipline and durable
handoff guidance)

---

## 1. Problem & Motivation

The roadmap skill can spend a substantial amount of time reading the open backlog, extracting
implementation-plan details, identifying dependencies, and reconciling those findings with an
existing roadmap before it writes anything. A real run completed much of that useful discovery,
but then hit a quota/interruption during synthesis. Because neither intermediate findings nor the
reconciliation state had been written durably, a later agent could not recover the work: it had
to reread the backlog and reconstruct the same judgments from scratch.

This is especially costly for `/roadmap`, where the input may be dozens of tickets and the value
comes from cross-ticket synthesis rather than any one read. The current instruction to keep the
host responsive does not solve loss of the dispatched agent's in-memory work.

The skill needs bounded, durable progress checkpoints during long runs so a replacement agent can
resume from useful state after quota exhaustion, context loss, process termination, or another
interruption. This must not weaken the skill's core safety boundary: the issue tracker remains
strictly read-only, and the roadmap remains the only durable final product of a successful run.

## 2. Required Design

Update the roadmap workflow to checkpoint at meaningful synthesis boundaries, including during a
large issue-reading pass rather than only immediately before final output. The implementation may
use a durable handoff artifact or another explicitly allowed recovery location, but it must define
the location and lifecycle unambiguously. A recovery artifact must never be placed in or modify
`issues/`, must not masquerade as the completed roadmap, and must be written atomically enough that
an interruption cannot leave a plausible but truncated checkpoint.

Each checkpoint must contain enough compact state for a fresh agent to continue without replaying
the whole run:

- target repository and intended roadmap path, plus a freshness marker suitable for detecting a
  changed backlog or roadmap;
- the issue inventory discovered for the run and which tickets have already been read versus
  remain unread;
- condensed findings needed for synthesis, including scope, dependencies, blockers, and product-
  value signal from the tickets already processed;
- the existing-roadmap reconciliation in progress: retained framing, shipped/closed items,
  proposed moves, conflicts, and decisions already made with their rationale;
- the derived product value axis, current theme/bucket model, unresolved decisions, and an
  explicit remaining-work list.

Checkpoint frequency should be bounded (for example, after a small batch of issues and at each
workflow-phase boundary) so failure near the end of a large read does not discard nearly all of
the work. Keep checkpoint content concise and resumable; this is not permission to copy every
ticket or dump an agent transcript.

On startup, the dispatched roadmapper should detect a compatible recovery artifact, validate it
against current repository state, and resume from it. If the backlog or roadmap changed, it should
identify and refresh the affected state rather than silently trusting stale conclusions. If the
artifact is unusable, report why and start clean.

The design must preserve `commands/roadmap.md`'s write constraint. It may explicitly authorize one
temporary recovery-note location outside the tracker, solely for this workflow. That note persists
when a run is interrupted so another agent can recover it, but is removed after the roadmap has
been written and verified successfully. No ticket, `issues/README.md`, tracker metadata, or source
code may be edited by a roadmap run.

## 3. Acceptance Criteria

- [ ] `commands/roadmap.md` defines bounded checkpoints during both issue ingestion and roadmap
      reconciliation/synthesis, not merely a final pre-write note.
- [ ] The skill names an allowed recovery-artifact location and documents its atomic-write,
      ownership, freshness-validation, resume, and successful-cleanup behavior.
- [ ] A checkpoint records the issue inventory, read/pending issues, compact per-issue findings,
      current roadmap reconciliation, decisions/rationale, derived value axis, and remaining work.
- [ ] A fresh agent can resume a compatible interrupted run from the checkpoint without rereading
      every already-processed ticket or discarding completed reconciliation work.
- [ ] Changed inputs are detected on resume and selectively refreshed or cause a clearly reported
      clean restart; stale state is never accepted silently.
- [ ] The issue tracker stays read-only: the workflow never edits `issues/*.md` or
      `issues/README.md`, nor runs `harnez index` or a mutating `harnez issues` command.
- [ ] During an interrupted run, the explicitly allowed temporary recovery note is the only write
      besides a possibly pre-existing roadmap; after successful completion, the roadmap is the
      only resulting durable output and the temporary note is removed.
- [ ] The final report says whether the run started fresh or resumed, identifies any stale state
      refreshed, and confirms recovery-note cleanup after the roadmap write succeeds.

## 4. Verification Guidance

Exercise the skill against a fixture repository with enough open tickets to require multiple
checkpoint batches and an existing roadmap that needs reconciliation:

1. Interrupt the run after at least one issue batch and again during synthesis. After each forced
   stop, verify that the recovery artifact is complete, parseable, and contains all required
   resume fields while `git diff -- issues/ issues/README.md` remains empty.
2. Start a fresh agent against the unchanged fixture and verify from its read/tool trace that it
   consumes the checkpoint, skips already-completed ticket reads, completes the remaining work,
   writes the expected reconciled roadmap, and removes the recovery artifact.
3. Repeat after changing one issue and the existing roadmap between interruption and resume.
   Verify that the changed inputs are detected and refreshed while unaffected checkpoint work is
   retained, or that the run explicitly rejects the checkpoint and restarts cleanly.
4. Compare an interrupted-and-resumed result with an uninterrupted run over the same inputs. The
   roadmap need not be byte-identical, but it must cover the same backlog state, dependencies,
   moves, and sequencing rationale.
5. Run any skill-content/config tests plus the repository's normal checks, and confirm a successful
   run leaves no recovery artifact, tracker diff, or unrelated file modification.

## 5. Non-Goals

- Turning `/roadmap` into a writer for ticket status, priority, index, or implementation plans.
- Persisting full ticket copies, transcripts, or hidden chain-of-thought as checkpoint data.
- Treating a partial checkpoint as a published roadmap or retaining recovery notes indefinitely
  after a successful run.

<!-- harnez:topic: First live harnez-advisor Codex design-review call, and a third live recurrence of the opt-in-doc-drop bug -->

# First Live Codex Advisor Design Gate, and a Third `init` Opt-In-Doc-Drop Recurrence

**Scope**: The first end-to-end use of the `harnez-advisor` skill's Codex path
(issue 303) as an actual pre-implementation design gate on a real ticket
(issue 316), plus a third live reproduction of issue 315's `harnez init`
opt-in-doc-drop bug during unrelated doc-onboarding work.

**Accessed**: 2026-09-11

## Finding 1 — the Codex/Astra advisor design gate worked as designed

`codex exec -m gpt-6-astra -c model_reasoning_effort=low "<prompt>"` was run
read-only against issue 316's settings.json-debloat proposal. This is the
first time the `harnez-advisor` skill's Codex invocation guidance (added to
`docs/commands/HarnezAdvisor.md` in this same work stretch, commit `571d792`)
was exercised for its actual intended purpose — an external, independent
design review — rather than just documented in the abstract.

The review caught real issues the original ticket had only flagged as open
questions, and closed them with a decisive stance:
- **No-go as written**: the "full" default preset denies interaction/safety
  tools (`AskUserQuestion`, `ExitPlanMode`, `ReportFindings`,
  `ScheduleWakeup`) that are not pure token overhead.
- **Revert semantics were unsound**: reset-to-`false` on revert would clobber
  a pre-existing `true` the user had set before harnez ever touched the file
  — a real correctness bug, not a style nit, that the ticket's original
  author had not surfaced.
- **Struct model was incomplete**: round-tripping the whole settings file
  through the proposed `*bool` structs would silently drop unknown fields
  (`permissions.allow`, `ask`, etc.) — caught by the advisor actually reading
  `internal/claude/config.go`'s existing `Permissions{Allow, Deny}` type and
  noticing the proposed structs didn't reconcile with it.

The advisor's own attempt to call `harnez rate` (its side of the Tool
Feedback Protocol, injected into its system context via the shared harnez
docs pipeline) failed with `read-only file system` — its sandbox couldn't
write to `~/.harnez/sessions/`. Harmless (no repo state was touched), but
worth knowing: **a dispatched external advisor's own telemetry/feedback
calls may silently fail inside its sandbox**, and the orchestrator should not
assume the advisor's tool-feedback protocol succeeded just because the
advisor's actual review task did.

**Take-away for `docs/commands/HarnezAdvisor.md` / issue 304**: the skill's
compatibility-matching contract worked correctly here because every
precondition (model, effort, project identity, task scope) was explicit in
the dispatch prompt rather than inferred — this is the first real evidence
that the prose-first contract produces a genuinely independent, useful
review rather than a rubber-stamp, when the invoking agent is disciplined
about not leaking its own framing/priors into the advisor prompt.

## Finding 2 — issue 315's bug reproduced a third time, now on a plain doc-only session

While onboarding `docs/lang/ManPages.md` earlier in this stretch, a plain
`harnez init` dropped the previously-opted-in `prototyping-features` doc from
AGENTS.md (issue 315, filed and reproduced once already at that point). It
recurred again in this session, independently, while onboarding an unrelated
one-line addition to `docs/lang/Bash.md` (the `timeout` convention, issue
319) — a `harnez init` run purely to verify a doc propagated correctly
dropped `prototyping-features` from AGENTS.md a second time in the same
session's worth of work, on a completely unrelated doc.

This is now three independent live reproductions of the same root cause
across two different sessions, from two unrelated trigger actions (adding a
new auto-detected lang doc; verifying an existing lang doc's propagation).
The bug is not an edge case — it fires on *any* plain `harnez init` re-run in
a project that has ever opted into a `default: false` doc. Updated issue 315
with this recurrence; the underlying fix (either `init` preserving previously
installed opt-in docs, or bundled docs not hard-referencing opt-in-only docs)
remains unimplemented. Given three live hits in one week of actual dogfooding,
this should be treated as higher priority than P2/Moderate the next time
someone triages the backlog — it silently corrupts a project's own doc set
on the most common possible invocation (`harnez init` with no flags).

## Related open tickets

- Issue 315 (bug, open) — root cause, now with a third recurrence noted.
- Issue 297 (open) — general "linked subdocuments for detailed language
  documentation" mechanism for `docs/lang/`. This is the same underlying idea
  as issue 317 (splitting `harnez-advisor`'s per-harness guidance into
  `resources:`-backed reference files) but for lang docs, which don't yet
  have an equivalent on-demand-loading mechanism — only skills do
  (`resources:` in `config.yaml`, proven by the `docup` skill). Worth solving
  once, generically, rather than twice.
- Issue 304 (open) — advisor lifecycle CLI/metadata tracking; this session's
  Codex call was fully manual (hand-built prompt, hand-invoked `codex exec`)
  and would benefit from whatever tracking issue 304 eventually adds.
- Issue 319 (closed this session) — `timeout` command convention added to
  `docs/lang/Bash.md`.

## What I'd do differently

- Before re-running `harnez init` purely to verify a one-line doc change
  propagated, check `git diff AGENTS.md` immediately after and be ready for
  the opt-in-doc-drop — at this point it should be treated as an expected
  side effect of any plain `init` run in this repo until issue 315 is fixed,
  not a surprise to re-diagnose each time.
- Consider adding a cheap regression guard for issue 315 (a test asserting
  that a project with a `default: false` doc installed keeps it across a
  second plain `init` run) rather than relying on live dogfooding to keep
  rediscovering it.

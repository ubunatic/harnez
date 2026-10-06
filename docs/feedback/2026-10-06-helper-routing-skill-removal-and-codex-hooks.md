# Helper Routing, Skill Removal, and Codex Hook Passthrough

## Outcomes and evidence

- Issue 736: narrowed advisor discovery to explicit reuse. Ordinary help, review, and
  second-opinion requests use one fresh helper with a short prompt. Lean sprints are
  the owner's usual workflow. Installed copies were refreshed; independent routing
  review and skill validation passed. Full-suite verification remains pending.
- [harnez.org issue 032](../../../harnez.org/issues/032-remove-all-mattpocock-skills-from-the-website-and-harnez.md): removed four mattpocock bundles, their sources, eight
  marked installed copies, and five generated skill/command pages. Generic external
  and bundled support remains tested through fixtures. Clean export contains 27 skills.
  Upstream affected packages and the scoped website's complete `make check` passed.
- Codex hook bug: a no-op emitted bare `permissionDecision: allow` without `updatedInput`.
  The owner observed Codex rejecting it; the installed handler reproduced that output.
  Commit `f5d895ed` changes no-ops to successful empty output. Focused regression tests
  and installed-handler probes passed; no separate live Codex session was run.

Durable contracts now live in [CommandsPipeline](../CommandsPipeline.md),
[ExternalSkills](../ExternalSkills.md), and [CodexHooks](../CodexHooks.md).

## What helped and what cost extra work

- Scoped temporary checkouts kept unrelated telemetry and 404 edits out of verification.
  The shared website check failed in another session's `rm` completion test; the clean
  scoped build passed. Those results describe different source states.
- Independent review found default-registry assumptions in bundle tests before commit.
  Synthetic fixtures retained collision, resolution, license, and provenance coverage.
- I guessed CLI/file names instead of discovering them, made malformed patches, omitted
  an untracked embed README from the first verification copy, and initially asserted the
  wrong export origin. These avoidable failures caused retries; they were rated immediately
  in some cases, but batched calls delayed others. Stop a dependent batch on failure.
- I found `docs/CodexHooks.md` only during this evergreen pass. Reading the docs index before
  diagnosing the hook would have surfaced its existing contract and stale ownership note.
- A reviewer reasoning through sample prompts verifies instruction clarity, not actual skill
  selection in a new model session. An installed-handler probe verifies its JSON contract,
  not the complete host runtime. Reports must keep these evidence limits explicit.

## Screenshot research and background visibility follow-up

- Filed issue 737 and dispatched the requested `luna:med` directly through Harnez.
  Its [research report](../studies/2026-10-06-webpage-screenshot-inspection-telemetry-study.md)
  was committed as `edbd0936`. The sample contains 705 Codex image-view rows across
  120 sessions, but cannot establish webpage screenshot costs. Guidance stayed unchanged.
- Background self-test passed all CLI progression steps. Research and self-test used
  the same tracked execution mechanism; the difference was explicit active confirmation,
  collection through exit, and visible execution identity. Research execution session
  `77560` was eventually collected successfully; self-test session `6413` exited normally.
- I ended the research dispatch reply before collecting completion and did not surface
  its host execution ID. I also failed to consult the existing Codex-specific exception
  in [HarnezAgentArchitecture](../HarnezAgentArchitecture.md) before relying on generic
  reactive-completion advice. Next time, discover that doc first, report where the worker
  is visible, and retain the execution handle through completion while remaining responsive.
- Research findings were verified against the committed report and scoped commit stat.
  This evergreen pass adds interpretation and lifecycle guidance, not independent database
  replication or implementation changes. The telemetry attribution gap remains unresolved.

## Proposed harness improvements — not applied

- Align AGENTS.md's default workflow with the owner's lean-sprint practice; reserve the full
  sprint for an explicit request or work that needs its additional phases.
- Add one short routing rule to the existing subagent guidance: ordinary help/review requests
  dispatch directly; load lifecycle guidance only for explicit reuse. Avoid another skill.
- Before verification in a dirty shared checkout, inventory tracked and untracked task files
  and use a clean scoped source snapshot. Keep this as practical guidance, not a new approval gate.
- Clarify the existing Harnez background rule's Codex exception: use `write_stdin` to
  collect shell completion and disclose execution identity; generic reactive-notification
  wording should not imply that Codex shell jobs behave like native subagents. No AGENTS.md,
  rule source, or installed harness instructions were changed in this evergreen pass.

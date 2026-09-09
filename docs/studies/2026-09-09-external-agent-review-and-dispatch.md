<!--
SPDX-FileCopyrightText: 2026 Uwe Jugel
SPDX-License-Identifier: AGPL-3.0-or-later
-->
<!-- harnez:topic: External Agent Review, Artifact-First Completion, and Claude Dispatch Lessons -->
# External Agent Review and Dispatch Lessons

## Scope

This study records the 2026-09-09 review of the `harnez init` Go workspace
isolation changes from issue 287. The review was delegated to Claude Code
Sonnet through a noninteractive CLI invocation. It extended the existing
Codex/AGY dispatch findings in issues 288 and 291.

## What the review established

The review was run with:

```sh
claude --model sonnet --dangerously-skip-permissions -p "<self-contained review prompt>"
```

The prompt required independent verification, concrete issue filing, durable
feedback, preservation of unrelated worktree changes, and no production-code
edits. Sonnet found two real follow-ups:

- Issue 289: `harnez init` scans `testdata` module fixtures as project modules;
  a malformed fixture can make initialization fail.
- Issue 290: the intended ambient Harnez/voxi workspace currently exposes an
  `audiolevel.Spec` signature drift that breaks Harnez's normal build.

The review also verified the core issue 287 decision logic and independently
checked the `trafficsim/go.work` artifact created by the earlier fix. The
findings, tests, and effectiveness assessment were written to
`docs/feedback/2026-09-09-gowork-init-isolation-independent-review-sonnet.md`.

## Why this review was effective

The two findings came from running the code under the real ambient workspace
conditions and probing a realistic malformed fixture. A diff-only review would
likely have missed both. The independent artifact check was also cheap and
useful: a live-canary claim can be confirmed by inspecting the claimed file
and its history rather than trusting the previous agent's prose.

This is one successful Sonnet review, not a general reliability claim. The
method is most valuable for infrastructure, build, workspace, and
cross-process changes where the ambient environment is part of the behavior.

## Dispatch and teardown lessons

Noninteractive external-agent calls have two separate completion signals:

1. The child process eventually emits a final response and exit status.
2. The repository may already contain useful, reviewable artifacts.

The Claude process created the two tickets and the durable review record, then
remained alive for several minutes without producing final stdout. The caller
stopped it after a bounded wait, inspected the worktree, verified the staged
scope, and committed the artifacts. A future dispatcher must therefore expose
an explicit timeout and teardown path, report artifact presence separately from
child exit status, and never discard valid outputs merely because final prose
did not arrive.

Write-capable external review also requires a visible authorization choice.
`--dangerously-skip-permissions` was appropriate only because the operator
explicitly requested this write-capable review. It must not be a skill or CLI
default. Review-only calls should use the most restrictive available mode.

## Process improvements proposed, not implemented here

- Implement issue 291 as the single adapter boundary for Codex, AGY, and Claude
  invocations, including model/flag validation and structured dispatch records.
- Add bounded timeout, status, and teardown handling to `harnez agent`, with
  artifact-first completion states.
- Store prompt path, tool, model, authorization mode, exit status, timeout,
  termination, and artifact presence in telemetry.
- Make the review preflight run `harnez find` and `harnez index --check` before
  committing, so unrelated untracked tickets cannot surprise the commit hook.
- Keep cross-agent review explicitly independent from implementation and rerun
  the repository's own verification after the external agent reports success.

These proposals belong in issues 288/291 and future harness work. This study
does not change global agent configuration or production code.

## What we would do differently

The first Claude call should have had an explicit wall-clock timeout from the
start and a structured output channel that could report progress without
waiting for final stdout. The caller also should have run the tracker/index
preflight before staging, since unrelated untracked issues 285/286 caused the
normal commit hook to reject an otherwise isolated review commit. No unrelated
files were committed; the review artifacts were committed separately after
scope inspection.

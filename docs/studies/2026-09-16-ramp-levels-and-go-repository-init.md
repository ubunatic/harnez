# RAMP Levels and Go Repository Initialization

**Date**: 2026-09-16  
**Status**: Research and implementation plan  
**Scope**: Go libraries and Go CLI applications; TUI support in the MVP, systemd services as a later extension  
**Related**: [Quota-1 study](Quota1Approach.md), [CLI design](../CLIDesign.md)

## Source and interpretation

[RAMP: Repository AI Maturity Profile](https://arxiv.org/html/2608.25241v1) classifies repositories by **committed AI practice artifacts**. Its level is the highest category evidenced by at least one artifact. Lower-level artifacts are common at higher levels, but they are not mandatory prerequisites. An L4 artifact without L2 context still yields L4 with a coherence warning. The paper's classifier uses tool-path rules and semantic embeddings; a path-and-content-rule implementation in Harnez would be a **RAMP-informed estimate**, not a reproduction of that classifier.

| Level | Paper category | Qualifying evidence |
| --- | --- | --- |
| L1 | Unconfigured | No validated committed AI artifact |
| L2 | Grounded prompting | AI behavior rules, AI tool config, agent-facing architecture or coding standards |
| L3 | Agent augmented | Named agents, reusable commands or domain skills |
| L4 | Orchestration | Runnable multi-agent flows or records of actual agent sessions |

The paper reports a repository-level classifier match of 34/35 against held-out human labels. In its observational agent-first comparison, L1 repositories had larger increases in cognitive complexity and static-analysis warnings than L2+ repositories. These are **reported associations**, not evidence that adding a template causes better software. Artifact presence also says little about specificity, freshness, enforcement, or whether an agent reads it. The paper had no L4 examples in its 441-repository development sample; L4 claims merit especially careful evidence review.

## Two results that Harnez should report

1. **Committed RAMP profile**: classify tracked, committed AI artifacts by category and retain the highest qualifying level. Show evidence paths and reasons. A working-tree file created by `init` is *projected* until committed. Untracked personal settings, global skills, ordinary human-facing docs, CI, tests, Make targets, and systemd units do not themselves count.
2. **Go agent-readiness gaps**: inspect whether a coding agent can find reliable project commands, module/package boundaries, API compatibility rules, test guidance, and app behavior. This is a Harnez recommendation list, not part of the paper's score. It may improve a repo that is already L3 or L4.

For each finding, preserve the category, evidence path, matching rule, confidence or ambiguity, and tracked/committed/projected state. Report gaps as suggestions. A missing lower level is a coherence warning, not a score cap. Never create filler agents, skills, session logs, or flows merely to raise the label.

## Go MVP signals and improvements

- **Shape detection**: inspect `go.mod`, `go.work`, package declarations, `main` packages, and existing build/test entry points. Distinguish library, CLI app, and mixed repo. Cobra usage is a useful CLI signal, not a requirement. TUI and service operation are additional capabilities, not exclusive project types. Do not execute project commands merely to classify.
- **L2 grounding**: existing `init` already creates `AGENTS.md` and auto-detects Go from `go.mod`. Add concise project-specific pointers: public package/API compatibility and examples for libraries; invocation, flags, output channels, exit behavior, config, and cancellation for CLI apps; terminal restoration, resize, Unicode width, and deterministic UI tests for TUIs. Link to actual files and commands when present; mark unknowns rather than inventing them. Preserve hand-written text and keep managed sections idempotent.
- **L3 capabilities**: offer opt-in, task-specific reusable guidance for Go change/verify and review. It should use the detected project shape and real test targets. A reusable skill or command qualifies only when it is an actual agent-facing artifact with a clear trigger and procedure. Do not scaffold a generic named agent without a job or invent a multi-agent workflow.
- **Service extension**: when a project contains unit files or service lifecycle code, offer separate guidance for startup/shutdown, signals, state/config paths, logs, and safe service verification. Never install, enable, or start a service during `init`.

Quota-1 is an optional test-loop guardrail adjacent to this work. Its `AGENTS.md` rules are L2 evidence; a genuinely reusable agent-facing procedure may be L3. `make test-q1` and the `harnez exec --quota-1` gate are enforcement signals, **not** RAMP categories. The shipped gate in `internal/quota1/quota.go` uses file timestamps and explicit bypass variables; it does not count agent turns. Eligible doc/ticket edits in a shared workspace can release the gate, and direct test commands bypass the wrapper. See the [implementation section of the Quota-1 study](Quota1Approach.md#7-implementation-architecture--shipped-components) rather than treating its original proposal as shipped behavior.

## Proposed `init` flow

1. Inspect before changes and emit a bounded baseline report, with an optional machine-readable form. Read repository files only; do not run `go`, Make, tests, scripts, or service managers during assessment.
2. Plan only project-local changes. Preview the files/managed sections to be written and require an explicit opt-in for new RAMP-oriented scaffolding. `apply` remains global-only.
3. Reconcile L2 guidance first, then selected L3 capabilities. Use existing managed-section rules; preserve custom content. Reassess after writes, showing projected changes separately from the committed baseline.
4. On a clean second run, report no changes. Never claim a higher committed level until files are committed.

Fixtures should cover a pure Go library, a Cobra CLI, a CLI with a TUI, a mixed module/workspace, and a service-bearing CLI. Include false-positive cases: generic `README.md`, unused template files, mere `Makefile` targets, and prose describing orchestration without a runnable flow. Verify offline operation, deterministic output, content preservation, and idempotency.

## Open design decisions

- Decide whether assessment is `init --ramp-report` or a reusable project-local assessment function surfaced by `init` and `assess`. Reuse `harnez assess` where it fits, while keeping write scope in `init`.
- Define a small versioned artifact-rule set and an explicit `unknown` state. Ambiguous custom tool directories should not be silently promoted.
- Pilot the proposed guidance on real sibling Go projects and review whether it tells an agent what to do at the relevant point of work. This is a content-quality check separate from the RAMP label.

## Evidence labels

- **Reported**: RAMP levels, classifier validation, and observed quality associations come from the linked paper.
- **Measured by code inspection**: current Go detection checks `go.mod` in `internal/claude/init.go`; `init` manages `AGENTS.md`, docs, and Makefile; Quota-1 behavior is in `internal/quota1/quota.go`.
- **Proposed**: all Harnez detector, report, and scaffolding behavior in this study. No effectiveness or token-saving claim has been measured for it.

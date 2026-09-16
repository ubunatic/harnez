# 368 — Report RAMP evidence and projected changes from harnez init

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `docs/studies/2026-09-16-ramp-levels-and-go-repository-init.md`, `docs/CLIDesign.md`, [[057-repo-assessment-and-code-metrics-command]]

---

## 1. Problem & Motivation

`harnez init` creates useful agent instructions but cannot explain a project's current RAMP evidence or the effect of its planned writes. The paper scores committed AI artifacts; reporting newly generated working-tree files as achieved maturity would be misleading.

## 2. Scope & Design

- Add an offline, read-only artifact inventory for the nine RAMP categories, with a small versioned rule set and bounded file reads. Use tracked committed content for the baseline, and label uncommitted/new content as projected. Handle absent Git metadata explicitly.
- Return the highest evidenced level even if lower categories are absent; emit a coherence warning instead of reducing the level. Include path, category, match reason/rule ID, and confidence or `unknown` for ambiguous cases. Avoid false promotion from generic docs, Make targets, CI, or service units.
- Expose a bounded human report through `harnez init` and machine-readable output if practical. Choose the CLI spelling during implementation; reuse the existing `assess` internals where appropriate. Keep `apply` global-only and detector calls free of `go`, Make, tests, scripts, and service-manager execution.
- Keep RAMP classification separate from practical Go-readiness recommendations. Do not claim to reproduce the paper's embedding classifier; label the result a RAMP-informed estimate.

## 3. Exit Criteria

- [ ] Fixtures cover L1–L4, mixed levels, missing lower levels, ambiguous paths, ordinary project docs, and uncommitted scaffolding.
- [ ] A report cites inspectable evidence and distinguishes baseline from projected level.
- [ ] Repeated assessment has stable output and makes no repository or global writes.
- [ ] CLI/design docs explain the scoring rule and its limits.

## 4. Verification

Run the project test target once per Quota-1 step after source edits, then exercise the report on fixture repos and confirm no project commands run or files change. Keep any future write option outside this read-only detection layer.

# 1136 — harnez release: infer bump size from tag history, clarify missing build, and warn on tag-only repos without version.yaml

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 091, 243, 698, 866, docs/GoRelease.md

---

## 1. Problem & Motivation
When running `harnez release` across different repositories:
1. **Bump size**: The default bump flag is hardcoded to `--bump patch` (`cmd/harnez/release.go`). For projects that evolve primarily in minor steps (such as `taia`: v0.1.0 -> v0.2.0 -> v0.3.0), defaulting to patch is wrong and requires manual override every time. `harnez release` should inspect the recent tag history to infer the standard bump increment if none is explicitly specified.
2. **Missing build clarity**: When no build configuration exists (no custom `build_cmd`, no `.goreleaser.yaml`, and no make target `dist`/`build`), the release runner outputs a quiet skipping note. It should say clearly and prominently that no build configuration or artifacts exist, distinguishing between intentional tag-only releases and missing build configurations.
3. **Tag-only repos without `version.yaml`**: Tag-only projects that lack a `version.yaml` single source of truth should trigger an explicit warning advising the user or agent about the missing spec file rather than silently synthesising an in-memory spec.

## 2. Technical Specification & Findings
- **Bump Inference** (`cmd/harnez/release.go`, `internal/release`):
  - When `--bump` is not explicitly supplied on the CLI, inspect git tags (`git tag --list --sort=-v:refname` or chronological recent tags).
  - If recent release tags reflect minor increments (e.g. `v0.1.0` -> `v0.2.0`), default to `minor`; if patch increments, default to `patch`.
  - Allow explicit CLI `--bump` (`patch`, `minor`, `major`, explicit version) to override any inferred default.
- **Missing Build Feedback** (`internal/release/runner.go:runBuildStep`):
  - Distinguish when `--skip-build` was explicitly passed vs. when build detection found no targets.
  - Clearly report status: `[build] No build configuration detected (no build_cmd in version.yaml, no .goreleaser.yaml, no make dist/build target); proceeding without artifact compilation.`
  - If downstream steps (`--skip-sign=false` or `--skip-publish=false`) expect artifacts in `dist/`, report or handle cleanly.
- **Warning on Missing `version.yaml`** (`internal/release/runner.go`):
  - If `spec == nil` / `version.yaml` is absent in `opt.Dir`, emit a clear warning banner:
    `[warning] No version.yaml found in <dir>; release is running in tag-only fallback mode without a declared release spec.`

## 3. Implementation & Verification Plan
- **M1**: Implement tag history inspection in `internal/release` to determine the predominant recent bump type when `--bump` flag is unset.
- **M2**: Add warning for missing `version.yaml` and clarify the no-build detection output in `runBuildStep`.
- **M3**: Add unit tests in `internal/release` testing bump inference across various git tag sequences (minor-step, patch-step, initial release) and missing build / missing spec scenarios.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| audit | jules-audit-1136 | - | - | failed: start refused, HTTP 400 "Precondition check failed" (06:33) |
| audit | jules-audit-1136b | - | - | failed: same refusal 3 min later, with 1 session open; likely the 2026-11-11 stop on new Jules work |

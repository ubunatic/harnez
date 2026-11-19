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

## Decision

- **Tag-history `--bump` inference**: Default `--bump` on `cmd/harnez/release.go` to `""` (`auto`: infer when `--bump` is omitted / `!Changed("bump")`, keeping explicit `--bump <patch|minor|major|x.y.z>` or positional bump arg highest priority). Inspect up to the latest 4 stable semver tags (ordered descending by version / `git tag --list --sort=-v:refname` parsed with `ParseSemver`, ignoring pre-releases unless only pre-releases exist); count consecutive same-major transitions (`minor` when `newer.Minor > older.Minor && newer.Patch == 0`; `patch` when `newer.Minor == older.Minor && newer.Patch > older.Patch`), never auto-infer `major`, tie-break toward the most recent transition, and fall back to `patch` when fewer than 2 stable tags exist or ambiguous. Emit `[version] Inferred bump: <bump> (from recent tags: ...)` so inference is visible in `-n`/real runs. (Host `/decide`, 2026-11-19.)
- **Malformed vs missing `version.yaml`**: Distinguish `os.IsNotExist` (tag-only fallback: emit `[warning] No version.yaml found in <dir>; release is running in tag-only fallback mode without a declared release spec.`) from invalid YAML errors in an existing spec file (fail cleanly instead of overwriting malformed `version.yaml`). (Host `/decide`, 2026-11-19.)
- **Build & downstream step clarity**: Distinguish `--skip-build` (`[build] Build skipped (--skip-build)`) from auto-detected absence (`[build] No build configuration detected (no build_cmd in version.yaml, no .goreleaser.yaml, no make dist/build target); proceeding without artifact compilation.`) and keep existing downstream warning clarity when `dist/` has no artifacts for signing/publishing. (Host `/decide`, 2026-11-19.)

## M1 — Release bump inference, build clarity, missing `version.yaml` warning, tests & docs

- Write set (expected): `cmd/harnez/release.go`, `cmd/harnez/testdata/help.golden`, `internal/release/{runner,version,sync,infer}.go`, `internal/release/*_test.go`, `cmd/harnez/release*_test.go`, `docs/practices/GoRelease.md`, `docs/practices/GoRelease.lite.md`, `docs/GoRelease.md`, `docs/commands/release.md` (if it states `--bump` default).
- Update `help.golden` for `--bump` flag description and sync any edited copyable doc sources via `make install && ./harnez init -d .` (checking `git diff` and reverting unrelated files).
- Locked tests:
  1. Bump inference across minor sequence (`v0.1.0 -> v0.2.0 -> v0.3.0` -> `minor`), patch sequence (`v0.1.0 -> v0.1.1 -> v0.1.2` -> `patch`), minor-then-patch shift (`v0.1.0 -> v0.2.0 -> v0.2.1` -> `patch`), <2 tags/empty (`patch`), non-semver/prerelease noise ignored, explicit `--bump patch|minor|major|2.0.0` overrides inference, `gitflow_test.go` (`1.0.0 -> 1.0.1`) still green.
  2. `runBuildStep`: `--skip-build` prints skip reason distinct from missing build config; missing build config emits exact `[build] No build configuration detected ...` message; signing/publishing with empty `dist/` handles/reports cleanly (`release_test.go:1114` updated ONLY to match the new message while keeping all assertions).
  3. Missing `version.yaml` emits `[warning] No version.yaml found in <dir>; release is running in tag-only fallback mode without a declared release spec.`; malformed existing `version.yaml` fails without overwriting the file.

## Refinements (from `rev-1136` review of `c5dd6ce8`)

1. **Tag prefix support (`internal/release/infer.go`, `runner.go`)**: pass `tagPrefix` into `InferBumpFromTags(dir, tagPrefix)` so custom `tag_prefix` / `--tag-prefix` (e.g. `taia-v0.1.0 -> taia-v0.2.0` -> `taia-v`) are matched/stripped before `ParseSemver` (when `tagPrefix == ""`, keep `v` handling). Add unit test for custom `tagPrefix`.
2. **Discriminating latest-4 test (`internal/release/infer_test.go`)**: replace the non-discriminating fixture with `v0.1.0, v0.2.0, v0.3.0, v0.4.0, v0.5.0, v0.6.0, v0.6.1, v0.6.2, v0.6.3` (capped 4 -> `patch`; uncapped -> `minor`).
3. **Error fallback test**: add unit test verifying `InferBumpFromTags` returns `"patch"` cleanly on a non-git directory (`git tag` failure).
4. **Error check, `--continue`, alignment & help (`internal/release/runner.go`, `cmd/harnez/release.go`, `help.golden`)**:
   - Use `errors.Is(err, os.ErrNotExist)` on `LoadVersionSpec`.
   - Suppress the tag-only fallback missing-`version.yaml` warning during `--continue` (or clarify it applies only when bumping, matching `--continue` semantics without breaking tests).
   - Align `[build]` column spacing (`  [build]     ...`) cleanly with siblings, and update `cmd/harnez/release.go:17` + `cmd/harnez/testdata/help.golden` to reflect tag-history bump inference.

## Jules runs

| kind | session name | Jules ID | PR | state |
| --- | --- | --- | --- | --- |
| audit | jules-audit-1136 | - | - | failed: start refused, HTTP 400 "Precondition check failed" (06:33) |
| audit | jules-audit-1136b | - | - | failed: same refusal 3 min later, with 1 session open; likely the 2026-11-11 stop on new Jules work |

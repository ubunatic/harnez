---
kind: sprint
ticket: 1136
date: 2026-11-19
branch: sprint/taia-harnez-pack
---

# Two-Agent Lean Sprint: 1136 — `harnez release` Bump-Size Inference & Missing Build/Spec Warnings

## 1. Scout Audit (`search-1136`, `claude:haiku:med`, 43s)
- Mapped `cmd/harnez/release.go:38` (`--bump` default `"patch"` shadowing empty/positional arg), `internal/release/runner.go` (`LoadVersionSpec` error discard `:105`, `runBuildStep` `:466-525`, signing `:567-572`, publish `:601-635`), `version.go` (`ParseSemver`, `BumpVersion`), `help.golden`, and `docs/practices/GoRelease{,.lite}.md` + `docs/GoRelease.md`.

## 2. Milestones & Review
- **M1 (`c5dd6ce8`)**: `dev-1136` (`agy:neon:low`, 12m28s, 1.09M tokens incl. cached) defaulted `--bump` to `""` (`InferBumpFromTags` inspecting up to the latest 4 stable semver tags, never inferring `major`, falling back to `patch`), surfaced missing `version.yaml` vs malformed YAML errors cleanly, distinguished `--skip-build` from missing build configuration, clarified empty-`dist/` signing logs, updated `help.golden` + `docs/practices/GoRelease{,.lite}.md` / `docs/GoRelease.md` / `docs/commands/release.md`, and passed `make lint` + `HTO=0 make test-q1`.
- **Cross-vendor Review (`rev-1136`, `claude:haiku:high`, 1m26s)**: Returned **PASS WITH MINOR FINDINGS** (tag prefix `tag_prefix` not stripped in `InferBumpFromTags`, non-discriminating latest-4 test fixture, untested `git tag` failure fallback, `errors.Is(err, os.ErrNotExist)` + `--continue` fallback-warning suppression + `[build]` column alignment + `release.go:17` help wording).
- **Refinements (`5d3b6b56`)**: `dev-1136-r2` (`claude:haiku:high`, 4m59s, 329k tokens incl. cached) passed `tagPrefix` to `InferBumpFromTags`, added custom-prefix + discriminating latest-4 + non-git fallback tests, updated `[build]` spacing, suppressed missing-`version.yaml` banner on `--continue`, updated `release.go` / `help.golden`, and passed `make lint` + `HTO=0 make test-q1`.
- **Host Verification**: `make install`, `make lint` (`0 issues.`, `lint: ok`), `HTO=0 go test ./internal/release/...` (`1.778s`, `ok`), and `HTO=0 go test ./cmd/harnez/...` (`69.367s`, `ok`).

## 3. Outcomes
- `harnez release` now infers `minor` vs `patch` from recent stable release tags (including custom `tag_prefix` repos like `taia`) unless `--bump` or a positional version/bump is passed.
- Missing build configurations vs `--skip-build` and tag-only fallback without `version.yaml` emit explicit diagnostic banners, while malformed `version.yaml` files fail safely without being overwritten.

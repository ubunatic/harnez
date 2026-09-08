# 284: harnez release build step defaults to GOWORK=off, with --allow-workspace override

**Status**: Closed — GOWORK=off default shipped and verified live
**Priority**: P2 (Medium)
**Severity**: Bug (silent-correctness risk)
**Category**: Release Engine
**Related**: [docs/practices/GoRelease.md](../docs/practices/GoRelease.md), [internal/release/runner.go](../internal/release/runner.go), [voxi issue 085](../../voxi/issues/085-onboard-voxi-to-harnez-release-pipeline.md)

---

## 1. Problem & Motivation

While onboarding `voxi` to `harnez release` (voxi issue 085), we discussed
adding a `go.work` at `~/projects/go.work` (`use ./harnez ./voxi`) for local
cross-repo co-development, since harnez's `internal/usage/mic.go` now
imports `ubunatic.com/voxi/audiolevel`. `go.work` is untracked (lives
outside any individual repo) and normally the right tool for this, but Go
auto-detects it by walking up from the CWD — so it also silently applies
to `harnez release`'s own build step (`goreleaser`/`make`/`--build-cmd`,
all of which shell out to `go build` under the hood).

Confirmed live: with the workspace active,
`go list -m ubunatic.com/voxi` from inside `harnez` resolved to the local
checkout with **no version** (workspace substitution); the same command
under `GOWORK=off` correctly resolved to the pinned, tagged `v0.1.1`. Left
unaddressed, a release cut from a co-development workspace could silently
build against untagged local sources instead of the pinned `go.mod`/
`go.sum` dependency it claims to release against — a correctness gap in
the release engine's core guarantee (build == what's pinned/tagged).

## 2. Resolution

- Added `Options.AllowWorkspace` (`internal/release/runner.go`), default
  `false`.
- New `runBuildCmd`/`buildEnv` helpers: the build-step subprocess
  (goreleaser, `make`, custom `--build-cmd`) now runs with `GOWORK=off`
  forced into its environment unless `AllowWorkspace` is set. `buildEnv`
  strips any pre-existing `GOWORK` env entry before appending `GOWORK=off`
  so an inherited shell `GOWORK` override can't leak through.
- New `harnez release --allow-workspace` flag to opt back in deliberately
  (e.g. cutting a release to validate an in-flight cross-repo change
  before either side is tagged).
- The `[build]` log line now states which mode ran (`GOWORK=off` vs.
  `GOWORK honored: --allow-workspace`), so it's visible in release output
  rather than an invisible environment difference.

## 3. Verification

- New unit test `TestBuildEnvGOWORK` (`internal/release/release_test.go`)
  — asserts `buildEnv(false)` forces `GOWORK=off` and strips any existing
  `GOWORK=...` entry; `buildEnv(true)` leaves the environment untouched.
- `go test ./...` — all green.
- Live, with `~/projects/go.work` actually present and active:
  - `harnez release --dry-run --force` (no flag) → logs
    `Running goreleaser ... (GOWORK=off)`.
  - `harnez release --dry-run --force --allow-workspace` → logs
    `Running goreleaser ... (GOWORK honored: --allow-workspace)`.
- Documented in `docs/practices/GoRelease.md` §4 ("go.work and Local
  Co-Development").

## 4. Notes

This is the kind of failure the project's own `docs/AgenticLoop.md`
"Live/Real-Environment Verification for hooks & env-resolution features"
review-checklist item exists to catch — a green `go test ./...` would not
have surfaced it; only actually creating the workspace and running
`go list -m` / `harnez release --dry-run` against it did.

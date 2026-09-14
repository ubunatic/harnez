# 338 — macOS CI verification via GitHub mirror

**Status**: Open — core CI landed, Homebrew scope dropped, push/PR trigger + CLI smoke test remain
**Priority**: P2 (Medium)
**Severity**: Build & Distribution (macOS Delivery)
**Category**: Packaging & CI / Multi-OS
**Related**: [docs/MacOSPortability.md](../docs/MacOSPortability.md), [341](341-concurrent-sqlite-telemetry-writers-lose-rows-on-macos.md)

---

## 1. Problem & Motivation

`harnez` builds native macOS binaries (`harnez_darwin_amd64` and `harnez_darwin_arm64`) via `.goreleaser.yaml`, but had no continuous integration (CI) pipeline running on macOS hosts to verify build health, runtime execution, and tests against real Darwin kernels.

Codeberg (the primary git forge) does not provide native macOS runner instances. The repository has a synced GitHub mirror (`git@github.com:ubunatic/harnez.git`), which supports GitHub Actions with native macOS runners.

**2026-09-14 decision**: Homebrew tap distribution is out of scope, now and likely
long-term. macOS users install the same way as every other platform — `curl`
installer or `go install` — not via `brew`. The original title/scope included a
`brews:` GoReleaser block; that has been dropped from this ticket entirely.

## 2. What's Done (2026-09-14)

- `.github/workflows/macos-hello.yaml` — `workflow_dispatch`-only, `macos-14`
  runner, `go build ./...` + `go test ./...`.
- `make macos-ci` (`scripts/macos-ci.sh`) — dispatches the run and polls
  quietly (no `gh run watch` job-tree spam), prints a single PASS/FAIL summary.
- `scripts/install-dev-deps.sh` — OS-aware dev-tool installer (currently just
  `minisign`), kept out of the workflow YAML to keep it minimal; candidate for a
  future `harnez install --dev` subcommand.
- First real run surfaced a genuine macOS-only bug: [issue #341](341-concurrent-sqlite-telemetry-writers-lose-rows-on-macos.md)
  (`internal/telemetry` concurrent SQLite writer race).

## 3. Remaining Scope

1. Decide whether/how to wire `macos-hello` into `push`/PR triggers on the
   mirror (currently manual-only via `make macos-ci`) — weigh macOS runner cost
   against the value of catching platform regressions automatically.
2. Add a `harnez` CLI smoke test step (not just `go test ./...`) to catch
   runtime/packaging issues the unit tests wouldn't (e.g. `harnez --help`,
   a real `harnez status` run) on Darwin.

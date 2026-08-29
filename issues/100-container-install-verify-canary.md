# 100 — Container canary: install `latest` Codeberg release and verify `--version`

**Status**: Resolved — `scripts/install-canary.sh`, `make install-canary`
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: `.goreleaser.yaml`, [[091-language-agnostic-release-spec-and-thin-make-release]], [[092-latest-release-links-and-readme-install-section-consolidation]], [[098-deb-rpm-packaging]], [[099-appimage-packaging]], `docs/Canary.md`

## Problem

Nothing currently verifies that a published release is actually
installable by a real, fresh user. `harnez release`'s preflight checks the
publishing side (toolchain, minisign key, forge auth — see issue 096), but
there's no check on the consuming side: does `curl`-ing/downloading the
`latest` release from Codeberg and installing it on a clean machine
actually produce a working binary? A broken tarball, a wrong arch mapping,
a stale/missing install script, or a binary that fails to start on a
minimal base image would currently only be caught by a user filing a bug.

This is exactly the "probe the real external mechanism" case
`docs/Canary.md` describes — a real download against the real Codeberg
release, not a mock.

## Proposed Fix

Add a container-based install-verify canary:

1. Spin up a clean container (minimal Debian or Fedora base — no Go
   toolchain, no dev tools, matching what a real end user's machine looks
   like) via Podman, consistent with this workspace's existing Podman-first
   convention (see `conreel`, wayreel issue 23).
2. Inside it, fetch the `latest` release for a given project from
   `codeberg.org/ubunatic/<project>/releases/latest` using whatever the
   project's actual documented install path is (tarball + manual extract
   today; `curl | sh` script or `.deb`/`.rpm`/AppImage once 098/099 land —
   this canary should exercise whichever paths actually exist for that
   project, not assume one).
3. Run `<binary> --version` (or the project's equivalent) and assert the
   reported version matches the tag just fetched.
4. Fail loudly and specifically (wrong version, binary missing, exec
   permission/format error, network/404) rather than a generic non-zero
   exit.

Land this in `harnez` first (dogfooding, per 098/099's convention), as
either a `harnez` subcommand (e.g. `harnez release verify` run after
publish) or a standalone `scripts/` canary — decide which based on whether
this needs to be a first-class, reusable-across-projects primitive or a
one-off verification script. Given the workspace already has 20-50
sibling repos publishing releases the same way, lean toward the reusable
primitive, but confirm that's actually wanted before generalizing beyond
`harnez` itself.

## Notes

- This becomes the natural verification step for issues 098 (`.deb`/`.rpm`)
  and 099 (AppImage) once those land — install via `apt`/`dnf` or run the
  AppImage inside the same kind of clean container and check `--version`,
  rather than inventing a separate verification mechanism per package
  format.
- Scope questions to resolve before implementing: run this per-release
  (blocking `harnez release` from completing) vs. as a separate, later
  `harnez release verify` step a human/CI triggers deliberately — the
  former catches breakage immediately but adds container-spin-up latency
  to every release; the latter is opt-in but can't accidentally block a
  release on it.

## Progress (2026-08-29)

Implemented as a standalone shell canary (`scripts/install-canary.sh`,
`make install-canary`), not a `harnez` subcommand — this stayed scoped to
`harnez` only, per the "confirm before generalizing" note above; no
cross-project mechanism was built.

What it does:
1. Queries `https://codeberg.org/api/v1/repos/ubunatic/harnez/releases/latest`
   for the current tag (no assumptions, no mocking — the real API).
2. Spins up a clean `debian:bookworm-slim` container via Podman (no Go
   toolchain, matching a real end user's machine).
3. Inside it, installs only `curl`/`ca-certificates`, downloads the
   matching `harnez-<version>-x86_64-linux.tar.gz` archive asset, extracts
   it, and runs `./harnez --version`.
4. Asserts the reported version string contains the fetched tag's version;
   fails loudly (`FAIL: expected version ..., got: ...`) on mismatch, or an
   early `curl -f` failure on a 404/network error.

Verified against the real, currently-published `v0.1.5` release:
```
Verifying harnez v0.1.5 (x86_64) installs and reports its version...
reported: harnez version 0.1.5
PASS: harnez --version reports 0.1.5
```

Scope decision: today's only documented install paths are `go install` and
`git clone && make install` (README's Installation section) — the tar.gz
archive isn't actually linked from the README yet (that's issue 092's
job). This canary exercises the tar.gz archive path directly since that's
the artifact `.goreleaser.yaml` actually produces and is what 098/099 will
sit alongside; once 092 lands a documented `curl | sh` or package-manager
path, extend this canary to cover those too rather than assuming one now.

Left as a follow-up, not decided here: whether this becomes a blocking
step in `harnez release` or stays a separate, deliberately-triggered
`make install-canary` — kept as the latter (unblocked, opt-in) since nothing
in this session called for changing the release flow itself.

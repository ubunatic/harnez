# 100 — Container canary: install `latest` Codeberg release and verify `--version`

**Status**: Open
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

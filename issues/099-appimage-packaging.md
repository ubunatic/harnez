# 099 — Add AppImage packaging to `harnez release`

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: `.goreleaser.yaml`, [[091-language-agnostic-release-spec-and-thin-make-release]], [[092-latest-release-links-and-readme-install-section-consolidation]], [[098-deb-rpm-packaging]]

## Problem

`harnez` ships as a `tar.gz` archive and a raw binary (`.goreleaser.yaml`,
linux amd64/arm64) — no AppImage. AppImage gives a single self-contained
executable that runs across distros without a package manager or install
step at all, which is a meaningfully different distribution story from
either the raw binary or `.deb`/`.rpm` (issue 098): no root, no system
package DB entry, just `chmod +x` and run — closer in spirit to the
existing `curl | sh` static-binary install than to native packaging.

## Proposed Fix

- Evaluate `goreleaser`'s AppImage support (via `nfpm` doesn't cover this —
  AppImage needs a separate build step, e.g. `linuxdeploy` or a dedicated
  goreleaser plugin/pipe) vs. a standalone script invoked from the release
  flow.
- Since `harnez` is a single static Go binary with no runtime assets
  (unlike a GUI app needing `.desktop`/icon bundling), confirm whether a
  minimal AppImage (binary + tiny AppRun wrapper) is worth the added build
  complexity over just the existing raw-binary artifact — this may be
  lower value for a CLI-only tool than for GUI projects in the workspace.
  Decide that before implementing, not after.
- If pursued: same version/arch matrix as the existing archives, signed
  consistently with the existing minisign step where feasible.

## Notes

- Filed alongside issue 098 (`.deb`/`.rpm`) at the same time, but keep them
  independent — AppImage's tooling and value proposition are different
  enough that one shouldn't block the other.
- Issue 092 already anticipated this ("When `.deb`, `.rpm`, or AppImage
  packaging is added to `harnez release`, how should projects declare which
  package types appear in their install snippet?") — fold the install-
  snippet question back into that ticket once this lands, rather than
  deciding it here.
- Scope this to `harnez` itself first (dogfooding), then decide whether/how
  it becomes a template other projects adopt.

# 098 — Add DEB and RPM packaging to `harnez release`

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: `.goreleaser.yaml`, [[091-language-agnostic-release-spec-and-thin-make-release]], [[092-latest-release-links-and-readme-install-section-consolidation]], [[099-appimage-packaging]]

## Problem

`.goreleaser.yaml` currently only produces a `tar.gz` archive and a raw
binary per arch (`archives:` — ids `default` and `binary`, linux
amd64/arm64). There's no `.deb` or `.rpm` output, so Debian/Ubuntu and
Fedora/RHEL users install via the `curl | sh` script or a raw binary
rather than their native package manager (no dependency tracking, no
`apt`/`dnf` upgrade path, no uninstall hook).

## Proposed Fix

Add an `nfpm:` section to `.goreleaser.yaml` (goreleaser's built-in nfpm
integration builds `.deb` and `.rpm` from the same build artifacts, no
separate toolchain needed):

- Package name `harnez`, same version/arch mapping already used by the
  existing archives (`amd64`→x86_64, `arm64`→aarch64).
- Install the binary to `/usr/bin/harnez` (or wherever this project's
  Go-toolchain-free install convention points — check against `make
  install`'s target).
- Include `README.md`/license as package docs, matching what the `default`
  archive already bundles.
- Sign the resulting `.deb`/`.rpm` consistently with the existing minisign
  step (`signs:`), or via each format's native signing if minisign doesn't
  apply cleanly to package formats — needs checking.

## Notes

- Issue 092 already anticipated this ("When `.deb`, `.rpm`, or AppImage
  packaging is added to `harnez release`, how should projects declare which
  package types appear in their install snippet?") — once this lands, fold
  the install-snippet question back into that ticket rather than deciding
  it here.
- `spriteview/docs/studies/2026-08-28-releasing-a-pure-python-project-with-goreleaser.md`
  may have relevant prior findings on goreleaser packaging quirks — check
  before implementing.
- Scope this to `harnez` itself first (dogfooding), then decide whether/how
  it becomes a template other projects' `.goreleaser.yaml` configs adopt —
  don't build a generic cross-project mechanism speculatively.

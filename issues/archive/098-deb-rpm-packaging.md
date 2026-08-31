# 098 — Add DEB and RPM packaging to `harnez release`

**Status**: Closed — resolved in `4f3817e`, 2026-08-29
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

## Progress — 2026-08-29

Added an `nfpms:` section to `.goreleaser.yaml` (goreleaser's key is
plural `nfpms:`, not `nfpm:` as in the ticket title — confirmed via
`goreleaser jsonschema`).

- `package_name: harnez`, formats `[deb, rpm]`, same
  `{{ .Version }}-{{ arch: x86_64|aarch64 }}-linux` name template already
  used by the `default`/`binary` archives.
- `bindir: /usr/bin` — checked against `Makefile`: `make install` runs
  `go install ./cmd/harnez` (`~/go/bin`, no toolchain-free path), and
  `make install-system` uses `/usr/local/bin` via `sudo install`. Neither
  is the FHS-standard destination for a distro package manager install;
  `/usr/bin` is what Debian/Fedora `.deb`/`.rpm` packages conventionally
  use, distinct from both existing manual-install targets.
- `contents:` bundles `README.md` and `config.yaml` into
  `/usr/share/doc/harnez/`, matching what the `default` archive's
  `files:` list already bundles. No `LICENSE` file exists anywhere in
  this repo (checked `find`/`grep` for `LICENSE`/`REUSE.toml`/"license"
  mentions — none found), so the `license` nfpm field and a license doc
  were left out rather than fabricated; the ticket's "license" bullet
  doesn't apply until a LICENSE file is added to the repo (separate
  concern, not blocking this ticket).
- Signing: no changes needed. `signs:` already signs the `checksum:`
  artifact (`SHA256SUMS`), and goreleaser adds `.deb`/`.rpm` outputs to
  that checksum file automatically like every other artifact — verified
  by inspecting `dist/SHA256SUMS` after a snapshot build, which listed
  both new package files alongside the existing archives/binaries.
  nfpm's own per-format signing (`deb.signature`/`rpm.signature`) was not
  needed since the existing minisign-over-checksum scheme already covers
  package integrity consistently with every other artifact.

**Verification**: `goreleaser check` passed. Ran
`goreleaser release --snapshot --clean --skip=sign,publish` (no tag/push,
no real release) — produced `harnez-<ver>-{x86_64,aarch64}-linux.{deb,rpm}`
in `dist/`. Inspected with `dpkg -c`/`dpkg -I` and `rpm -qlp`/`rpm -qip`:
both formats install `/usr/bin/harnez` (correct mode, root-owned) plus
`/usr/share/doc/harnez/{README.md,config.yaml}`; package metadata
(name, version, arch, maintainer, homepage, description) all populated
correctly; both `x86_64`/`aarch64` variants built for both formats.
`dist/` was removed after verification (gitignored, not committed).

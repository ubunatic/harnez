# 099 — Add AppImage packaging to `harnez release`

**Status**: Closed — evaluated, not pursued
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

## Progress / Decision (2026-08-29)

Evaluated per the ticket's own "decide before implementing" gate. Decision:
**do not pursue** AppImage packaging for `harnez`. `.goreleaser.yaml` was
not touched.

Rationale:

- **No native goreleaser support.** Confirmed via `goreleaser jsonschema`
  (goreleaser v2, current installed version) — there is no `appimage` key
  anywhere in the schema, matching the ticket's own assumption. Getting an
  AppImage out of the release flow would mean a standalone build step
  (`linuxdeploy`/`appimagetool`) wired in via a goreleaser hook/publisher,
  entirely outside goreleaser's artifact/checksum/sign pipeline that 098
  already gets "for free" for `.deb`/`.rpm`.
- **Tooling isn't present and adds a real CI dependency.** Neither
  `appimagetool` nor `linuxdeploy` is installed locally or accounted for
  anywhere in this repo's build environment. Standing up AppImage output
  means vendoring/installing an external, non-Go, non-goreleaser binary
  tool in CI just to produce this one artifact — real added complexity for
  a P3/Low ticket.
- **The value proposition is already met.** AppImage's differentiator per
  the ticket is "no root, no package manager, just chmod +x and run."
  `harnez` already ships exactly that today via the `binary` archive id in
  `.goreleaser.yaml` (`archives: - id: binary`, `formats: [binary]`) — a
  single static Go binary (`CGO_ENABLED=0`), no runtime assets, no
  `.desktop`/icon bundling needed or wanted for a CLI tool. An AppImage
  here would just be that same binary re-wrapped in a squashfs image plus
  an `AppRun` shim.
- **AppImage would arguably be a downgrade for this tool.** Running an
  AppImage typically requires `libfuse2`/FUSE support on the host (or
  `--appimage-extract` as a workaround) — a dependency the existing raw
  binary and `.deb`/`.rpm` artifacts don't have. For a CLI-only static
  binary, that's added friction, not less.

Net: AppImage's main differentiator is redundant with the existing
`binary` archive artifact, and the packaging story it would add is more
complex (external tooling, no goreleaser pipe integration, FUSE runtime
dependency) for no concrete benefit to `harnez` users. Closing without
implementation. No `.goreleaser.yaml` changes were made.

If a concrete future need appears (e.g. a user request, or `harnez`
growing GUI-adjacent assets that actually benefit from AppImage's desktop
integration), reopen this ticket rather than resurrecting it from
scratch — the research above (schema check, tooling absence, `binary`
archive comparison) still applies.

Per the ticket's own note: the install-snippet question from issue 092
("how should projects declare which package types appear in their install
snippet?") should fold back into 092 covering `.deb`/`.rpm` only (from
098) — AppImage is not part of that matrix now that this ticket is
closed.

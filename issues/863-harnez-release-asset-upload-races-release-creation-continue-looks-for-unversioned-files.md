# 863 — harnez release: asset upload races release creation, --continue looks for unversioned files

**Status**: Open
**Priority**: P2 (Medium) — every release can stop half-published and the documented resume path fails
**Severity**: Major
**Category**: Bug
**Related**: docs/GoRelease.md

---

/goal `harnez release` publishes all assets in one run, and `harnez release --continue` finishes a
half-published release by uploading only the missing assets with their real `dist/` names. Verified with a
test against a fake `fj` (release not yet visible on first upload; release with some assets present), or
stop and report when blocked.

## Findings (v0.2.0 release, 2026-10-22)

1. `harnez release -b minor` tagged and pushed v0.2.0, created the release, uploaded `SHA256SUMS`, then failed:
   `fj upload SHA256SUMS.minisig: exit status 1 (Error: could not find release harnez v0.2.0)`. A second
   later `fj release list` showed the release, so the upload raced its creation (no retry).
2. `harnez release --continue` then failed with
   `open /home/uwe/projects/harnez/dist/harnez-aarch64-linux: no such file or directory`; `dist/` holds
   `harnez-0.2.0-aarch64-linux.{deb,rpm,tar.gz}`, so the resume path builds unversioned names.
3. Workaround used: `fj release asset create "harnez v0.2.0" dist/<file> <file>` for the 7 missing files;
   Codeberg now lists all 8.

## Way forward

- Retry uploads that fail with "could not find release" (bounded, values in spec).
- `--continue`: read the asset list from the forge, upload only missing files, with names from the
  same source the first run uses (`dist/artifacts.json`).

## Delivered: `0a4d4d80` (retry + manifest names); host review found an asset-set regression

- Retry and spec (`spec/release.yaml`) fine; release tests pass.
- `harnez release --continue -n --skip-build` (v0.2.0) lists 10 assets: the 8 published ones plus the bare
  binaries `harnez-aarch64-linux`, `harnez-x86_64-linux` (from `dist/harnez-linux_linux_*/harnez`). The
  first run never uploads bare binaries.
- `harnez release -n --skip-build` with a stale 0.2.0 `dist/` would publish v0.2.1 with only those 2 bare
  binaries (previously: "no release artifacts; publishing without attachments").

### Pre-Work / Required Refinements

1. One function decides the asset set, used by both the first run and `--continue`: versioned archives and
   packages for the release version, `SHA256SUMS`, `SHA256SUMS.minisig`; never bare binaries. Test: both
   paths yield the same 8 names for the v0.2.0 `dist/` layout.
2. Artifacts not matching the release version are never uploaded (stale `dist/`). Test it.
3. Rerun `make test-q1` after the last change (the previous run predates the final edit).
4. Note in the ticket, not code: `SHA256SUMS` lists the bare binaries although they are not attached;
   say whether that is intended or file nothing and leave it as a remark.

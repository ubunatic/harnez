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

### Refinements delivered

1. First publish and `--continue` share one resolver for versioned archives/packages plus
   `SHA256SUMS` and its signature. A test confirms both paths list the same eight v0.2.0 names
   and exclude the bare binaries.
2. The resolver excludes artifacts for other versions. Tests cover stale versioned artifacts;
   `harnez release -n --skip-build` with the stale v0.2.0 dist previews v0.2.1 with no assets.
3. `make test-q1` passes after the final changes; `grep -- '--- FAIL'` returns no matches.
4. The checksum coverage note below is intentional and remains a ticket remark.

Remark: This is intentional. GoReleaser's `SHA256SUMS` covers every built artifact, including the
bare binaries; the published release asset set contains only versioned archives/packages and the
checksum files.

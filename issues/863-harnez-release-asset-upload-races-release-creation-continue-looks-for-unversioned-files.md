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

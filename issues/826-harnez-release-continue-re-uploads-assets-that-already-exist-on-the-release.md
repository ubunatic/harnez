# 826 — harnez release --continue re-uploads assets that already exist on the release

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug

## Problem

2026-10-20, v0.1.32: `harnez release -b patch` tagged and pushed, then fj publish failed with
`api error: token was already used` (expired fj login), printed as a Cobra usage dump. After a
fresh `fj auth login` the release was published twice (user ran it, host ran `--continue`):
the Codeberg release got 16 assets, two of each name, and rebuilt .deb/.tar.gz differed by a few
bytes, so one SHA256SUMS did not match half the files. Host fix: delete all, re-upload the local
signed dist set.

## Expected

- `--continue` (and a repeated publish) checks existing release assets: skip identical names,
  or replace them, never add duplicates.
- Do not rebuild artifacts when a signed dist set for this version already exists.
- A forge auth failure prints a short hint (`fj auth login`), not the usage text.

# 826 — harnez release --continue re-uploads assets that already exist on the release

**Status**: Closed — Fixed in d3799b83: verified signed dist reuse, existing-asset skipping with published checksum validation, and concise runtime/auth errors; lint, touched-package tests and test-short pass.
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


## Delivery

- Implemented in `d3799b83`; recovery docs and release skill source updated in `d7f97014`.
- Publishing lists fj release assets before upload, prints `exists, skipped` for existing
  names, and checks local archives against published SHA256SUMS before any mutation.
  Checksum conflicts or existing assets without a manifest stop for manual recovery;
  remote assets are never deleted automatically.
- Continue reuses a complete version-matched signed dist set, skipping build and signing.
  Missing signatures/archives require rebuilding; corrupt archives fail clearly.
- Release runtime errors suppress Cobra usage; token/auth errors suggest `fj auth login`.
- Fake-backed tests cover absent/partial/complete remote sets, published and local hash
  conflicts, auth/network failures, missing published checksums, and complete/partial dist.
  A command test verifies runtime errors omit usage. Existing help goldens pass unchanged.
- Verified: `make lint`, `go test ./internal/release ./cmd/harnez`, `make test-short`, and
  `make install` all passed. Initial red tests failed on the two unimplemented helpers.
- No real release, Codeberg access, push, or delegation was performed.

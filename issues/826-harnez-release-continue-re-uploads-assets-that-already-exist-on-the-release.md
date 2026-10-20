# 826 — harnez release --continue re-uploads assets that already exist on the release

**Status**: Closed — Fixed in 4ade5768: completeDist resolves GoReleaser raw binaries through artifacts.json; the real v0.1.32 dist passed a read-only dry check, and lint plus all requested tests passed.
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

## M2 Pre-Work / Required Refinements (host review 2026-10-20)

- Real harnez dist fails `completeDist`: SHA256SUMS lists `harnez-x86_64-linux` and
  `harnez-aarch64-linux`, which goreleaser keeps in `dist/harnez-linux_linux_amd64_v1/` and
  `dist/harnez-linux_linux_arm64_v8.0/`, not at `dist/<name>`. Result: `--continue` always rebuilds,
  so the 826 duplicate-bytes problem stays. Resolve listed names via goreleaser's
  `dist/artifacts.json` (path per name), or verify only the files that get uploaded; keep missing
  minisig/archives -> rebuild. Add a test fixture shaped like the real dist (binaries in subdirs).
- Verify against the real `dist/` (read-only, no publish): add a dry check or test helper and show
  `completeDist` returns true for the current v0.1.32 dist.

## M2 Delivery

- Fixed in `4ade5768`: checksum names resolve through `dist/artifacts.json` to the actual
  GoReleaser paths, including raw binaries under platform subdirectories. Manifest paths are
  constrained to the project directory; missing signatures or files still trigger rebuilding.
- Added a real-shaped fixture with two nested raw binaries, a versioned archive, checksum
  manifest, signature, and artifact path manifest. The test confirms the set is complete.
- Read-only check of the current v0.1.32 distribution passed:
  `./harnez release --dir . --continue --dry-run --skip-sign --skip-publish --skip-push`
  reported `Complete signed dist for 0.1.32 exists, skipped build and signing`.
- Passed: `make lint`, `go test ./internal/release ./cmd/harnez`, `make test-short`, and
  `make install`. No release or remote publish was run.

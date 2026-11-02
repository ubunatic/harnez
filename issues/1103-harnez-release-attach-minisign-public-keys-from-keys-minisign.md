# 1103 — harnez release: attach minisign public keys from keys/minisign/

**Status**: Closed — key index validation, signer check, minisign-<KEYID>.pub assets and signer in notes (b37de868); reviewed; go test ./... green
**Priority**: P2 (Medium)
**Severity**: Feature
**Category**: Release Engine
**Related**: [internal/release/minisign.go](../internal/release/minisign.go), [issues/314](314-release-auto-create-minisign-key-if-missing.md)

---

## Problem

`harnez release` attaches archives, `SHA256SUMS` and `SHA256SUMS.minisig`, but no
public key, so users cannot easily verify a release (seen on lmcoder v0.1.15, key
`6C9CA9E043C23FEB`, two maintainers).

## Contract (agreed with sitegen and lmcoder, confirmed by the owner)

The project repo holds the source of truth:

- `keys/minisign/<KEYID>.pub`: one standard minisign public key file per key, kept unchanged.
- `keys/minisign/keys.yaml`: it starts with `schema: 1`, then `keys:`, a list of
  `{id, maintainer, status: active|retired, added: YYYY-MM-DD, retired?: YYYY-MM-DD, note?}`.
  A key is added or retired by a commit to this file, so each rotation is visible.

`harnez release`:

- attaches each active key as the asset `minisign-<KEYID>.pub`, with its bytes unchanged;
- states `SHA256SUMS signed by <KEYID>` in the release notes;
- refuses to release when `keys.yaml` is invalid, when a listed `.pub` is missing, when
  a `.pub` key ID differs from its entry, or when the signing key is not `active`;
- only warns, and releases as it does today, when `keys/minisign/keys.yaml` does not exist.

sitegen (separate ticket) serves the same files at `/tools/<tool>/keys/` (a
single-project site uses `<site>/keys/`), with a page listing every key and its history.

Put the paths and asset name pattern in `spec/release.yaml`, not as Go defaults.

/goal Implement the contract above with tests (valid list, missing .pub, ID mismatch,
retired signing key, no keys.yaml only warns) and verify it with a dry-run release. Stop and
report if blocked on a user decision or a denied permission; on a loop or no progress, clear
the goal and write a failure report.

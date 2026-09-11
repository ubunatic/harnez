# 314 — harnez release: auto-create minisign key if missing and no repo key defined

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Feature
**Category**: Release Engine
**Related**: [internal/release/minisign.go](../internal/release/minisign.go), [issues/284](284-harnez-release-build-step-defaults-to-gowork-off-with-allow-workspace-override.md)

---

## 1. Problem & Motivation

When running `harnez release` on a newly onboarded or existing repository without an explicit `--sign-key` or existing project key, `ResolveMinisignKey()` currently falls back to `~/.minisign/minisign.key`. If that global key is password-protected, the non-interactive key check fails with:

```
Error: minisign key test failed: minisign signing failed: exit status 2
Output: Password: get_password()
```

If neither `~/.minisign/<project>.key`, a repo-local key (`.minisign.key`, `minisign.key`, `<project>.key`), nor `~/.minisign/minisign.key` exists (or the global key is unusable/encrypted), `harnez release` fails hard and requires the user to manually discover and run `minisign -G -W -s ~/.minisign/<project>.key -p ~/.minisign/<project>.pub`.

`harnez release` aims to provide an automated, low-friction release workflow. When no project key or repo-local key is defined, it should offer or automatically create a new unencrypted project key pair at `~/.minisign/<project>.key` (and `~/.minisign/<project>.pub`) using `minisign -G -W`.

## 2. Proposed Behavior

1. **Resolution & Auto-creation Flow**:
   - Check explicit `--sign-key` first.
   - Check `~/.minisign/<project>.key`.
   - Check local repo key files (`.minisign.key`, `minisign.key`, etc.).
   - If no project or repo key is found:
     - Before falling back to global `~/.minisign/minisign.key` (or if global key is password-encrypted / absent), auto-generate a new unencrypted key pair at `~/.minisign/<project>.key` and `~/.minisign/<project>.pub` via `minisign -G -W -s ... -p ...`.
     - Log creation clearly: `  [key]       Generated new minisign keypair at ~/.minisign/<project>.key`
     - Proceed with release signing using the newly created key.

2. **Safety & Non-Destructive Behavior**:
   - Never overwrite an existing key file.
   - Create `~/.minisign/` directory with `0700` permissions if it does not exist.
   - Respect `--dry-run` by reporting that a key would be generated without actually invoking `minisign -G`.

## 3. Acceptance Criteria

- `harnez release` automatically creates `~/.minisign/<project>.key` and `~/.minisign/<project>.pub` if missing and no repo-local key exists.
- Non-interactive signing succeeds seamlessly on fresh repos without manual `minisign` setup.
- `--dry-run` reports the intended key generation cleanly without modifying `~/.minisign/`.
- Unit tests added to `internal/release/release_test.go` covering auto-generation and fallback resolution.

## 4. Verification

- `go test ./internal/release/...`
- Test in a clean environment without pre-existing keys: `harnez release --dry-run` and live release verification.

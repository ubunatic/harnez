# 1118 — harnez forge token: create, store and revoke a read-only Forgejo token

**Status**: In Progress — built, awaiting the owner's live create
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1116

---

/goal Give harnez a command that creates a read-only Forgejo token from the owner's password, stores
it safely and is read before CODEBERG_TOKEN/FORGEJO_TOKEN, with rotate and revoke, so the 1116 forge
read mode works on private repos (harnez/taia); or stop and report when blocked on a user decision.

## 1. Problem & Motivation
Owner request relayed by taia (2026-11-04): forge tracker mode (1116) and `pr wait` read private repos
only with a token, and none is set up. Creating one by hand and exporting env vars is error-prone.

## 2. Findings (verified against Codeberg's swagger, Forgejo 16, and forgejo.org docs)
- `POST /users/{user}/tokens` needs basic auth with the password, never a token; 2FA adds
  `X-Forgejo-OTP`. Body `{name, scopes}`; response 201 carries the secret once in `sha1`.
- `DELETE /users/{user}/tokens/{token}` takes the id (falls back to name); 204 on success.
- Scope names are `read:<group>`. Docs leave open whether `/repos/{o}/{r}/issues` needs `read:issue`
  alone or also `read:repository`, so the token gets both.

## 3. Result
- `harnez forge token create|rotate|revoke|status [--host codeberg.org]` (`cmd/harnez/forge_token.go`,
  `internal/forgetoken`). Password and 2FA code come only from `/dev/tty` (no echo for the password);
  no flag or env var; errors never contain them.
- Settings in `spec/forge_token.yaml`; its schema allows only `read:*` scopes. Name
  `harnez-read-<host>-<YYYYMMDD-HHMM>`.
- Storage: OS keyring via `secret-tool` when it works, else `$XDG_CONFIG_HOME/harnez/forge-tokens/<host>.token`
  (0600, dir 0700). Metadata (name, id, user, scopes, store; no secret) in `<host>.json`.
- `forge.TokenSource` reads the stored token first for Forgejo hosts, then the env vars.
- Rotate stores the new token, then revokes the old one by id; a failed revoke keeps the new one and
  names the old one. A failed local store revokes the just-created token again.
- Tests: `internal/forgetoken/forgetoken_test.go` (fake forge + fake keyring), `TestTokenSourceStoredFirst`.

## 4. Open
- Live check: the owner runs `harnez forge token create` once, then `harnez issues show` on taia.

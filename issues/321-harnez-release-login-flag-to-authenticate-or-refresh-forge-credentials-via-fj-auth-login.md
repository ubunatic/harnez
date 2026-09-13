# 321 — harnez release --login flag to authenticate or refresh forge credentials via fj auth login

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [issues/096](096-release-forge-token-401-warning-lacks-fix-hint.md), `cmd/harnez/release.go`, `internal/release/runner.go`, `internal/release/forge.go`, `docs/GoRelease.md`

---

## 1. Problem & Motivation

When executing `harnez release`, publishing to Codeberg / Forgejo instances requires a valid API token in `~/.local/share/forgejo-cli/keys.json` (or environment variables).

When that token is expired or missing:
1. The preflight `has_releases` check outputs a 401 warning or fails.
2. The subsequent `fj release publish` step fails with an API error (e.g. `token was already used` or invalid claims).

Currently, users must know how to invoke `fj auth login` directly in their shell to refresh their credentials. Providing a first-class `--login` flag on `harnez release` lowers friction and allows users (and assisted environments) to trigger the interactive forge login process directly from `harnez`.

## 2. Technical Specification / Design

1. **CLI Flag**:
   - Add `--login` to `harnez release` in `cmd/harnez/release.go` (`opt.Login`).
   - Flag description: `authenticate or refresh forge credentials via 'fj auth login' before release`.

2. **Login Execution**:
   - In `internal/release/forge.go`, implement `RunForgeLogin(host string) error`:
     - Discovers the forge host from the git remote origin (defaulting to `codeberg.org` or configured forge host).
     - Spawns `fj auth login` (or `fj auth login <host>`) with `cmd.Stdin = os.Stdin`, `cmd.Stdout = os.Stdout`, `cmd.Stderr = os.Stderr` so interactive OAuth browser launch and fallback URL prompts work smoothly.
3. **Control Flow**:
   - If `harnez release --login` is invoked standalone (no version bump or continuation requested), complete the login process, verify the refreshed token, and exit 0.
   - If `--login` is combined with release operations (e.g. `harnez release --login --bump patch`), run the login step before preflight checks and artifact build/publish.

4. **Documentation**:
   - Update `docs/GoRelease.md`, `README.md`, and `docs/CLIDesign.md` to document `harnez release --login`.

## 3. Implementation & Verification Plan

1. Add `Login bool` field to `release.Options` in `internal/release/runner.go`.
2. Register `--login` flag in `cmd/harnez/release.go`.
3. Implement `RunForgeLogin` in `internal/release/forge.go` and hook into `Runner.Run()`.
4. Add unit and CLI tests in `cmd/harnez/` and `internal/release/`.
5. Update docs and test with dry-run/mock commands.

## 4. Acceptance Criteria

- [ ] `harnez release` accepts `--login` flag.
- [ ] Running `harnez release --login` executes `fj auth login` with interactive stdin/stdout/stderr attached.
- [ ] Standalone `harnez release --login` exits 0 after authentication without running build or publishing.
- [ ] Documentation updated across `docs/GoRelease.md` and `README.md`.

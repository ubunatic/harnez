# Language-agnostic release spec — replace per-project release Make target sprawl

**Status:** Closed — resolved in implementation
**Severity:** N/A (feature/DX proposal, not a bug)
**Category:** Architecture / Tooling

## Problem

Every sibling project (`spriteview`, `emojig`, `wayreel`, ...) that publishes
releases has independently accreted a large, near-duplicated set of Make
targets: `deps`, `test-minisign`, `release-build`, `release-snapshot`,
`info`, `release-diff`, `release-publish`, `release`, `release-full`, plus
(in emojig) `tag`, `bump-patch`, `bump-minor`, `bump-major`. The
`.goreleaser.yaml` `signs:` block and much of the Makefile skeleton were
copy-pasted verbatim between projects.

## Implementation & Resolution

Implemented built-in `harnez release` command and language-agnostic `version.yaml` spec architecture:

1. **`version.yaml` Spec as Single Source of Truth**:
   - Manages version definitions adhering to `docs/Spec.md`.
   - Supports semver strings, structured components (`major`, `minor`, `patch`, `prerelease`, `build`), and explicit file mappings.
   - Embeds template in `docs/templates/version.yaml` and auto-scaffolds via `harnez init` or release auto-detection.

2. **Language Code Sync & Generation (`internal/release`)**:
   - **Go**: Auto-detects/updates `version.go` (`var Version = "X.Y.Z"`).
   - **Python**: Updates `__version__.py`, `version.py`, and `pyproject.toml`.
   - **Zig**: Updates `build.zig.zon` (`.version = "X.Y.Z"`) and `version.zig`.
   - **Rust**: Updates `Cargo.toml` (`version = "X.Y.Z"`).

3. **Built-in `harnez release` Subcommand**:
   - **Preflight checks**: Toolchain verification (`git`, `goreleaser`, `minisign`, `fj`).
   - **Passwordless Minisign**: Project key discovery (`~/.minisign/<project>.key`, `.minisign.key`, or `--sign-key`) with non-interactive signing.
   - **Forgejo / Codeberg API Auto-Enable**: Detects git remote, checks repository API `GET /api/v1/repos/{owner}/{repo}`, and auto-enables `has_releases` via API if disabled.
   - **Git Tag & Push Automation**: Automates version commit, tag creation, branch push, and tag push.
   - **Safe Idempotency (`--continue`)**: Safely recovers from interrupted network/forge uploads without moving or deleting tags.
   - **Dry Run (`--dry-run`)**: Previews the entire release pipeline without modifying git, files, or remote forge.

4. **Thin Makefile Convention**:
   - Standardizes `release:` recipe across projects to:
     ```makefile
     release: check ⚙️  # release the project using harnez
     	harnez release
     ```

## References

- `spriteview/docs/studies/2026-08-28-releasing-a-pure-python-project-with-goreleaser.md`
- `docs/Spec.md`
- `docs/practices/GoRelease.md`

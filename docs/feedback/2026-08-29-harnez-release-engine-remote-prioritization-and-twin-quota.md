<!-- SPDX-FileCopyrightText: 2026 Uwe Jugel -->
<!-- SPDX-License-Identifier: CC-BY-4.0 -->

# Session Retrospective: Harnez Release Engine, Non-Interactive Signing & Twin-Quota Alignment

*Date: 2026-08-29*
*Status: Recorded*

---

## 1. Key Accomplishments & Architectural Decisions

### A. Aligned Twin-Quota Layout Across Agents (`harnez usage`)
- **Visual Pattern**: Replaced staggered quota window lines with a compact twin-quota batch layout:
  ```text
  Gemini     [███░] 92% [░░░░] 3%
  Claude/GPT [█░░░] 35% [░░░░] 0%
  Wk / 5h    [███░] 83% [░░░░] 13%
  ```
- **Label Fixed Alignment**: Standardized 10-character width for model and agent labels (`Gemini`, `Claude/GPT`, `Wk / 5h`) so progress bars align vertically without jumping.

### B. Built-in, Spec-Driven `harnez release` Command (Issue 091)
- Replaced 600+ lines of per-project Makefile boilerplate across sibling repositories with a unified `harnez release` command.
- **`version.yaml` as Single Source of Truth**: Semver is read, bumped (`patch` by default, `--bump minor/major/x.y.z`), and automatically synced into language-native files:
  - Go: `version.go` (`var Version = "X.Y.Z"`)
  - Python: `version.py`, `__version__.py`, `pyproject.toml`
  - Zig: `build.zig.zon` (`.version = "X.Y.Z"`), `version.zig`
  - Rust: `Cargo.toml` (`version = "X.Y.Z"`)
- **Non-Interactive Minisign**: Unencrypted keypairs (`minisign -G -W`) in `~/.minisign/<project>.key` eliminate blocking tty prompts during autonomous agent execution.
- **Forge API Preflight**: Automatically queries Codeberg/Forgejo REST API to verify and enable `has_releases: true` if disabled on newly created repositories.
- **Idempotency & Resumption**: Added `--continue` to recover safely from forge upload hiccups without re-bumping or moving existing git tags.

### C. URL-Based Forge Remote Prioritization Cascade
- Upgraded remote detection from name-only (`origin`) to a 3-tier URL-based priority cascade:
  1. `origin` if targeting `codeberg.org` or `github.com`.
  2. Any remote targeting `codeberg.org` (e.g. `codeberg`, `upstream`, `forgejo`).
  3. Any remote targeting `github.com` (e.g. `github`, `mirror`).
  4. Preflight failure if no supported public forge remote exists (preventing accidental releases against local mirrors or internal remotes).
- Tested and verified on `lmcoder` (where `origin` is a local SSH host mirror `uwe@um760:projects/lmcoder` and `codeberg` is the release target).

---

## 2. Practical Learnings & Pitfalls Discovered

1. **Subagent Handoff with Minisign**: Standard `minisign -G` encrypts the secret key with an interactive password prompt. Autonomous subagents will hang or abort without `-W` (unencrypted key pair).
2. **Codeberg `404 Not Found` Target**: When `has_releases` is toggled off on Codeberg repo creation, API release endpoints return 404. Auto-enabling via `PATCH /api/v1/repos/{owner}/{repo}` with `{"has_releases": true}` resolves this permanently.
3. **Multi-Remote Topology**: Repositories synced across local test hosts often use `origin` for local sync. The release tooling must select the forge remote by URL inspection rather than assuming `origin` is the forge.

---

## 3. Filed Tickets & Follow-ups

- **[Issue 092](issues/092-latest-release-links-and-readme-install-section-consolidation.md)**: Explore consolidating latest release links (`codeberg.org/ubunatic/<project>/releases/latest`) and standardizing README/website install sections across projects.

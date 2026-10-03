# 698 — teach agents when and how to release repos (harnez release guidance and rules)

**Status**: Open
**Priority**: P1 (High)
**Severity**: Normal
**Category**: Feature
**Related**: `docs/GoRelease.md`, `docs/AgenticLoop.md`, `.harnez/rules/Tools.md`, `issues/091-language-agnostic-release-spec-and-thin-make-release.md`, `issues/246-add-commit-and-publish-skills-for-staged-commit-ownership-and-multi-project-release-publish-workflows.md`

---

## 1. Problem & Motivation

Autonomous coding agents (orchestrators and developers) frequently complete features, fix bugs, or deliver whole sprints, but either:
1. **Don't know when to trigger a release**: They close tickets and leave commits unreleased on `main` without bumping semver or publishing artifacts, requiring explicit human prompting.
2. **Don't know how to release properly**: They attempt custom bash git tagging, manual `goreleaser` invocations, or ad-hoc file edits instead of using `harnez release`.
3. **Don't choose the right semver bump**: Confusion around patch (`0.x.Y+1`) vs minor (`0.X+1.0` or `X.Y+1.0` for new features/widgets) vs major bumps.

We need clear rules and guidance embedded in Harnez prompts, rules (`.harnez/rules/`), and evergreen documentation (`docs/GoRelease.md`, `docs/AgenticLoop.md`) that teach agents the exact release criteria, lifecycle triggers, and command usage.

## 2. Technical Specification & Guidance

### A. When an Agent Should Release
- **Feature Completion / Milestone Sprints**: When a new feature, widget, or significant capability is completed, tested green, and ticket(s) are closed.
- **Breaking Changes or Upgrades**: When public APIs or module paths change (accompanied by `docs/Upgrading.md` updates).
- **Explicit Instruction**: When requested by the user or when a sprint/goal specifies release delivery upon green tests.
- **Pre-Conditions for Release**:
  - `git status` is clean (no unstaged/uncommitted files).
  - All tests and linters pass (`make test-q1` or `harnez exec --quota-1 -- make test`).
  - Spec validation passes (`cmd/validate-spec`).
  - Documentation and issue tickets are committed and indexed.

### B. How an Agent Should Release
Agents must use the standardized release CLI:
```bash
# For feature additions, new widgets, or major capability additions:
harnez release --bump minor

# For bug fixes, doc updates, refactors, or small tweaks:
harnez release --bump patch

# When breaking public APIs:
harnez release --bump major
```

### C. What `harnez release` Automates Under the Hood
1. Reads and bumps single-source-of-truth `version.yaml`.
2. Synchronizes language-specific files (`version.go`, `__version__.py`, `build.zig.zon`, etc.).
3. Runs GoReleaser / build commands with `GOWORK=off` (preventing untagged local module pollution).
4. Signs checksums with Minisign (`~/.minisign/<project>.key`).
5. Ensures forge repository release settings (`has_releases` API).
6. Tags git commit and pushes branch and tag to remote forge (`origin/main`, `vX.Y.Z`).
7. Uploads built binaries, archives, and signed checksums to Codeberg/Forgejo via `fj release`.

## 3. Implementation & Verification Plan

- **M1 (Rule & Prompt Updates)**:
  - Add release lifecycle guidance to `.harnez/rules/Tools.md` and `.harnez/prompts/developer.md` / orchestrator instructions.
  - Clarify bump strategy: patch for bug fixes, minor for features/widgets, major for breaking changes.
- **M2 (Evergreen Documentation)**:
  - Update `docs/GoRelease.md` with an "Agentic Release Guide" section detailing pre-flight checks, command syntax, and post-release validation.
  - Cross-reference from `docs/AgenticLoop.md` under the Hygiene/Closeout phase.
- **M3 (Verification & Release)**:
  - Verify rule synchronization via `harnez rules sync` / `harnez index`.
  - Validate that agents running `/lean-sprint` or `/release` properly identify and execute release steps.

# 935 — Add optional container test, CI (default GH), and first release plan to new-project skill

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Enhancement
**Related**: 899, 902

---

/goal Update the new-project skill (`docs/commands/new-project.md`) to include optional interview questions and bootstrap tasks for container tests (including cross-arch verification), CI setup (defaulting to GitHub Actions), and a planned first release milestone; verify with test and docs lint, or stop and report when blocked on user input.

## 1. Problem & Motivation

When bootstrapping a new project with `/new-project`, the initial interview in step 1 and the bootstrap epic tasks in step 8 currently omit three common project needs:
1. **Container Tests**: Projects (especially those with cross-compilation, systemd, or cross-arch execution) often need containerized integration/smoke tests (e.g. rootless Podman/Docker running cross-arch images).
2. **CI Pipeline**: Automated CI (typically GitHub Actions) to run tests, linters, and license checks on push/PR.
3. **First Release Plan**: Explicit planning for the initial tag/release milestone once the first epic or walking skeleton is completed.

Adding these as optional prompts during the interview and wiring them into the bootstrap epic template ensures new projects get consistent testing, CI, and release planning from day one.

## 2. Technical Specification / Findings

In `docs/commands/new-project.md`:
- **Step 1: Interview**:
  - Add optional CI prompt: `CI pipeline (default: GitHub Actions, Codeberg Actions, or none)`.
  - Add optional Container tests prompt: `Container tests (e.g. cross-arch smoke tests via Podman/Docker)`.
  - Add optional First release prompt: `First release milestone plan (e.g. target tag v0.1.0 after initial epic)`.
- **Step 8: Bootstrap Epic Tasks**:
  - Include tasks for CI workflow scaffolding (e.g. `.github/workflows/ci.yaml`), container test harness, and first release planning when requested during the interview.

## 3. Implementation & Verification Plan

1. Update `docs/commands/new-project.md` with the new optional prompts in the interview and the corresponding epic task items.
2. Verify documentation formatting and references via `make test` or `harnez docs` verification.

# 371 — Add optional systemd service guidance to Go init profile

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: `docs/studies/2026-09-16-ramp-levels-and-go-repository-init.md`, [[369-detect-go-library-cli-and-tui-shape-for-init-guidance]], [[370-offer-useful-go-agent-capabilities-through-harnez-init]]

---

## 1. Problem & Motivation

Some Go CLI applications also run as systemd services. The main Go library/CLI/TUI MVP should not assume service deployment, but service-bearing projects need agent guidance for lifecycle and operational verification.

## 2. Scope & Design

- Detect checked-in `.service` units and Go service lifecycle code as *capability evidence*, not as a separate exclusive project type or a RAMP artifact. Ask for an explicit profile choice when evidence is ambiguous.
- Offer opt-in project-local L2 guidance for startup/shutdown, signal handling, configuration and state paths, logging, unit files and safe local verification. Point to actual project files and commands; do not generate host-specific paths or credentials.
- If the L3 Go capability from 370 is installed, add only the service-specific checks needed for a real change/review task. Reuse existing deployment-transparency guidance where applicable.
- `init` must never invoke `systemctl`, install a unit, enable/start a daemon, or present local build success as proof of active service state.

## 3. Exit Criteria

- [ ] A service-bearing Go CLI fixture receives relevant guidance; an ordinary CLI does not.
- [ ] Service metadata alone does not raise the reported RAMP level.
- [ ] Preview, content preservation and second-run idempotency match the Go MVP behavior.
- [ ] A pilot on a real service project confirms instructions reflect its actual unit and lifecycle.

## 4. Verification

Use fixtures and read-only inspection of a real service repo. Run the project test target once after source edits under Quota-1; no verification step should start or alter a live service.

# 716 — Install Harnez skills and hooks into Pi instances

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [#712 Pi CLI driver](712-add-support-for-the-pi-coding-agent-cli-1-0.md), [#070 cross-agent hook support](070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode.md), [#631 external skills](631-external-skills-canary-then-user-registry-b-then-vendored-skills-a.md)

---

## 1. Problem & Motivation
The Pi driver in #712 lets `harnez agent` launch Pi, but it does not make the
user's Pi installation a Harnez-integrated agent. The intended Pi support is to
make Harnez-owned skills and supported hooks available when the user runs Pi
itself, including outside a Harnez-launched session.

Harnez currently has a partial Pi hook integration: a generated Distill
extension can target `~/.pi/agent/extensions/harnez-distill.ts`. However, the
general skill target list does not include Pi, and there is no complete,
user-facing lifecycle for installing and updating Harnez's Pi-compatible
skills and hooks. The Pi CLI driver and Pi-instance integration are separate
features; closing #712 covered the former only.

**Goal**: Install and maintain Harnez's Pi-compatible skills and hooks in the
user's Pi instance, or stop and report if Pi's supported extension/skill
surfaces cannot safely provide the required integration.

## 2. Technical Specification / Findings
- Pi 1.0+ supports user-level skills and extensions; confirm the exact current
  discovery paths and hook APIs against Pi's primary documentation during
  implementation.
- Harnez generates a Pi Distill extension from
  `internal/claude/distill_adapters.go`, with a configurable target in
  `config.yaml`; this is one adapter, not the full Harnez hook set.
- `internal/claude/apply.go:SkillTargetsByAgent` currently enumerates Gemini,
  Codex, Claude, and Prime skill targets, but not Pi.
- Preserve user-owned Pi files and support the user's configured Pi home
  (`PI_CODING_AGENT_DIR`, or an explicit Harnez setting/target). Keep generated
  and managed files identifiable and safe to update or remove.
- This issue is Pi-instance provisioning, not another provider/model driver and
  not a request to enumerate Pi model IDs in `harnez agent models`.

## 3. Implementation & Verification Plan
- Define the Pi integration surface for Harnez-owned skills and hooks. Include
  every compatible Harnez skill; explicitly report any skill/hook that is not
  compatible rather than silently omitting it.
- Add an explicit, idempotent install/update/status/removal lifecycle for
  Pi-managed files. Choose a CLI surface that respects the existing separation
  between global `apply` and project-local `init`; read `docs/CLIDesign.md`
  before changing `apply` flags or scope.
- Add tests for target resolution (including `PI_CODING_AGENT_DIR`), complete
  skill coverage, managed-file updates, idempotency, and preservation of
  user-owned files.
- Verify with the current Pi CLI that installed skills are discoverable and
  usable, and that installed hooks/extensions actually run. Include an isolated
  live canary and update Pi integration documentation.

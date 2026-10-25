# 953 — Interactive Codex sessions are treated as Harnez leaf agents

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: #486, #581, #665, #470

---

## /goal

Make a normal interactive Codex session usable as a Harnez host, with host operations available and leaf-worker restrictions preserved for spawned agents; verify the self-test and agent dispatch from that host, or stop and report if the active host sandbox policy prevents the required behavior.

## 1. Problem & Motivation

This interactive Codex session could not use Harnez as a normal host. Harnez exported `HARNEZ_AGENT_ROLE=advisor` into the interactive session. `harnez agent selftest --step hello` and attempts to start an agent were rejected with “agent role advisor is a leaf worker,” even though the user was chatting with the top-level interactive host and expected it to act as orchestrator.

The same session had related sandbox friction: `harnez rate` could not write its session record under `~/.harnez/sessions`, and adding `/home/uwe/.cache/go-build` to `~/.codex/config.toml` did not change the active managed permission roots. Gokay lint failed package loading with the default Go cache but worked with `GOCACHE=/tmp/...`.

This leaves ordinary Codex + Harnez workflows confusing and unreliable: host operations are blocked as if the host were a leaf, and filesystem restrictions are not explained by the user-level Codex configuration.

## 2. Technical Specification / Findings

- `env` showed `HARNEZ_AGENT_ROLE=advisor` and `CODEX_PERMISSION_PROFILE=:workspace` in the interactive session.
- `harnez agent list` showed the Codex session as active with source `vscode`.
- `harnez agent selftest --step hello` failed immediately with the advisor leaf-worker refusal.
- `harnez agent start` was refused with the same leaf-worker message, even when `--role orchestrator` was supplied.
- `harnez rate` failed to write `/home/uwe/.harnez/sessions/2322f1b00adbf2d2.ticket` because the filesystem was read-only.
- The configured writable root `/home/uwe/.cache/go-build` remained unwritable after resuming; setting `GOCACHE=/tmp/gokay-sandbox-gocache` allowed Gokay lint to complete and report a source finding.

Investigate whether the host role environment is incorrectly inherited or set by the interactive Codex integration, and clarify which filesystem restrictions are controlled by Harnez versus the host-managed Codex permission profile.

## 3. Implementation & Verification Plan

- Confirm how interactive Codex hosts are identified and why this session receives the `advisor` leaf role.
- Ensure ordinary interactive hosts can run Harnez self-tests and dispatch agents, while spawned leaf workers remain unable to manage agents.
- Provide actionable behavior or documentation when the active Codex sandbox makes Harnez session state or configured cache paths read-only.
- Verify the flow from an interactive Codex host without overriding `HARNEZ_AGENT_ROLE` manually.

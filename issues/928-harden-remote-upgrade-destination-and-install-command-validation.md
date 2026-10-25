# 928 — Harden remote upgrade destination and install command validation

**Status**: Closed — 70072b79: rejects unsafe SSH destinations and install paths; touched packages and make test-q1 passed, make install completed
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: [920](920-usage-server-collects-remote-load-over-ssh-slow-polling-without-viewers.md), [Epic 921](921-epic-harnez-server-as-monitor-for-local-and-remote-ai-hosts.md), 919

---

## 1. Problem & Motivation
/goal Validate upgrade destinations and shell operands before executing any subprocess; prove malformed hosts and paths cannot become SSH options or shell commands. Stop and report if safe remote path semantics require an owner decision.

Automatic server-start upgrades accept load.watch_host without the validation used by usage remote queries. A host beginning with - is passed directly to ssh and can become an option (including ProxyCommand), executing a local command rather than addressing a host.

## 2. Technical Specification / Findings
- internal/server/remote_upgrade.go:157-168 (before the concurrent 920 M3 refactor): only empty-host checking precedes appending host to ssh arguments. Apply a shared destination validator to both ssh and scp before any probe.
- internal/server/spec.go:89 rejects some metacharacters but permits newlines, spaces, parentheses and glob characters. The install path is interpolated unquoted into version and mkdir/chmod/mv shell commands. A path containing a newline passes validation and can execute another remote command; whitespace can split operands.
- These are separate from 920 M3's shared SSH options and failure logging fix. Config is local input; do not claim a network unauthenticated exploit.

## 3. Implementation & Verification Plan
- Define shared accepted host syntax, rejecting option-like destinations and controls before every upgrade subprocess.
- Define safe home-relative install paths; quote shell operands while preserving intentional home expansion, or reject unsupported syntax consistently in runtime and schema.
- Table-driven tests for leading-dash hosts, ProxyCommand payloads, newlines, whitespace and shell syntax; assert no subprocess for invalid inputs and exact arguments for valid aliases/user@host.
## Decision (host, 2026-10-25)

- One host validator in `internal/sshutil`, used by the upgrader and by usage remote/loadstream/history.
- Install path: `~/` plus safe ASCII components; reject traversal, whitespace, controls, shell syntax, globs
  (reject rather than quote; default `~/...` keeps working). Schema and runtime share the rule.

- Acceptance: probes, copy and install all use validated destinations and safe path handling; existing valid defaults work. Exit when tests pass, or report the owner decision needed.

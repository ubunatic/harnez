# 837 — Converge harnez server (HTTP/TCP) with the proposed harnez serve (Unix socket, JSON-RPC)

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Architecture / Decision
**Related**: 829, 830, 831, 776

---

/goal One documented server design: either the built `harnez server` (HTTP on 127.0.0.1 TCP, issues 829/830) and the proposal in `docs/HarnezServerArchitecture.md` (`harnez serve`, Unix socket, JSON-RPC) are converged in code, or the proposal is amended to match what was built, with the reasons recorded. Stop and report when blocked on a user decision or denied permission.

## 1. Problem & Motivation
The 829-831 sprints built `harnez server` without first reading the existing proposal
(`docs/HarnezServerArchitecture.md`, 2026-10-20). Command name, transport and protocol now differ.
Settle it before roadmap step 3 (the supervisor) builds on either one.

## 2. Technical Specification / Findings
- Built: `internal/server`, `spec/server.yaml` (127.0.0.1:18729), `GET /status`, `POST /usage/passive`,
  services with `Run` and optional `RegisterRoutes`; port bind + status probe is the singleton.
- Proposed: Unix socket (later TLS remote), one versioned JSON-RPC API behind adapters.
- Trade-offs to weigh: a Unix socket has file permissions as auth and no port clash, while TCP is
  reachable by any local user unless authenticated; HTTP is easy for hooks (curl) and tests.
- Uncertain: whether statusline/hook clients (830) need anything beyond HTTP POST.

## 3. Implementation & Verification Plan
- Decide (user) the command name, transport and protocol; record it in the architecture doc.
- If converging: keep the service interface, swap the transport, keep the 829/830 client tests green.

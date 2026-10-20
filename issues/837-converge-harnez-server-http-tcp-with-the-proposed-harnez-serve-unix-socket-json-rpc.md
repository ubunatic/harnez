# 837 — Converge harnez server (HTTP/TCP) with the proposed harnez serve (Unix socket, JSON-RPC)

**Status**: Closed — harnez server serves HTTP over a Unix socket; newest build takes over the running server
**Priority**: P1 (High)
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

### Decisions (2026-10-20)
- User: switch to a Unix socket now, before 830 M3 and 831 build on the TCP port.
- Host (delegated): keep HTTP routes, served over the Unix socket (`curl --unix-socket` still works,
  830 client tests keep their shape). JSON-RPC stays the target for the later agent-session API
  (roadmap step 4), not for these routes. Keep the command name `harnez server`; amend the doc.
- Socket path in spec, under `$XDG_RUNTIME_DIR/harnez/` (fallback state dir), dir `0700`, socket `0600`.
- Singleton: if the socket exists, probe `/status`; healthy harnez → "already running", exit 0;
  no answer → stale, remove and bind; another program → error. Drop the TCP port and its spec key.
- User: the latest version wins. When a healthy harnez server answers, compare versions: if the new
  binary is newer, ask the running server to shut down (authenticated by socket permissions, e.g.
  `POST /shutdown`), wait for the socket to go away within a spec timeout, then bind; if same or older,
  print "already running (vX)" and exit 0. `/status` must report a comparable version. Host decision:
  "newer" = higher semver; for equal semver compare build identity (binary mtime or embedded build
  time), so a fresh `make install` of the same version replaces the old process. Test both directions
  and the equal case. This also covers 535's "restart after upgrade" for clients that auto-start.
- Order: after 830 M2 lands; 830 M3 and 831 then use the socket.

### Delivered (31f04748)
Host check 23:08: developer edited tests after its single quota run; host `make test-q1` rerun green.
`harnez usage` auto-started the server on `/run/user/1000/harnez/server.sock` (0600, no TCP port);
`/status` reports `build_identity`; a second `harnez server` prints "already running (v0.1.33)", exit 0;
statusline update stored (row 278607).

## 3. Implementation & Verification Plan
- Decide (user) the command name, transport and protocol; record it in the architecture doc.
- If converging: keep the service interface, swap the transport, keep the 829/830 client tests green.

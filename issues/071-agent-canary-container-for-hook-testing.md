# 071 — Agent Canary Architecture: Separate `harnez` Container, `lmcoder` Hosts Server+Proxy Only

**Status**: Closed — architecture delivered; remaining scope tracked by [[073-agent-canary-cloud-credentialed-claude-agy-codex]]
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics & UI Standards
**Related**: [[070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode]], [[069-pretooluse-hook-optional-autopipe-bash-through-distill]], [[072-agent-canary-local-llm-pi-opencode]], [[073-agent-canary-cloud-credentialed-claude-agy-codex]], `docs/other/Canary.md`

---

## 1. Problem & Motivation

Testing a hook mechanism (issue 069's Claude Code `PreToolUse` rewrite, and issue 070's planned
Pi/OpenCode extensions) against a real agent CLI currently means installing that CLI on the host
and pointing a hook at live credentials — which we've decided against as a default. We want an
isolated, disposable way to verify "does this hook actually fire and rewrite the command" per
agent, without installing agent runtimes on the host or risking the host's real config.

**Host stays minimal by design**: only Claude Code, AGY (Antigravity), Codex, and Prime Agent are
installed locally, intentionally. Everything else (Pi, OpenCode, and any future agent CLI) should
run in a container instead.

## 2. Critical Finding: Most Of This Already Exists in `../lmcoder`

**Do not rebuild from scratch.** `~/projects/lmcoder/scripts/agent-canaries/` already has
per-agent `Containerfile`s (`claude-code`, `codex`, `opencode`, `pi`, `prime-agent`, plus a combined
`multi/Containerfile`), a Go canary runner (`main.go`/`run.sh`) implementing almost exactly the
ping-pong / hello-world / token-telemetry test protocol we want (and already accepting
`harnezArgs`/`harnezInit` to run `harnez init` in the isolated workspace before the agent runs),
and `lmcoder proxy` giving containers one stable local backend address instead of a hardcoded
hostname per tool.

## 3. Decision: Ownership Split

- **`lmcoder` hosts the LLM backend only**: `lmcoder start --model <small>` for the local
  `llama-server`, and a **second** `lmcoder proxy --listen-port <new port, distinct from the
  existing 8735 default>` instance dedicated to this use case, so it doesn't collide with any
  proxy already pointed at another machine (e.g. `um760`). This is not a workaround — `lmcoder`'s
  own `proxy.go` deliberately split 8734/8735 specifically so `serve` and `proxy` coexist ("while
  also test-serving a small local model," per its source comment); a second proxy on a third port
  is the same pattern one step further.
- **`lmcoder`'s container images are not extended.** No agent-spawning changes to
  `scripts/agent-canaries/*/Containerfile` or `multi/Containerfile` for now.
- **`harnez` owns a single, combined container** — one `Containerfile` at
  `harnez/scripts/agent-canary/Containerfile` (naming mirrors `lmcoder`'s `scripts/agent-canaries/`
  and this repo's existing `scripts/canary-usage.sh`) — not one file per agent CLI. Per-tool
  isolation comes from separate `RUN` instructions (one per CLI, same pattern `lmcoder`'s own
  `multi/Containerfile` already uses — pi even gets its own `RUN` there specifically for
  `--ignore-scripts`), which already gives independent build-cache invalidation per tool without
  needing separate files. `multi/Containerfile` proves five unrelated Node-based CLIs coexist fine
  in one `node:22-slim` image with no `ENTRYPOINT` (each invoked by name at `docker run` time) —
  same approach here. Credential mounting (Pi/OpenCode: none; Claude/AGY/Codex: real, see
  [[073-agent-canary-cloud-credentialed-claude-agy-codex]]) is a `docker run`-time concern, not a
  build-time image-per-tool concern.
  **Explicit course-correction**: an earlier draft of this ticket proposed one `Containerfile` per
  agent (`pi/Containerfile`, `opencode/Containerfile`, etc., mirroring `lmcoder`'s file layout
  1:1). That was reconsidered as unnecessary file proliferation — the same anti-pattern was called
  out as already present in `lmcoder` itself (6 Containerfiles where `multi/Containerfile` alone
  already proves one file suffices). Not filed as an `lmcoder` issue yet — that's the user's call
  to make, not assumed here.
- **Merging the two setups later, if it ever makes sense, is future cross-repo work** — done by an
  agent working in the `lmcoder` repo, not designed for preemptively here.
- **Cross-repo issue filing is normal in this workspace.** Friction or gaps found while using
  `lmcoder start`/`lmcoder proxy` for this new purpose (an external project standing up `lmcoder`
  as infrastructure, not `lmcoder` testing itself) get filed as `lmcoder` issues, not silently
  worked around in `harnez`. Hook logic and its tests stay `harnez`'s.

## 4. Split Into

- **[[072-agent-canary-local-llm-pi-opencode]]** — near-term, buildable now: Pi + OpenCode against
  the local `lmcoder`-hosted backend, no credentials needed.
- **[[073-agent-canary-cloud-credentialed-claude-agy-codex]]** — deferred: Claude Code, AGY, Codex
  against their real cloud plans with mounted host credentials, plus a net-new AGY Containerfile.
  Blocked on resolving the credential-mounting security posture before implementation starts.

## 5. Non-Goals (For Now)

- No full agentic coding-task testing in either child ticket. Liveness (pong) plus at most one
  distill-eligible tool call is the ceiling — this is about verifying hook mechanics, not agent
  capability.
- Not attempting `lmcoder`'s own local-model-serving/tuning concerns.

---

## Implementation Plan

### Assessment: architecture decided and delivered; nothing left to implement here

Verified 2026-09-04 — every §3 decision is already realized in-tree:

- `scripts/agent-canary/Containerfile` exists: **one** combined file (not one
  per agent), `node:22-slim`, no `ENTRYPOINT`, per-tool `RUN npm install -g`
  layers (`opencode-ai`, and `@earendil-works/pi-coding-agent` with
  `--ignore-scripts` exactly as §3 predicted), with `harnez` cross-built from a
  `golang:1.24-bookworm` stage. Naming matches `lmcoder`'s `scripts/agent-canaries/`.
- `scripts/agent-canary/run.sh` drives `build|static|pi|opencode|pi-hook|opencode-hook`
  and documents the `lmcoder start` / `lmcoder proxy` prerequisite on a
  dedicated port (`HARNEZ_AGENT_CANARY_PROXY_PORT=8736`,
  `HARNEZ_AGENT_CANARY_BACKEND_PORT=8737`), with an explicit "do not use the
  default 8735 proxy or any um760-backed proxy" warning — the §3 ownership
  split, implemented.
- `scripts/agent-canary/configs/` + `bin/` hold the runtime templates and
  launchers; `lmcoder`'s images were not extended.
- **[[072-agent-canary-local-llm-pi-opencode]] is Closed** (live local
  Pi/OpenCode liveness verified; hook-fire probe inconclusive, tracked by
  [[070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode]]).
- **[[073-agent-canary-cloud-credentialed-claude-agy-codex]] is still Open and
  deferred**, blocked on the credential-mounting security posture.

### Close-out steps (no code)

1. Flip this ticket's `Status` to
   `Closed — architecture delivered; remaining scope tracked by [[073]]`,
   since 071's own deliverable was the *decision* plus the split, both of
   which landed. A parent ticket should not stay open merely because one
   deferred child is open — 073 carries its own blocker.
2. Confirm `docs/other/Canary.md` mentions the agent-canary container as a
   worked example, or leave it alone if the doc is intentionally generic.
3. Do **not** open the cross-repo `lmcoder` Containerfile-proliferation issue
   §3 mentions — the ticket explicitly records that as the user's call, not an
   assumed action.

### Risks / open questions

- Only close after confirming with the user that a parent-architecture ticket
  closing ahead of its deferred child matches this tracker's convention; the
  alternative (keep 071 open as an umbrella until 073 resolves) is defensible
  and costs nothing.

### Scope

**None (close-out only)** — a status-header edit, no implementation.

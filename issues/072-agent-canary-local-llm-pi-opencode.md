# 072 — Agent Canary Container: Pi + OpenCode Against Local `lmcoder` Backend

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Agentic Ergonomics & UI Standards
**Related**: [[071-agent-canary-container-for-hook-testing]], [[070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode]], [[069-pretooluse-hook-optional-autopipe-bash-through-distill]]

---

## 1. Scope

Add **Pi** (`@earendil-works/pi-coding-agent`) and **OpenCode** (`opencode-ai`) to the single
shared `harnez/scripts/agent-canary/Containerfile` (see
[[071-agent-canary-container-for-hook-testing]] §3 — one combined file, not one per tool; `lmcoder`
hosts server+proxy only, this container is `harnez`'s own), talking to a local LLM backend hosted
by `lmcoder`. Purpose: verify each tool's hook/extension mechanism (from issue 070's research —
Pi's `tool_call` extension event, OpenCode's `tool.execute.before` plugin event) actually rewrites
a Bash command through `harnez distill` before it runs.

No credentials needed — both tools point at the local backend, not a real cloud plan.

## 2. Implementation & Verification Plan

1. Start the backend: `lmcoder start --model <small tier, TBD — check `lmcoder`'s `llama-canary/`
   for an existing small-model choice before picking cold>`, plus a second, dedicated
   `lmcoder proxy --listen-port <new, distinct from 8735>`.
2. `harnez/scripts/agent-canary/Containerfile`: one `node:22-slim`-based file (matching `lmcoder`'s
   `multi/Containerfile` shape), with one `RUN npm install -g opencode-ai` and one separate
   `RUN npm install -g --ignore-scripts @earendil-works/pi-coding-agent` (kept in its own `RUN`,
   same reason `lmcoder`'s does — see its comment), bakes in `harnez` only (not `rtk`, not the
   other agent CLIs yet), points at our own dedicated proxy port from step 1, no `ENTRYPOINT` (each
   CLI invoked by name at `docker run` time).
3. Implement the actual hook wiring these containers are meant to test — this ticket assumes
   issue 070's Pi/OpenCode adapters (`internal/distill.RewriteBashCommand` + a generated
   `.pi/extensions/rtk.ts`-equivalent / OpenCode plugin) already exist; if not yet done, do that
   first or as part of this ticket — don't build the test container before there's a hook to test.
4. Test protocol (per issue 071 §5 non-goals — keep this minimal): ping-pong liveness, then exactly
   one distill-eligible tool call (e.g. `git status` or `go test`) with
   `HARNEZ_DISTILL_AUTOPIPE=true` set, and assert the hook fired — exact assertion mechanism TBD
   (output-content diffing between raw and distilled forms is the simplest starting point).
5. File any `lmcoder start`/`lmcoder proxy` friction discovered along the way as `lmcoder` issues,
   not local workarounds (per issue 071 §3).
6. Verify with a live run against the real installed-in-container CLIs, plus `harnez status`.

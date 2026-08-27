# 072 — Agent Canary Container: Pi + OpenCode Against Local `lmcoder` Backend

**Status**: Closed — live local Pi/OpenCode liveness verified; hook-fire probe inconclusive, tracked by [[070-cross-agent-distill-autopipe-hook-agy-codex-pi-opencode]]
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

## 3. Implementation Notes

First buildable phase implemented:

- `scripts/agent-canary/Containerfile` is one shared `node:22-slim` image for Pi and OpenCode.
- The image installs `git`, `git-lfs`, `ca-certificates`, `opencode-ai`, and
  `@earendil-works/pi-coding-agent` in cache-friendly layers; Pi uses `--ignore-scripts`, matching
  `lmcoder` prior art.
- The current `harnez` binary is built in a Go builder stage and copied into the runtime image.
- Runtime launchers generate Pi/OpenCode configs from templates so the canary can point at a
  dedicated local `lmcoder proxy` port without rebuilding the image.
- Default live canary model is `qwen2.5-0.5b-instruct-q4`, selected from `../lmcoder/spec/models.yaml`
  as the tiny default smoke-test model suitable for the user's 8 GB VRAM constraint.
- Default local backend/proxy ports are `8737`/`8736`; do not use the default `8735` proxy or any
  `um760`-backed proxy for this ticket.

Deterministic verification implemented:

- `make agent-canary-static` builds the image and runs `harnez apply`, `harnez status`,
  `harnez distill hook`, Pi/OpenCode adapter file existence checks, and Pi/OpenCode CLI help checks
  fully inside the container.

Live verification still requires a local-only backend/proxy pair:

```bash
lmcoder start --model qwen2.5-0.5b-instruct-q4 --port 8737 --log-file /tmp/harnez-agent-canary-lmcoder.log
lmcoder proxy --insecure --backend-host 127.0.0.1 --backend-port 8737 --listen-port 8736
scripts/agent-canary/run.sh pi
scripts/agent-canary/run.sh opencode
```

Live local verification completed 2026-08-27:

- Initial port/process check found only the pre-existing `lmcoder proxy --insecure --listen-port
  8735` on PID 711859. Ports 8736 and 8737 were free and used for this canary.
- `make vendor-llama` was required in `../lmcoder` because the pinned `llama-server` binary was
  missing. This downloaded `llama-b10590-bin-ubuntu-vulkan-x64.tar.gz` and installed
  `../lmcoder/.vendor/llama.cpp/b10590/llama-server`.
- `qwen2.5-0.5b-instruct-q4` was not cached. `lmcoder start` downloaded only that configured tiny
  GGUF to `~/.cache/llama-canary/models/qwen2.5-0.5b-instruct-q4_k_m.gguf`.
- Backend start from the `harnez` working directory needed an explicit absolute `--server
  /home/uwe/projects/lmcoder/.vendor/llama.cpp/b10590/llama-server`; otherwise `lmcoder start`
  looked for `llama-server` relative to the current project.
- Dedicated local backend: `lmcoder serve` PID 1115792 and `llama-server` PID 1115799 on port 8737,
  using `--ctx-size 16384`.
- Dedicated local proxy: `lmcoder proxy` PID 1113790 on port 8736, forwarding to
  `127.0.0.1:8737`. The existing 8735 proxy was not touched.
- `curl http://127.0.0.1:8736/v1/models` returned the local
  `qwen2.5-0.5b-instruct-q4_k_m.gguf` model through the dedicated proxy.
- `scripts/agent-canary/run.sh pi` returned `PONG`.
- `scripts/agent-canary/run.sh opencode` initially looped on
  `request (91xx-92xx tokens) exceeds the available context size (8192 tokens)`. The canary
  container had to be stopped with `podman stop`, which escalated to SIGKILL after SIGTERM did not
  stop it within 10 seconds.
- Restarting the same tiny model with `--ctx-size 16384` fixed OpenCode liveness; `timeout 120s
  scripts/agent-canary/run.sh opencode` returned `PONG`.
- `scripts/agent-canary/run.sh static` passed after rebuilding the image. This proves `harnez
  apply` installs both adapters in the container and that `HARNEZ_DISTILL_AUTOPIPE=true harnez
  distill hook` rewrites a representative Bash command.
- Live hook firing was not conclusively observed. A bounded OpenCode probe with
  `HARNEZ_DISTILL_AUTOPIPE=true`, `--auto`, `--print-logs`, and a `git status` prompt loaded the
  canary config and reached the model, but the 0.5B model emitted a `task` JSON blob rather than
  executing a shell tool. A bounded Pi probe with only `bash` enabled returned `DONE` without a
  visible tool transcript. Treat liveness as verified here; keep issue 070 open for a stronger
  tool-call canary or a more capable local model when desired.

Additional local model verification completed 2026-08-27:

- Target model `qwen3-4b-instruct-2507-q4` was initially present in `../lmcoder/spec/models.yaml`
  but not cached.
- `lmcoder start --model qwen3-4b-instruct-2507-q4 --ctx-size 16384 --port 8737` downloaded
  `Qwen3-4B-Instruct-2507-Q4_K_M.gguf` to `~/.cache/llama-canary/models/` and loaded it
  successfully through the vendored `llama-server`.
- Dedicated local backend was `lmcoder serve` PID 1142586 with `llama-server` PID 1146133 on port
  8737. Dedicated local proxy was `lmcoder proxy` PID 1146416 on port 8736, forwarding to
  `127.0.0.1:8737`. The existing proxy on port 8735 was not touched.
- Loaded idle memory observed through `lmcoder status`: about 6.5GiB / 8.0GiB VRAM and 436-450MiB /
  11.6GiB GTT. During Pi/OpenCode generation the GPU reached 99% busy while VRAM stayed around
  6.5GiB and GTT stayed below 0.5GiB.
- `HARNEZ_AGENT_CANARY_MODEL=qwen3-4b-instruct-2507-q4 scripts/agent-canary/run.sh pi` returned
  `PONG`.
- `HARNEZ_AGENT_CANARY_MODEL=qwen3-4b-instruct-2507-q4 scripts/agent-canary/run.sh opencode`
  returned `PONG`.
- Live hook firing was still not observed by this pass because the canaries were liveness-only
  `PONG` prompts and did not force a shell tool call. Keep issue 070 open for a deterministic
  Pi/OpenCode tool-call fixture.

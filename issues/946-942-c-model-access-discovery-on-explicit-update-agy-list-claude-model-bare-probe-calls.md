# 946 — 942 C: model access discovery on explicit update (agy list, claude /model, bare probe calls)

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 942 (epic), 945, 941 (depends on both), canary `docs/studies/2026-10-25-canary-model-access-discovery.md`

---

/goal (slice C1, re-cut 2026-10-25) `harnez models probe <spec>` sends one bare probe to one model on its
harness CLI (claude, codex, agy) and prints `accessible`, `denied` or `unknown`, the error shape, and the input
tokens the call used; tests use recorded CLI output. It writes nothing. Listings and access.yaml are 954, batches
955. Or stop and report when a CLI cannot be probed bare. Circuit breaker: on a loop or no progress, clear the
goal and write a failure report here.

## Rules (epic decisions)

- Runs only on explicit update; asks before any probe call; stops on the first quota or auth error; budget from
  spec (945).
- **Bare probes (owner, 2026-10-25):** a probe asks the model with no harnez preface and as little system
  prompt and context as the CLI allows: no harnez hooks, rules, AGENTS.md, CLAUDE.md, skills or MCP servers,
  tools off, one-word prompt, smallest output limit. Find each CLI's minimal mode (e.g. claude bare/no-settings
  flags, codex/agy equivalents) and prove it with a canary: record the input tokens of one probe per CLI.
- Listing proves advertisement, a successful call proves access; ambiguous errors and timeouts stay unknown; an
  incomplete run never removes a model; unknown access does not disable defaults.
- Error shapes from the canary: claude `unrecognized_model`, codex HTTP 400 `model_not_found`, agy "not
  recognized as a known model or custom model".
- API listing (OpenAI, Google) only opt-in, keys never stored by harnez; Anthropic list API unverified.

# 412 — add support for harnez read -I docs vs native agent plain docs tests in canary-agenticloop-lite

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: issues/411 (token-cost baseline + doc-context trace), issues/362 (real-invocation harness), issues/366 (doc-compression canary loop), `scripts/canary-agenticloop-lite/main.go`, `docs/practices/ConciseMode.md` mentions `harnez read -I` PNG rendering elsewhere in this repo's Harnez Managed Conventions block

---

## 1. Problem & Motivation

`canary-agenticloop-lite` currently only exercises one context-delivery mode: a doc
file (`docs/AgenticLoop.md` or the lite variant) copied in verbatim as `AGENTS.md`,
read by the agent as plain text/markdown (via its own `Read` tool, confirmed in
issue 411's follow-up work — this costs an extra tool-call turn).

Separately, this repo's own Harnez Managed Conventions instruct agents to use
`harnez read -I <file>` — a "dense visual PNG context card" — instead of native
file-read tools for medium/large files, specifically to reduce token cost and
context fatigue. Issue 411's session also demonstrated `harnez read -I` on a
`claude -p "/context"` pipe, producing a rendered PNG plus a token breakdown
across text/ViT compression targets (Raw Text, Claude ViT, OpenAI ViT, Gemini
ViT).

There is currently no fixture/mode in this harness that measures whether an
agent instructed to read a doc via `harnez read -I` (visual PNG card) actually:
(a) follows that convention over its native plain-text read tool when both are
available, and (b) costs meaningfully more or less in real invoked tokens than
the plain-doc-as-AGENTS.md path this harness already measures.

## 2. Technical Specification / Findings

Scope for this ticket, informed by the existing harness structure in
`scripts/canary-agenticloop-lite/main.go`:

- Add a second "delivery mode" dimension alongside the existing doc `variant`
  (full/lite): e.g. `--delivery native` (current default: plain-text
  `AGENTS.md` copy-in) vs `--delivery harnez-read` (doc content pre-rendered
  via `harnez read -I` into a PNG, with `AGENTS.md` instructing the agent to
  view the rendered card at a given path instead of reading the raw doc).
- Reuse the existing `traceDocContext`/`measure-cost` machinery to report
  real per-mode token cost (`first-turn`, `total`, `turns`) side by side, the
  same way `full` vs `lite` variants are currently compared.
- Needs a canary-first probe (per this repo's own practice, `docs/Canary.md`)
  before committing to a specific fixture design: confirm `harnez read -I`'s
  actual CLI invocation/output contract (PNG path + token-breakdown text,
  as seen ad hoc in issue 411's session) is stable and scriptable
  non-interactively from a temp workspace, the same way the existing harness
  spawns isolated `os.MkdirTemp` workspaces per run.
- Open question needing the probe's answer, not assumed here: whether an
  agent invoked via `claude -p`/`agy -p` can view a PNG at all in this
  headless mode (image-reading tool availability in `-p` sessions), which
  would determine whether this is even a viable comparison or whether it's
  agent-support-gated and needs to be flagged as SKIP per agent like the
  existing `agy` cwd-isolation caveat in `results.md`.

## 3. Implementation & Verification Plan

1. Canary-first probe: from an isolated temp dir, run `harnez read -I` against
   a sample doc, then attempt `claude -p`/`agy -p` non-interactively with a
   prompt instructing the agent to view the resulting PNG; confirm whether
   either CLI's `-p` mode actually has image-viewing tool access. Record the
   result in `results.md` regardless of outcome (per this harness's existing
   honesty-about-limitations pattern from issue 411).
2. If viable: add the `--delivery` flag (or fixture-level field) to `run` and
   `measure-cost`, wire the harnez-read PNG rendering into the isolated
   workspace setup, and extend the results table with a delivery-mode column.
3. If not viable for one or both agents: document the limitation in
   `results.md` next to the existing `agy` cwd-isolation caveat, and close
   this ticket noting the finding rather than forcing an unsupported
   comparison.
4. Update `results.md`'s "Guide: running the suite/units yourself" section
   with the new flag/mode once implemented.

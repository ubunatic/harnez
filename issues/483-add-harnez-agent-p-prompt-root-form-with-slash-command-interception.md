# 483 — Add harnez agent -p/--prompt root form with slash-command interception

**Status**: Open

**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Agentic Ergonomics
**Related**: #479 (epic, design §2.1, §2.5), #480, #481, #482, #435

---

## 1. Problem & Motivation

Claude and agy users know `-p "prompt"` and `-c`. `harnez agent` should accept
the same shape without a verb, so one-liners and scripts stay short:

```
harnez agent -p "…"                       # new agent, default model
harnez agent --model=<m> -p "…"
harnez agent --name=<n> "…"               # upsert (start or resume)
harnez agent -c -p "…"                    # continue last attributable agent in -d, or start
harnez agent -p "/compact"                # intercepted agent command
```

## 2. Technical Specification

- The `agent` root command becomes runnable when `-p`/`--prompt`, `-c`, or
  `--name` plus positional prompt words are given; without them it prints help
  as today. Verbs keep working unchanged.
- Flags shared with the verbs: `--name`, `--model`, `-d`, `-f`, `--stream`,
  `--json`, `--plan`. Prompt assembly is #480, session choice is #482.
- Slash commands: `-p "/<command>"` is intercepted by harnez and never sent to
  the model. Initial set: `/compact`, `/stop`, `/status`. They act on the
  session chosen by `--name`, `-c` or attribution.
- An unknown `/x` is an error saying to send it literally with `-- /x`, so a
  typo is never forwarded to the model.
- Any `-p` text that does not start with `/` is a normal prompt.

## 3. Implementation & Verification Plan

- Root `RunE` that dispatches to the same start/resume/upsert code paths as the
  verbs; no second implementation of turn handling.
- Tests: each shown one-liner; `/compact` intercept without spawning a provider;
  unknown `/x` error; `-- /x` literal pass-through; help output unchanged when
  no prompt flags are given.
- Coordinate with #435 so interception of native subagent spawning and this
  slash-command handling do not overlap.

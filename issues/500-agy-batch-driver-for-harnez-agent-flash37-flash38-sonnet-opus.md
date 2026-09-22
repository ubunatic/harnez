# 500 — agy batch driver for harnez agent (flash37, flash38, sonnet, opus)

**Status**: Open
**Priority**: P1 (High)
**Severity**: Major
**Category**: Feature
**Related**: `internal/subagent/{driver,claude,codex,interactive}.go`, `cmd/harnez/agent.go`, `spec/agent.yaml`, 498 (Claude resume)

## /goal

`harnez agent start|resume --model agy:<m>` runs agy non-interactively and safely, pinned to the
session's working directory, for four models: `agy:flash37`, `agy:flash38`, `agy:sonnet`,
`agy:opus`.

## Problem

- `cmd/harnez/agent.go:20` `agentDriver` has no `agy` case → `UnsupportedDriver`.
- `spec/agent.yaml` has `agy:flash: gemini-3.7-flash`, which is fine for agy but the only alias.

## Canary findings (agy CLI, probed 2026-09-22 in an empty scratch dir)

- Print mode: `agy --model <m> --output-format json -p "<prompt>"`. **`-p` takes the next token as
  its value**, so `-p` must be the LAST flag, directly followed by the prompt.
- JSON result (single object on stdout):
  `{"conversation_id":"…","status":"SUCCESS","response":"PONG\n","duration_seconds":1.7,"num_turns":1,"usage":{"input_tokens":11810,"output_tokens":23,"thinking_tokens":21,"cache_read_tokens":0,"total_tokens":11833}}`
  Treat any `status` other than `SUCCESS` as an error (include the response text).
- Resume: `agy --conversation <id> --model <m> --output-format json -p "<prompt>"` — works, same id returned.
- `--model gemini-3.7-flash --effort low` works (base name + effort). `agy models` lists
  `gemini-3.7-flash-*`, `gemini-3.8-flash-*`, `claude-sonnet-4-6`, `claude-opus-4-6-thinking`.
- **Safety trap**: without `--add-dir`, agy writes files into `~/.gemini/antigravity-cli/scratch/`,
  NOT the cwd. With `cmd.Dir = <dir>` AND `--add-dir <dir>` the file landed in `<dir>`. Both are
  required. Tool calls were auto-approved in print mode without `--dangerously-skip-permissions`;
  do not add that flag.
- Tier mapping: `low`→`low`, `med`→`medium`, `high`→`high` via `--effort`.

## Pre-work (developer)

M1 (agy driver):
1. `spec/agent.yaml`: replace `agy:flash` with
   `agy:flash37 {name: gemini-3.7-flash}`, `agy:flash38 {name: gemini-3.8-flash}`,
   `agy:sonnet {name: claude-sonnet-4-6}`, `agy:opus {name: claude-opus-4-6-thinking}`, all
   `provider: agy, tier: low`. Check the schema (`spec/schemas/agent.schema.json`) and any test that
   pins `agy:flash`. Note bare `sonnet`/`opus` become ambiguous with `claude:*`
   (`resolveModelIn` handles that; fix tests that relied on bare names if any).
2. New `internal/subagent/agy.go`: `AgyDriver` modelled on `ClaudeDriver` (injectable `Command`),
   but it must run with `cmd.Dir = dir`. `RunOptions.Dir` exists; `Resume` has no dir parameter —
   check how codex/claude get the dir (session `WorkingDir`, process cwd) and choose the smallest
   clean way to pass it (e.g. a `Dir` field on the driver set in `agentDriver`, or cwd). Args:
   `--add-dir <dir> --model <name> --effort <effort> --output-format json -p <prompt>`; resume adds
   `--conversation <id>`. `Compact` → resume with `/compact`; `Stop`/`Delete` → nil.
3. `cmd/harnez/agent.go`: add `case "agy"`.
4. `internal/subagent/interactive.go` agy case: map `med`→`medium` for `--effort` too.
5. Tests (`agy_test.go`): arg order (`-p` last, prompt right after), `--add-dir` present with the
   dir, effort mapping, resume args, JSON parse incl. tokens (input/output/cached/thinking counted
   sensibly), non-SUCCESS status → error, malformed JSON → error.

## Acceptance

- [ ] `make test-q1` green; `make install` run.
- [ ] `harnez agent models` lists the four agy models.
- [ ] Live test (host, in an empty scratch dir without AGENTS.md): one tiny prompt per model via
      `harnez agent start`, plus one `resume`; a file write lands in the scratch dir.

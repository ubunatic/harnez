# Decide

`harnez decide` asks a decision model typed questions about a state and prints typed answers.
Code: `internal/decide`, `cmd/harnez/decide.go`. Backends: `spec/decide.yaml`. History: issue 633.

## Model

- A request is a `state` (text or JSON) plus named questions. Each question has a type:
  `noul` (probability that a yes/no statement is true), `choice` (one option from a map
  of option -> description) or `score` (a level from a list of level descriptions, lowest first).
- `Backend` is the only interface callers use: `Decide(ctx, *Request) (*Response, error)`.
- `spec/decide.yaml` names backends. Each entry gives a `protocol` (the wire format), a base URL,
  a path, and optionally a model and an API key variable. `protocols` in `spec.go` maps each
  protocol to its client; `TestProtocolsMatchSchema` keeps it equal to the schema's enum.

## Backends

| Backend | Protocol | Key | Notes |
|---|---|---|---|
| `jev` (default) | `systemone` | `$TYPESAFE_API_KEY` | `https://api.typesafe.ai/v1/systemone`; `$TYPESAFE_API_BASE` overrides the URL (proxy, LiteLLM) |

Adding a local model (Laya, lmcoder, ...):

- If it serves the System One wire format, add a spec entry with `protocol: systemone`, a local
  `base_url` and no `api_key_env`. No code change.
- If it has its own format, add a client implementing `Backend`, register it in `protocols` and
  add its name to the enum in `spec/schemas/decide.schema.json`.

## Wire Format (System One, verified live 2026-09-29)

`POST /v1/systemone`, `Authorization: Bearer <key>`, body `{model, state, questions}`. Answers:
`{"type":"noul","noul":0.93}`, `{"type":"choice","choice":"x","confidence":..,"probabilities":{..}}`,
`{"type":"score","score":2.0,"confidence":..,"legend":{"0":..},"probabilities":{"0":..}}`.
Errors: non-200 with `{"detail":{"error_type","message"}}`. A call takes about 0.6 s.

## CLI

- One quick question: `--noul`, `--choice` or `--score`, with `--option "name: desc"` or `--level`.
- Or a request file: `-f gate.yaml` (YAML or JSON; `-` reads stdin).
- State: `--state`, `--state-file`, the arguments, or stdin. Any of these replaces a state in
  the `-f` file; stdin is not read when the file has a state.
- `--pick <id>` prints only that answer (as JSON with `--format json`).
- Exit codes: 0 ok, 1 `--threshold t` not met (the answer's level, i.e. noul probability,
  the chosen option's probability or the score level, is below `t`), 2 any error.

## Pitfalls

- Gates must treat exit 2 (network, key, bad input) as "unknown", not as "no".
- With no state anywhere else, `decide` reads stdin; an open pipe that never closes blocks it.
- Every backend needs `timeout` in the spec; there is no default in code (docs/Spec.md).
- `Backend` has no `Close`. An in-process local model that holds resources should add an
  optional `io.Closer` check at the call site rather than widen the interface.

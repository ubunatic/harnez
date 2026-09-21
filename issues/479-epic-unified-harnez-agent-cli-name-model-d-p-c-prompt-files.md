# 479 — Epic: unified harnez agent CLI (--name, --model, -d, -p, -c, prompt files)

**Status**: Closed — children 480-484 closed; unified agent CLI, truthful resume state, quiet stream output and memorable names delivered; 476 (sync/async), 477 (hook completion) and 485 (usage-limit model autodetect) split out as follow-ups

**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: #480, #481, #482, #483, #484, #485, #476, #477, #478, #291, #342, #306, #144, `docs/HarnezAgentArchitecture.md`

---

## 1. Problem & Motivation

`harnez agent` grew verb by verb: `start <provider:model> <prompt>`,
`resume <session> <prompt>`, `-d` only on `start`/`chat`, no default model, no
multi-part prompts, no way to reach "the agent of this repo" without knowing its
name. Callers (host agents and humans) must remember argument order per verb,
and repo-default agents (for example one that files tickets in a given repo)
are awkward to address.

This epic tracks one coherent CLI so every verb takes the same handles
(`--name`, `--model`, `-d`, prompt input) and so a single line such as
`harnez agent -d <repo> -c -p "file a ticket for X"` works.

## 2. Refined Design

### 2.1 Handles, shared by every verb

| Flag | Meaning | Scope |
|------|---------|-------|
| `--name <name>` | Session name. Replaces the positional session. | all verbs |
| `--model <spec>` | Provider/model/tier. Optional (default in #484). Only meaningful when a session is created; on an existing session a conflicting value is an error, never a silent switch. | start, upsert |
| `-d, --dir <dir>` | Working directory of the agent (start) or attribution/filter scope (resume, list, status, stop, delete). Persistent flag on `agent`. | all verbs |
| `-p, --prompt <text>` | Prompt text, claude/agy style. | root form |
| `-c, --continue` | Continue the most recent attributable session in `-d`, or start one if none. | prompt verbs |
| `-f, --file <file>` | Prompt file, repeatable. `-f -` reads stdin. | prompt verbs |

Kept from work already delivered: `--stream full|stats`, `--json`. The delivered
`--plan-first` becomes `--plan=yes|no` (see #476); `inline` is dropped because a
blocked caller cannot intervene during an in-agent sleep, the turn boundary is
the review window.

### 2.2 Prompt assembly (#480)

Parts are joined by one blank line, in this order: all `-f` files (in flag
order), then the positional words (joined with spaces), then everything after
`--` verbatim. `--` is the escape hatch for prompts that start with `-` or `/`.
The harnez protocol preamble is added at send time and is never stored.
Stored `start_prompt` keeps the user text; files are recorded by path plus size,
not inlined.

### 2.3 Verb matrix

```
agent start  [--name N] [--model M] [-d D] [-f F]... [prompt...] [-- more...]
agent resume  --name N              [-d D] [-f F]... [prompt...] [-- more...]
agent resume [-d D]                        [prompt...]            # attribution, #482
agent -c     [-d D] -p "..."                                     # continue or start
agent        --name N "..."                                       # upsert
agent -p "..." | -p "/compact"                                    # new agent / command
agent <stop|delete|status|compact> --name N [-d D]
```

Corrections to the first draft:

- The last two lines of the draft's grouped list are identical; the second one
  is taken to be `resume`.
- `start --name X` when X exists is an error, `resume --name X` when X is
  missing is an error. Only the bare `agent --name X` form and `-c` upsert.
  Explicit verbs stay explicit.
- Names are unique per session store (not per directory); a collision is an
  error that names the existing session and its directory.

### 2.4 Attribution rules (#482)

A session is *attributable* to a call when it is resumable (see #478), not
quarantined (#306), in the same canonical `-d`, and manageable by the caller
(`HARNEZ_SESSION_ID` / `AGY_CONVERSATION_ID` lineage, as `CanManage` does today).

- bare `resume "prompt"`: exactly one attributable session, else an error that
  lists the candidates. Never guess when sending prompts.
- `-c`: the most recently active attributable session; none means start.
- destructive verbs (`stop`, `delete`) never accept `-c`; they need `--name` or
  a unique attribution and print what they resolved.
- Every resolution is echoed in `[session info: … resolved=name|dir|continue|new]`.
- Open question: Claude Code and Codex hosts may not export a session id
  comparable to `AGY_CONVERSATION_ID`; without one, attribution falls back to
  directory only, which is why bare `resume` demands uniqueness.

### 2.5 Slash-command interception (#483)

`-p "/<command>"` is handled by harnez for a fixed set (`/compact`, `/stop`,
`/status`, initially). An unknown `/x` is an error ("send it literally with
`-- /x`") so a typo is never forwarded to the model as prompt text.

### 2.6 No legacy forms

The old positional design is dropped, not deprecated (decision recorded in this
epic): there is no `start <provider:model> <prompt>`, no `<verb> <session>`
positional and no `chat <provider:model>`. Sessions are selected only by
`--name` (or by attribution, #482), models only by `--model`, and positional
words are always prompt text. Old invocations fail with a usage error that shows
the new form. Shell completion, `--help`, the man page and
`docs/HarnezAgentArchitecture.md` describe only the new forms.

## 3. Child Tickets and Order

| # | Ticket | Prio | State |
|---|--------|------|-------|
| 480 | Prompt input grammar (variadic, `--`, `-f`, stdin) | P2 | closed |
| 481 | Unified `--name`/`--model`/`-d`, old positional forms removed | P2 | closed |
| 484 | Default model from one spec value (phase 1) | P3 | closed |
| 478 | Listed sessions resumable or clearly terminal | P1 | closed |
| 482 | Attribution, bare `resume`, `-c` | P2 | closed |
| 483 | `agent -p` root form, upsert, slash commands | P2 | closed |

Follow-ups outside this epic (explicitly split out, not blocking closure):

| # | Ticket | Why it is not part of the epic |
|---|--------|-------------------------------|
| 485 | Autodetect the default model from usage limits | phase 2 of #484, needs a stable usage source |
| 476 | `--sync`/`--async` modes and first-use tip | async needs #477 first; `-d` and planning parts were delivered here |
| 477 | Hook-driven completion instead of polling | Codex hook investigation, independent research task |

Existing related tickets updated with a pointer to this epic: #476 (rescoped),
#291 (absorbed by #481/#484, closed), #342, #306, #144, #477, #478.

## 4. Delivered so far (before this epic)

Streaming Codex turns with labeled `[session info]`, `[message]`,
`[heartbeat]` (30s, 1m, 2m, 4m, 6m, 10m, then every 5m) and `[done]` blocks;
`--stream full|stats`; new-vs-cached token reporting; protocol preamble with
`CONFIRM:`/`PLAN:` labels, confirmation watchdog and violation notice;
`--plan-first` gate; compaction driven by uncached tokens with the ack split
from the reply. See `git log -- cmd/harnez/agent_stream.go`.

### 4.1 Delivered by the epic

`--name`/`--model`/`-d` on every verb; old positional forms removed with usage
hints; prompt files, `--` tail and stdin; default model from `spec/agent.yaml`;
bare `resume` and `-c` attribution with `resolved=` in the header; runnable
`agent` root form with `--name` upsert; `/compact`, `/stop`, `/status`;
`--plan yes|no`; truthful resume state (`RESUME` column, last error, Codex
terminal detection); memorable generated names; quiet stderr and no usage
dumps on errors; `harnez` session tips kept out of agent output; Codex resume
uses the same sandbox settings as start.

## 5. Implementation & Verification Plan

1. Land #480 and #481 first (pure CLI surface), including tests that the old
   positional forms are rejected with a usage error showing the new form.
2. Land #478, then #482 so attribution never selects a dead session.
3. Land #483 on top of the three.
4. Update `docs/HarnezAgentArchitecture.md`, `harnez agent --help`, shell
   completion and man page in the same sprint as the last child.
5. Close the epic when every child is closed or explicitly dropped; keep the
   table in section 3 current as tickets change state.

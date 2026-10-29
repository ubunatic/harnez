# 1000 — agy reviewer turns end with an empty reply when the model calls a tool

**Status**: Closed — Code landed as issue 995 before renumbering: 5a1589a7 (auto_approve_tools for advisor/reviewer, parseAgy empty-reply error); renumbered in merge 3731684a; merged via ubunatic/harnez PR #3. Live canary argon-991c: agy:argon:low advisor read the code and returned a full 991 refinement (8m08s).
**Priority**: P1 (High)
**Severity**: Major
**Category**: Bug
**Related**: 969, 992

---

## 1. Problem & Motivation

An `agy` turn whose model calls a tool ends after about 200 output tokens. harnez records it as
`completed`, prints an empty `[msg 1]` and exits 0, with no error or warning. Reviewers on agy
(the one-shot `agy:argon:low` review on the shared plan) silently return nothing and burn a scarce
turn. Owner policy: reviewers may use read tools and basic read commands (`git diff`, `git log`,
`cat`, `rg`).

Reported 2026-10-28 from the loom repo (branch `session-pty-keys-fixes`), sessions in
`~/.harnez/agents/`:

| Session | Model | Prompt | Output tokens | Reply length |
| --- | --- | --- | --- | --- |
| `pr-review-session-fixes` (f63e25b0), 2 turns | agy:argon:low | "run git diff main...branch" | 189, 254 | 0, 0 |
| `full-review-argon` (87de2536) | agy:argon:low | diff via `-f`, "you may read repository files" | 238 | 0 |
| `full-review-argon2` | agy:argon:low | diff via `-f`, "Do NOT call any tools" | 3494 | 11550 |
| `diff-review-flash38` | agy:flash38:med | diff via `-f`, "no tools are needed" | 1588 | 5733 |

Workaround in use: attach the diff with `-f` and forbid tool use in the prompt.

## 2. Technical Specification / Findings

Read from source at 0.2.6+4016; not verified with a live agy run, because the shared plan is
one-shot only.

1. **No permission flag reaches agy.** `internal/subagent/agy.go` `Run` builds
   `--add-dir <dir> [--model] [--effort] --output-format json -p <prompt>`; `Resume` is the same
   with `--conversation`. `--role` maps to nothing on the agy side. In headless `-p` nobody can
   approve the first tool call, so agy ends the turn.
2. **agy has no read-only allowlist.** `agy --help` offers only `--dangerously-skip-permissions`
   (approve every tool) and `--sandbox` (terminal restrictions).
   `~/.gemini/antigravity-cli/settings.json` has no permissions key (only `trustedWorkspaces`).
3. **The empty reply passes as success.** `parseAgy` accepts `status: SUCCESS` with an empty
   `response`.

## 3. Implementation & Verification Plan

1. `parseAgy`: `SUCCESS` with an empty `response` and `output_tokens > 0` is an error naming the
   likely cause ("agy turn ended without a reply; likely an unapproved tool call"). Table test.
   Consider the same check for the other providers' parsers.
2. Owner decision, tool access for agy read-only roles (advisor, reviewer):
   - a. `--dangerously-skip-permissions --sandbox` for those roles (writes inside the dir remain
     possible), or
   - b. no tools: harnez attaches the context and the role prompt says so; document it as policy.
3. Canary (one agy turn, owner's go): a reviewer prompt that needs `git diff` returns a non-empty
   reply, or a clear error per item 1.

## Decision (owner, 2026-10-28)

Option a, for the read-only roles: an agy session with role `advisor` or `reviewer` runs with
`--dangerously-skip-permissions --sandbox`, on `Run` and `Resume`. Writes inside `--add-dir` stay
possible; accepted. `developer` and `orchestrator` are unchanged (their tool access is a separate
decision).

## Implementation notes (host, 2026-10-28)

- The role does not reach the drivers today: `RunOptions` (`internal/subagent/driver.go:77`) has
  only `Prompt`, `Model`, `Dir`, and `Resume` takes `(id, prompt, model)`. Plumb the role (or a
  resolved "auto-approve tools" bool) from the session to the agy driver; the other drivers ignore
  it. A resumed session uses the role stored in its record.
- Which roles auto-approve is a spec value (`spec/agent.yaml` role entry, with its JSON schema),
  not a Go list (repo rule: configurable values live in `spec/`).
- Empty reply: `parseAgy` (`internal/subagent/agy.go`) returns an error when `status` is
  `SUCCESS`, `response` is empty and output tokens > 0, naming the likely cause. Table test.
- Tests: argv for advisor/reviewer (flags present, on Run and Resume) and developer (absent) with
  the existing fake command; never a real `agy`.
- Host canary after `make install`: the ticket 991 refinement as one `harnez agent --role advisor
  --model agy:argon:low` turn with read tools.

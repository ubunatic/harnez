# 1012 — Agent selftest hello and later steps omit --session so separate tool shells fail

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Bug
**Related**: 665, 666, 1009; [agent selftest](../cmd/harnez/agent_selftest.go), [session resolution](../internal/resolve/resolve.go), [Tools rule](../.harnez/rules/Tools.md)

---

## 1. Problem & Motivation

`harnez agent selftest` keeps its progress per session in `/tmp/harnez-selftest-<id>.json`. When no `--session` flag is given and no agent session environment variable is set, the id is derived from the parent process id (`ppid-<hash>-<unix time>`). Each separate tool shell can have a different parent, so the later steps look up a different state file and fail with `run 'harnez agent selftest --step hello' first`.

The hello output and the guidance that points to it do not carry the session id forward. A host following the printed steps verbatim gets a failing sequence. The same sequence passes when every step passes one shared `--session`.

## 2. Technical Specification / Findings

Observed by the host (reproduced):

- `harnez agent selftest --step hello` (no `--session`) printed a `ppid-…` session id. Its printed next steps were `HTO=0 harnez agent selftest --step background` and `harnez agent selftest --step confirm`, with no `--session`.
- The later `confirm` step, run from a separate tool shell, failed without a session flag.
- The whole sequence (`hello`, `background`, `confirm`, `confirm-running`, `verify`) passed when every step used the same explicit `--session <id>`.

Observed in this repository's `/tmp` state, read-only:

- `harnez-selftest-ppid-1e39b285d62a5f23-1793266004.json` holds only `hello`. A background step with no `--session` failed in the same session with `run 'harnez agent selftest --step hello' first` (log `/tmp/harnez-selftest-background.log`).
- `harnez-selftest-ppid-cfd1b400f80241c7-1793263947.json` holds `hello` and `background`, with no confirm steps. The confirm steps ran under a different id; the files do not show which process ran them.

Session precedence in `internal/resolve/resolve.go` (`Session`): an explicit value wins, then the first set variable of `SessionEnvVars` (`CLAUDE_CODE_SESSION_ID` is the confirmed one), then the parent-process lock fallback with a 30-minute window. `agent_selftest.go` applies `--session` first, then `resolve.Session`, then the literal `default`. The state file default directory is `os.TempDir()` (`--state-dir` overrides it).

Related but distinct: 665 and 666 are closed and cover the step sequence and HTO=0. Neither covers session identity.

## 3. Implementation & Verification Plan

**Goal:** a host can run the whole self-test sequence from separate tool shells by copying the session id that hello prints into every later step, and the printed instructions say so.

Desired output:

- `hello` prints the resolved session id and then the exact next commands with that id already filled in: `HTO=0 harnez agent selftest --step background --session <id>`, `harnez agent selftest --step confirm --session <id>`, then `confirm-running` and `verify` with the same flag. The id is quoted safely for a shell.
- The `background`, `confirm`, `confirm-running`, and `verify` error messages that say `run 'harnez agent selftest --step hello' first` name the `--session <id>` flag they need.
- `Tools.md` guidance (`config.yaml`) keeps saying to pass the same `--session` and matches the printed commands.

Tests (targeted):

- A hello run with no session environment variable prints `--session <id>` in each suggested step, and the id matches the state file.
- Separate invocations that each pass the same `--session` and `--state-dir` complete the sequence and `verify` passes. Use a temp state dir; no real `/tmp` state.
- Without `--session`, a later step in a different derived session still fails, and the message names the flag.
- The existing `cmd/harnez` self-test tests keep passing unchanged.

Goal check: the end-to-end host sequence in a non-leaf host session passes with the printed commands, with no hand-edited ids.

Circuit breaker: if making hello print the id changes session precedence (explicit, env, then parent), stop and ask before changing it. If two attempts at the printed-command fix still fail the cross-shell reproduction, stop and report the state files and process ids instead of adding more fallbacks. Do not change `resolve.Session` precedence or the lock window in this ticket.

Out of scope: the leaf-worker guard (`HARNEZ_AGENT_ROLE`), which stays as it is.

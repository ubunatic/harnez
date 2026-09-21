# 481 — Unify agent --name, --model and -d flags with legacy positional compatibility

**Status**: Open

**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: #479 (epic, design §2.1, §2.3, §2.6), #476, #291, #484, #480

---

## 1. Problem & Motivation

Today the session is positional for `resume`, `stop`, `delete`, `status` and
`compact`, but `--name` for `start` and `chat`; the model is a mandatory first
positional of `start`; `-d` exists only on `start` and `chat`. Hosts cannot
scope `list`/`status` to a repo and cannot use one flag set across verbs.

## 2. Technical Specification

- `--name <name>` on every verb that names a session. On `start` it names the
  new session (error if taken, message names the existing session and its
  directory); on `resume`, `stop`, `delete`, `status`, `compact` it selects the
  session (error if missing).
- `--model <spec>` on `start` (and the upsert form, #482): `codex:luna:low`,
  `codex:luna`, or a bare unambiguous alias such as `luna`. If omitted, use the
  default from #484. On an existing session, a `--model` that differs from the
  stored model is an error; it never switches the model silently.
- `-d, --dir` becomes a persistent flag of `agent`: agent working directory for
  `start`, attribution/filter scope for `resume`, `list`, `status`, `stop`,
  `delete` (folds in the `-d` item of #476).
- Legacy positionals keep working with a one-line stderr deprecation note:
  - `start <provider:model> <prompt…>` when the first argument is exactly a
    known model spec and at least one more argument follows;
  - `resume|stop|delete|status|compact <session> …` when the first argument
    names an existing session (name or ID).
  Ambiguity (an argument that is both a session name and prompt text) resolves
  to the legacy meaning only when `--name` is absent.
- Shell completion covers `--name` (session names) and `--model` (known specs).

## 3. Implementation & Verification Plan

- Shared `resolveSession(name, dir)` helper used by every verb, so #482 adds
  attribution in one place.
- Tests: each verb with `--name`; legacy forms with the deprecation note;
  `--model` conflict on an existing session; name collision message; `-d`
  scoping of `list` and `status`.
- Update `docs/HarnezAgentArchitecture.md`, `--help` text and the man page.
- Close #291 as absorbed once this and #484 land (model alias and flag
  normalization).

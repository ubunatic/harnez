# 481 — Unify agent --name, --model and -d flags and remove the old positional forms

**Status**: Closed — all flags unified, old positional forms removed with usage hints, completion added, docs and help text updated

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
- The old positional forms are removed (no deprecation period): no
  `start <provider:model> <prompt…>`, no `<verb> <session>`, no
  `chat <provider:model>`, no `attach <session>`. Positional words are always
  prompt text. Session selection is `--name` (or attribution, #482); model
  selection is `--model` (`chat --model`, `attach --name`).
- Old-style invocations fail with a usage error that shows the new form, for
  example `start: model is now --model <spec>; positional words are prompt text`.
  A mistyped `--model` value is rejected before any provider process starts.
- Shell completion covers `--name` (session names) and `--model` (known specs).

## 3. Implementation & Verification Plan

- Shared `resolveSession(name, dir)` helper used by every verb, so #482 adds
  attribution in one place.
- Tests: each verb with `--name`; old positional forms rejected with the usage
  hint; `--model` conflict on an existing session; name collision message; `-d`
  scoping of `list` and `status`.
- Update `docs/HarnezAgentArchitecture.md`, `--help` text and the man page.
- Close #291 as absorbed once this and #484 land (model alias and flag
  normalization).

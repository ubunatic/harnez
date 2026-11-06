# 1173 — Jules config loader reports a structure mismatch as not valid JSON

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 1071; study docs/studies/2026-11-06-jules-backlog-cleanup.md

---

## Problem

On 2026-11-06 `~/.jules/config.json` held `"github_projects": ["harnez", "cati", ...]` (strings).
`julesConfig` wants objects (`internal/subagent/jules.go:86`, `Projects []struct{ Repo string }`).
`loadJulesConfig` turns the unmarshal type error into `Jules config <path> is not valid JSON`
(`internal/subagent/jules.go:275-276`), although the file is valid JSON. The fix took a
manual structure dump.

## Want

- Tell a syntax error from a type mismatch; for the latter say
  `github_projects entries must be {"repo": "<name>"}`. Never include the key in the error.

## Done when

- A test with string entries checks the new message and that the key is absent from it.

## Plan accepted (dev-1173, claude:haiku:med, 2026-11-06)

Probed live with encoding/json: string entries, a plain string and `{"repo":5}` give
`*json.UnmarshalTypeError` (field `github_projects`/`github_projects.repo`), truncated JSON gives
`*json.SyntaxError`. Each mismatch gets a message that names the expected shape; the syntax case keeps
"not valid JSON" with the offset. Existing key-secrecy test stays unchanged. Extra per-field messages
accepted.

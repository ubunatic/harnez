# 1092 — Resume a Jules session started on another machine

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1088, 1090

---

/goal `harnez agent resume` can pick up a Jules session that another machine started, given only the Jules session ID that the ticket records in git, and the pending 1087 run (1090) is resumed this way. Circuit breaker: on a loop, a hard blocker or no progress, clear the goal and write a failure report here.

## 1. Problem & Motivation

The `/jules` skill records each run's Jules session ID in a `## Jules runs` table in git so that another machine can continue the run. But `harnez agent resume --name <name>` only finds sessions in the local store (`~/.harnez/agents`), so on any other machine it fails:

```
$ harnez agent status --name jules-implement-1087b
Error: session "jules-implement-1087b" not found
```

Seen on 2026-11-01 with run `5832791262383075829` (ticket 1090). Its state is `AWAITING_USER_FEEDBACK`, with no PR. The only way to read it from the second machine was raw curl against the Jules API.

## 2. Technical Specification / Findings

- `FileSessionStore.Find` (`internal/subagent/session.go:149`) resolves a registry ID or name and errors with `session %q not found`.
- `JulesDriver.Resume(ctx, id, prompt, model)` (`internal/subagent/jules.go:556`) takes the provider session ID and needs no local state beyond `~/.jules/config.json`, so the driver can already serve a remote run.
- `Session.ProviderSessionID` (`session.go:19`) holds the Jules ID for local runs.
- Jules API: `GET /v1alpha/sessions/<id>` returns `state`, `title` and `outputs[].pullRequest`. Activities are at `/v1alpha/sessions/<id>/activities`.

## 3. Implementation & Verification Plan

Proposed shape (open for the developer plan):

- **M1 (adopt):** `harnez agent resume --model jules --id <jules-session-id> [--name <name>] <wait|text>`. When no local session matches, create a local session record with `Provider: jules` and `ProviderSessionID: <id>`, then run the normal resume. Validate the ID against the API first (`GET sessions/<id>`), and refuse with the API's message if it fails. `status --id` works the same way, without a write.
- **M2 (docs):** update the `/jules` skill's "Run tracking" catch-up step to use the new form instead of the URL and `gh` fallback, and do the same in 1090's "Pick up" section.
- Tests use the existing fake Jules client from `internal/subagent/jules_test.go`, covering: an unknown ID (API 404 gives a clean error), an adopt that creates exactly one record, a second adopt that reuses it, `wait` sending no message, and the key never appearing in any error.
- Acceptance: on this machine, `harnez agent status --model jules --id 5832791262383075829` shows `AWAITING_USER_FEEDBACK`.

## Decision

- An explicit `--id` flag rather than a name search: the name lives only in the starting machine's store, while the ID is what git holds.
- Plan approved (dev-1092, 2026-11-01): with `--id` alone, the adopted record's name defaults to `jules-<last 6 chars of id>`. `--name` sets the name; it is refused when it conflicts with an existing record for that ID or with another session. The ID is validated with a GET before any write. `status --id` makes one GET and writes nothing.

## M1 Pre-Work

- Before adopting, confirm that `runResume` and `CheckResumable` (`agent_run.go:668`, `agent.go:659`) accept a Jules record with no local run history. If they reject it, fix that path inside M1.

# 1218 — agent status for a running Jules session warns: decode Jules API response: unexpected end of JSON input

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: [[1112-auto-mode-refuses-bash-calls-the-gear-shim-hook-rewrote-doubling-tool-calls]]

---

## 1. Problem & Motivation
Reported 2026-11-11 by the loom session (loom `docs/studies/2026-11-11-tuios-research-and-jules-sweep-story.md`,
sections 4 and 7). `harnez agent status --model jules --id 1348299604964268629`, on a running session in
state `IN_PROGRESS`, printed the state line and then:

```
Warning: could not read Jules activities: decode Jules API response: unexpected end of JSON input
```

The activity list (and so Jules's last message) was missing. Jules shuts down on 2026-12-31, so this matters
only for the runs still open until then; hence P3.

/goal `harnez agent status --model jules` on a running session shows its activities, or a warning that names
the real cause (empty body, truncated body, HTTP status), with a test; or stop and report when the cause
cannot be reproduced. On a loop or no progress, clear the goal and write a failure report here.

## 2. Technical Specification / Findings
- The warning is set in `internal/subagent/jules_adopt.go:69`.
- Responses are read with `io.ReadAll(io.LimitReader(resp.Body, julesMaxResponseBytes))`
  (`internal/subagent/jules.go:184`), limit 32 MiB (`jules.go:30`). Activities carry patches, so a body that
  hits the limit would be cut silently and fail to decode with exactly this error. Unverified: the response
  size of that session is unknown. An empty body (for example a 2xx with no content while the session runs)
  gives the same error; also unverified.
- Seen once; the session ID above can be re-read with `harnez agent status` while Jules still exists.

## 3. Implementation & Verification Plan
- Make the reader report a cut body (read limit + 1 and error when exceeded) and an empty body as distinct
  errors that name the URL path and HTTP status.
- Test with a fake server returning an empty body and an oversized body.
- If the cause is the size, page or stream the activity list instead of raising the limit.

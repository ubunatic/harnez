# 1181 — Isolate Harnez session state and constrain installation destinations

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Major
**Category**: Architecture

## Goal

/goal Enable least-privilege Harnez operation across concurrent agent sessions and narrowly scoped installation, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or no progress, record what was tried, why it failed, and the resume point.

## Problem and acceptance

Agents currently need access to shared `~/.harnez/sessions/` and `/run/user/$UID/harnez/procs/`, and `make install` writes shared executable and man-page directories. Broad write access risks tampering with other sessions and replacing unrelated binaries. Confirm the actual live layout, permission requirements and recent history before implementation.

- Design and verify session-scoped state, locks, logs and process records so one sandboxed session cannot alter another's; use a trusted broker for any necessary shared registry. Do not rely solely on same-user filesystem modes for isolation.
- Provide or document a constrained installation flow: build in a session workspace and install only explicitly approved Harnez binary and man-page destinations, without granting agents write access to all of `~/go/bin` or the shared man directory. Address symlink and path substitution attacks.
- Document exact read/write needs and coordination with taia's sandbox; test cross-session denial and allowed normal operations. Assess compatibility and migration rather than assuming a particular layout.
- Investigate separately the observed Bubblewrap `setting up uid map: Permission denied`; filesystem allowlists alone cannot fix namespace policy.

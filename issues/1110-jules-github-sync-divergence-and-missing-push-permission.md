# 1110 — Jules GitHub sync: divergence and missing push permission

**Status**: Closed — sync warning names GitHub-ahead vs diverged with the next step, and adds a credentials hint on a denied/403 push (syncGitHubBranch, internal/subagent/jules.go; TestJulesSyncNamesGitHubAhead, TestJulesSyncHintsAtCredentialsOnDeniedPush). harnez still pushes with the caller's own git credentials; a separate credential was not added.
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## Problem
Reported 2026-11-03 by the voxi and lmcoder sessions.

- voxi: `git push github main` was refused ("Permission ... denied to alfizock"), so GitHub main stayed behind and Jules audited old code.
- lmcoder: GitHub main had a commit Codeberg lacked (pushed to github only from another session); the sync cannot fast-forward.

Mitigated: the /jules skill says the push uses your own credentials and what to do when refused.

## Goal
/goal The start-time sync warning names the cause (permission denied vs diverged, with both SHAs) and the next step; stop and ask the owner before adding any new credential handling.

## Done when
- Warning text distinguishes refused-permission from divergence, tested with a fake remote.
- Owner decides whether harnez should push with a separate credential.

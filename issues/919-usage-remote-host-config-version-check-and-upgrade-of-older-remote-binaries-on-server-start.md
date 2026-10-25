# 919 — usage: remote host config, version check and upgrade of older remote binaries on server start

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: Epic 921, 912 (swap row needs the new remote binary), 879 (remote load)

---

/goal On start, the harnez server reads the same remote-host setting the usage UI uses, compares each
remote's harnez version with its own, and installs the local build on a remote that runs an older one; a
remote that runs a newer version is left alone. Verified with a test and a live run against x600; or stop and
report on an owner decision. Circuit breaker: on a loop or no progress, clear the goal and write a failure
report here.

## Notes (owner, 2026-10-25)

- Setup: this laptop (T14) manages x600. `harnez usage` connects to x600 at start, so the host is configured
  somewhere; find and document where (only the cache `~/.local/state/harnez/agents/usage/remote-load-*.json`
  was found so far). The server must read that same value, not a second copy.
- Never downgrade: older remote → upgrade, equal or newer → keep. Assumed safe; a developer on x600 pulls
  before working.
- Binary only, no repo sync. How to deliver it (scp of the local binary vs. remote `git pull && make install`)
  is a design point.
- Prerequisite: `harnez version` does not exist on either host; the comparison needs a comparable version
  (release version plus commit/time, or build ID).

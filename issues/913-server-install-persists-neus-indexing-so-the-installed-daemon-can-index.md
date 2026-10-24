# 913 — server install persists neus indexing so the installed daemon can index

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 897 (opt-in server neus indexing, flags only), 910 (epic)

---

/goal The installed `harnez server` (systemd unit from `harnez server install`) can run neus indexing: the
on/off choice and repo list live in a user-editable config (not the embedded spec), `server install` and the
running daemon read it, and a test plus a live `systemctl --user` check on this host show a scan running; or
stop and report on a design choice. Circuit breaker: on a loop or no progress, clear the goal and write a
failure report here.

## Finding (897 review, 2026-10-24)

- Indexing turns on only with `--neus-index --neus-repo PATH` (`cmd/harnez/server_cmd.go:86`); `server install`
  does not write them into the unit, so the background daemon never indexes.
- `spec/server.yaml:22,28` holds `enabled` and `repos`, but users cannot edit the embedded spec.
- Repo list: consider the repos harnez already knows (uman discovery or the finder's index list) instead of
  hand-typed paths.

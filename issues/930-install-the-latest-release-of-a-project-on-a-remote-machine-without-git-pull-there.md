# 930 — Install the latest release of a project on a remote machine without git pull there

**Status**: Closed — Filed in the wrong repo; moved to lmcoder 220
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 919 (server upgrades its own harnez binary on remotes), lmcoder v0.1.8 (Epic 216 vision service)

---

/goal One harnez command installs a project's latest release (or the local build) on a named remote
host, including its non-Go payload such as scripts and systemd user units, without a git checkout or
`git pull` on that host and without sudo. Verified by installing lmcoder v0.1.8 on x600 and starting
`lmcoder-vision.service` from the installed files. Stop and report when blocked on a user decision
(e.g. where installed files live) or denied permission; on a loop or no progress, clear the goal and
write a failure report.

## 1. Problem

To run the lmcoder vision service on x600 (2026-10-25) the host had to `ssh x600`, `git pull --ff-only`
a stale `~/projects/lmcoder` checkout (8809705, many releases behind), copy the unit file from the
checkout into `~/.config/systemd/user/` and enable it. The unit's `ExecStart` points into that checkout
(`%h/projects/lmcoder/scripts/vision-sidecar/server.py`). This makes a remote machine depend on a
working tree that can be dirty, stale or on another branch, and the steps are ad hoc each time.

919 solved this for harnez's own binary only (copy local build to remotes on server start).

## 2. Open questions

- Source: release assets from the forge (signed, `SHA256SUMS`) or the local build pushed over ssh, or both.
- Payload: Go binary only, or a release archive that also carries scripts/units (lmcoder's vision
  service is Python plus a venv; the venv itself stays per-host).
- Install location for non-binary files (e.g. `~/.local/share/<project>/<version>/`) and how units refer to it.
- Whether this is a generalisation of 919's mechanism or a separate `harnez` subcommand.

## 3. Verification

- lmcoder v0.1.8 installed on x600 by the command; `systemctl --user restart lmcoder-vision.service`
  serves `/openapi.json` on 8750 from the installed files, with `~/projects/lmcoder` absent or untouched.
- Re-running with the same version is a no-op; a newer version replaces it.

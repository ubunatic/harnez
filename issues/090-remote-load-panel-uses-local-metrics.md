# 090 — Remote Usage Load Panel Uses Local Metrics

**Status**: Closed — resolved in 6522a05
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: [[050-remote-host-flag-and-watch-hotkey]], [[088-load-panel-ram-vram-gtt-memory]], [[089-load-panel-combine-gpu-vram-gtt-row]], [docs/studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md](../docs/studies/2026-08-28-kernel-standard-metrics-sourcing-policy.md)

## Resolution

`UsageSummary` now carries an optional `Load *LoadSnapshot` field
(`internal/usage/types.go`), populated with `CollectLoadSnapshot()`
(`internal/usage/load.go`) whenever a local `harnez usage --json` process
runs — including the process CollectRemote invokes over SSH on the remote
host (`cmd/harnez/main.go`). `buildLoadBox` (`internal/usage/watch.go`) now
takes the active remote host and that snapshot: in local mode it still
calls `CurrentCPULoad()`/`CurrentGPUs()` live at the 1Hz redraw cadence
(unchanged); in remote mode it renders exclusively from the snapshot last
fetched at the existing remote collection interval (no SSH call in the
redraw path), and shows an explicit "remote load unavailable" placeholder
rather than silently substituting local telemetry when no snapshot has
arrived yet. The Load panel title also tags the active host
(`Load (@host)`) in remote mode. No vendor CLI/SDK telemetry source was
added on either side — same kernel-standard-only policy as local.

Verified with `go vet ./...`, `go test ./...` (all packages, including new
tests `TestParseRemoteJSON_WithLoad`,
`TestUsageSummary_LoadRoundTripsThroughJSON`,
`TestBuildLoadBox_RemoteUsesSnapshot`,
`TestBuildLoadBox_RemoteWithoutSnapshotDoesNotFallBackLocally`), and a real
build (`harnez usage --json` on the built binary now includes a `load`
object with `cpu`/`gpus` keys). No SSH loopback host was available in this
environment to exercise the live `--host` path end-to-end.

## Problem

`harnez usage --host <ssh-host>` fetches remote agent usage via SSH, but the `[L] Load` panel is
still built locally from the host process's own `/proc` and `/sys` files. After [[088]], that means
RAM, VRAM, and GTT rows shown during a remote usage session can describe the local machine while
the agent/quota panels describe the remote machine.

That mismatch is subtle and misleading. Remote mode should either show remote load telemetry or
make clear that local load telemetry is local.

## Cost Model

The expensive part is not collecting memory on the remote machine. `/proc/meminfo` and amdgpu sysfs
reads are cheap kernel-file reads there, just like they are locally.

The expensive part is transport and process setup:

- one SSH subprocess and round trip is orders of magnitude more expensive than several procfs/sysfs
  reads;
- polling SSH at the Load panel's 1 Hz local redraw cadence would add avoidable latency and system
  noise;
- the existing remote path already runs `harnez usage --json` over SSH, so extending that payload
  is likely cheaper and cleaner than opening a second SSH telemetry path.

## Desired Behavior

- Add remote load telemetry to the remote JSON payload, or otherwise carry a compact load snapshot
  alongside `UsageSummary`.
- In remote mode, render `[L] Load` from that remote snapshot instead of local `CurrentCPULoad()` /
  `CurrentGPUs()`.
- Preserve the kernel-standard-only telemetry policy on the remote host: no `rocm-smi`,
  `nvidia-smi`, vendor SDK, or vendor CLI fallback.
- Do not poll SSH at 1 Hz. Remote load can update at the existing remote collection interval unless
  a future design introduces a persistent multiplexed connection.

## Acceptance Criteria

- `harnez usage --host <host> --summary` does not silently show local RAM/VRAM/GTT as if it were
  remote.
- `harnez usage --watch --host <host>` does not open a new SSH subprocess every Load redraw.
- Tests cover parsing/rendering a remote payload that includes load telemetry.
- The UI labels or data flow make host ownership unambiguous.

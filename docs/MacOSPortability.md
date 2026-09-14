# macOS Portability & OS-Agnostic Architecture

Architecture decisions, platform boundaries, and implementation strategy for transitioning `harnez` from a Linux-first tool to an OS-agnostic system, prioritizing macOS (Darwin) as the primary non-Linux tier.

---

## 1. Design Principles & Strategy

1. **Zero Degraded Experience on Linux**: Existing Linux workflows, low CPU footprint, and telemetry fidelity must remain untouched.
2. **Standard Go Cross-Platform Libraries First**: Prefer standard, stdlib-adjacent libraries (e.g. `golang.org/x/term`, `golang.org/x/sys/unix`) over shelling out to CLI tools or bespoke unsafe ioctl code.
3. **Graceful Degradation for Platform-Specific Hardware Features**:
   - Hardware telemetry (CPU per-core meters, RAM breakdown, thermals) that relies on `/proc` or `/sys` can remain Linux-first initially; on macOS, unsupported graphs are gracefully hidden or simplified (e.g. in compact view).
   - Audio/mic activity monitoring is a nice-to-have feature that degrades gracefully to `MicUnavailable` when platform audio servers are absent, with native macOS CoreAudio probes added incrementally.
4. **Compile-Time Build Tag Isolation**: OS-specific implementations are partitioned cleanly using Go build tags (`_linux.go`, `_darwin.go`, `_fallback.go`) behind uniform internal interfaces.

---

## 2. Subsystem Architecture & Portability Plan

### 2.1 Terminal & Console I/O
- **Status**: Tracked in [issue #286](file:///home/uwe/projects/harnez/issues/286-promote-golang-org-x-term-for-terminal-operations-in-go-conventions.md) (Bucket: **Now**).
- **Strategy**: Replace all `stty -F /dev/tty size` and `stty cbreak -echo` subprocess calls in [`internal/usage/watch.go`](file:///home/uwe/projects/harnez/internal/usage/watch.go) with `golang.org/x/term` (`term.GetSize`, `term.MakeRaw`, `term.Restore`).
- **Benefit**: Completely cross-platform, zero subprocess overhead, eliminates GNU vs BSD `stty` flag incompatibilities.

### 2.2 Process Inspection & Agent Counting
- **Status**: Research tracked in [issue #334](file:///home/uwe/projects/harnez/issues/334-research-os-agnostic-process-inspection-across-macos-and-linux.md).
- **Strategy**: Replace `ps -eo comm=` in [`internal/usage/process.go`](file:///home/uwe/projects/harnez/internal/usage/process.go) with direct process table queries:
  - **Linux**: Direct `/proc/[pid]/comm` or `/proc/[pid]/stat` inspection.
  - **macOS (Darwin)**: `sysctl(KERN_PROC, KERN_PROC_ALL)` or `proc_listpids()` / `libproc` via `golang.org/x/sys/unix`.
- **Benefit**: Eliminates fork/exec overhead during rapid polling and avoids GNU vs BSD `ps` output quirks.

### 2.3 Audio Notifications & Lifecycle Hooks
- **Status**: Tracked in [issue #335](file:///home/uwe/projects/harnez/issues/335-support-afplay-audio-notifications-on-macos-in-default-hooks.md).
- **Strategy**: Support native macOS `afplay /System/Library/Sounds/Glass.aiff` for the `Stop` event hook alongside Linux `ffplay` / `paplay`.

### 2.4 Shell Interception Shim & Environment
- **Status**: Research tracked in [issue #336](file:///home/uwe/projects/harnez/issues/336-research-cross-platform-shell-shim-patterns-for-macos-and-linux.md).
- **Strategy**: Ensure `~/.harnez/shims/bash` and `~/.harnez/env.sh` handle macOS default `zsh`, Apple Silicon Homebrew paths (`/opt/homebrew/bin`), and POSIX `sh` fallback smoothly without breaking subshell environment propagation.

### 2.5 Security & Permission Whitelists
- **Status**: Research tracked in [issue #337](file:///home/uwe/projects/harnez/issues/337-research-macos-system-permissions-and-cli-whitelist-for-config-template.md).
- **Strategy**: Structure `permissions.allow` in `config.yaml` to include safe macOS CLI tools (`launchctl`, `diskutil`, `log show`, `brew`) and macOS system read paths (`/Library/**`, `/Applications/**`) alongside Linux equivalents.

### 2.6 System Hardware Telemetry & Audio Gating
- **Telemetry**:
  - `internal/usage/load.go` splits into `load_linux.go` (reading `/proc` and `/sys`) and `load_darwin.go` / `load_fallback.go`.
  - On macOS, `CPULoad` returns standard `getloadavg` load averages and basic memory metrics, leaving thermals/per-core graphs cleanly hidden if unavailable.
- **Audio Monitoring**:
  - `internal/usage/mic.go` detects backend availability; on macOS without PipeWire/Pulse/ALSA, returns `MicUnavailable` (displaying cleanly without breaking TUI layouts) until an optional CoreAudio probe is implemented.

### 2.7 CI Verification (`macos-hello`)
- **Status**: Implemented (2026-09-14), tracked in [issue #338](file:///home/uwe/projects/harnez/issues/338-macos-ci-verification-via-github-mirror-and-homebrew-release-packaging.md).
- **Homebrew packaging descoped**: macOS users install the same way as everyone
  else — `curl`-based installer or `go install` — not via a Homebrew tap. Do not
  reintroduce `brews:` GoReleaser config without a fresh user decision.
- **What exists**:
  - `.github/workflows/macos-hello.yaml` — `workflow_dispatch`-only (not on
    `push`/`PR`, to keep it opt-in and free of surprise cost), runs on `macos-14`,
    builds and runs `go test ./...` on real Darwin.
  - `make macos-ci` (`scripts/macos-ci.sh`) — dispatches the run and polls
    `gh run view --json status` quietly (no live-redrawn job tree from
    `gh run watch`, which floods agent context on every call). Prints one
    `PASS: macos-hello run <id>` line on success, or `FAIL: ... <url>` plus a
    ≤20-line grep of the failure log on failure.
  - `scripts/install-dev-deps.sh` — OS-aware installer (`brew`/`apt`) for tool
    dependencies `go build`/`go test` don't provide themselves (currently just
    `minisign`, needed by `internal/release` tests). Kept as a standalone script
    rather than inlined into the workflow YAML so the YAML stays minimal; a
    candidate for a future `harnez install --dev` subcommand.
- **First real finding**: the first real-Darwin run surfaced a genuine bug
  invisible on Linux CI — see [issue #341](file:///home/uwe/projects/harnez/issues/341-concurrent-sqlite-telemetry-writers-lose-rows-on-macos.md)
  (`internal/telemetry` concurrent SQLite writers race, confirmed flaky across
  repeat runs, not yet fixed). This is the concrete payoff of running on a real
  macOS runner instead of trusting `go build`/cross-compilation alone.
- **Not yet done**: `macos-hello` is `workflow_dispatch`-only — it does not run
  automatically on push/PR to the mirror. Wiring that in (and deciding on cost
  tradeoffs of running macOS CI on every push) is future work, not yet ticketed.

---

## 3. Implementation Roadmap

```mermaid
flowchart TD
    A["Phase 1: Core Terminal & Doc Foundations (#286)"] --> B["Phase 2: Research Studies (Process #334, Shims #336, Permissions #337)"]
    B --> C["Phase 3: Native macOS Hooks (#335) & CI Pipeline (#338, done)"]
    C --> D["Phase 4: Telemetry & Audio Build-Tag Decoupling"]
    D --> E["Phase 5: Fix macOS-only findings from CI (#341)"]
```

1. **Immediate (Now)**:
   - Land issue #286 (`golang.org/x/term` for terminal operations).
2. **Next**:
   - Complete research studies (Process inspection #334, Shell shims #336, Permissions #337).
   - Implement `afplay` macOS notification support (#335).
   - ~~Setup GitHub Actions macOS CI workflow (#338)~~ — done: `make macos-ci`.
3. **Later**:
   - Refactor `load.go` and `mic.go` into OS-gated modules (`_linux.go`, `_darwin.go`, `_fallback.go`).
   - Fix the concurrent SQLite telemetry writer race macOS CI surfaced (#341).
   - Homebrew packaging is explicitly out of scope — macOS installs via
     `curl`/`go install` like every other platform.

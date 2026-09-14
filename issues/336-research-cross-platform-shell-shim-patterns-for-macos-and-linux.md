# 336 — Research cross-platform shell shim patterns for macOS and Linux

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Architecture (Platform Portability)
**Category**: Architecture / Shims & Hooks
**Related**: [docs/MacOSPortability.md](../docs/MacOSPortability.md)

---

## 1. Problem & Motivation

`harnez` installs a transparent shell interception shim at `~/.harnez/shims/bash` and an environment script at `~/.harnez/env.sh`. The bash shim wraps tool execution to enable `harnez exec` hooks, distill rewrites, and command telemetry without requiring agent modification.

On Linux:
- `#!/bin/bash` is universal and consistently located at `/bin/bash` or `/usr/bin/bash`.
- Standard user shell environment loading follows Linux conventions (`~/.bashrc`, `~/.profile`).

On macOS:
- Default user login shell is `zsh` since macOS Catalina (10.15).
- macOS built-in `/bin/bash` is frozen at GNU Bash 3.2 (licensed under GPLv2), lacking Bash 4+ associative arrays and modern features.
- Users who install modern Bash via Homebrew have it located at `/opt/homebrew/bin/bash` (Apple Silicon) or `/usr/local/bin/bash` (Intel).
- PATH precedence and shell startup file evaluation (`~/.zshrc`, `~/.zshenv`, `~/.bash_profile`) differ fundamentally from Linux.

## 2. Research Scope & Prior Art

Research how other multi-platform developer tools, version managers, and agent wrappers implement cross-platform shims:
- **`asdf` / `mise` / `nvm` / `direnv`**: How they structure shims across bash, zsh, and POSIX `sh`.
- **PreToolUse / PTY interceptors**: How agent frameworks wrap CLI commands portably across macOS and Linux shells.
- Compatibility of POSIX `#!/bin/sh` vs `#!/usr/bin/env bash` vs native Go binary shims (`gear` binary symlink).

## 3. Key Questions to Answer

1. Should the shim use `#!/usr/bin/env bash` or POSIX `#!/bin/sh` to run reliably regardless of whether modern bash is in `/opt/homebrew/bin` or `/bin`?
2. How do subshell environments and PATH variables propagate when Claude Code or Antigravity executes shell commands on macOS?
3. What are the edge cases with macOS SIP (System Integrity Protection) when overriding or shimming standard PATH binaries?

## 4. Deliverables

- Research report in `docs/studies/` outlining findings and concrete recommendations.
- Follow-up development ticket(s) to implement macOS-compatible shell shims and test them against both Apple Silicon and Intel macOS environments.

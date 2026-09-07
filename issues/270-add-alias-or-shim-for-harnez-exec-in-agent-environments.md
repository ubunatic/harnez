# 270 — Add ⚙ alias or shim for harnez exec in agent environments

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: Issue 118; Issue 195; Issue 197; Issue 209; Issue 268; `cmd/harnez/exec.go`; `internal/claude/`

---

## 1. Problem & Motivation

User request: `add "⚙" alias/shim for "harnez exec" in agent envs where agent needs to call "harnez exec"`.

In agent environments where an agent executes commands or needs to explicitly invoke `harnez exec` (e.g. for tool attribution, execution metrics/telemetry recording, distillation via `--distill`, failure expectation via `--expect-failure`, or when running nested subshell commands directly), writing `harnez exec -- ...` has several friction points:

1. **Token footprint**: `harnez exec --tool <name> --` consumes unnecessary prompt/completion tokens on every explicit invocation in agent transcripts and context windows.
2. **Ergonomic brevity**: A single-glyph emoji/symbol alias like `⚙` (U+2699 GEAR) provides a compact, immediately recognizable indicator of an orchestrated/instrumented execution harness call.
3. **Agent environment integration**: In subagents, synthetic environments, or custom shell setups managed by `harnez apply` (or session hooks/profiles), having `⚙` readily available in `$PATH` or shell profile aliases allows agents to call `⚙ <cmd>` directly without boilerplate or path resolution friction.

## 2. Design Considerations & Seams

- **Binary / PATH Shim vs. Shell Alias**:
  - **PATH Shim**: A symlink or small executable wrapper named `⚙` placed in a managed PATH directory (such as `~/.claude/bin/⚙`, `~/go/bin/⚙`, or `~/.local/bin/⚙`) ensures availability regardless of subshell invocation method (`bash -c`, `sh -c`, `exec.Command`, or interactive subshell) without relying on interactive shell alias expansion (`shopt -s expand_aliases`).
  - **Shell Alias / Function**: Adding `alias ⚙='harnez exec'` or `⚙() { harnez exec "$@"; }` in generated agent environment setups (e.g. bashrc/profile fragments, AGY hooks, or agent prompt instructions).
  - **Binary Dispatch**: The `harnez` binary could detect if `os.Args[0]` (or `filepath.Base(os.Args[0])`) is `⚙`, automatically behaving as `harnez exec "$@"`.
- **UTF-8 and Shell Compatibility**:
  - `⚙` (`\xe2\x9a\x99`) is valid UTF-8 and fully supported as a filename, symlink, and executable command name in Linux/macOS filesystems and shells (`bash`, `zsh`, etc.).
  - Consider whether an ASCII alternative (e.g. `hx` or `hz-exec`) should coexist alongside `⚙` for environments with strict ASCII-only tooling constraints.
- **Lifecycle & Distribution**:
  - Installed or updated during `harnez apply` (or `make install`) alongside other agent tooling, hooks, and commands in `~/.claude` or system PATH.

## 3. Scope

**In scope:**
- Provide a `⚙` shim / executable symlink and/or shell helper that delegates directly to `harnez exec`.
- Support `harnez` binary auto-dispatching to `exec` subcommand when invoked as `⚙` (via symlink or multicall dispatch).
- Manage installation/provisioning of the `⚙` shim via `harnez apply` (e.g. under managed agent bin / PATH or hook environment).
- Document usage of `⚙` in agent docs / command references (e.g. `docs/practices/AgenticLoop.md` or `commands/`).
- Automated tests verifying execution of `⚙` with various arguments (`⚙ echo hello`, `⚙ --tool test -- ls`, etc.).

**Out of scope:**
- Modifying core execution semantics of `harnez exec` (timeouts are tracked in Issue 268; distillation in Issue 178).
- Forcing non-agent human user shell profiles to bind `⚙` if outside managed agent environments.

## 4. Acceptance Criteria

- [ ] Invoking `⚙ <cmd>` or `⚙ [flags] -- <cmd>` behaves identically to `harnez exec [flags] -- <cmd>`.
- [ ] `harnez apply` provisions the `⚙` shim/symlink in managed agent environments / PATH so it is immediately usable by agents.
- [ ] Multicall/symlink dispatch works when `harnez` binary is symlinked to `⚙`.
- [ ] Flags such as `--tool`, `--distill`, `--expect-failure`, and `--timeout` are correctly forwarded through `⚙`.
- [ ] Unit tests cover `⚙` symlink/dispatch invocation and argument handling.
- [ ] Documentation reflects `⚙` as a supported compact alias for `harnez exec` in agent workflows.

## 5. Verification Guidance

- **Unit test**: Test `os.Args[0]` multicall dispatch and symlink execution in Go tests.
- **Live verification**:
  1. Run `make install` and `harnez apply`.
  2. Run `⚙ echo "hello from gear"` in bash and verify exit code and stdout.
  3. Run `⚙ --tool test -- true` and verify telemetry row recording in `harnez usage`.


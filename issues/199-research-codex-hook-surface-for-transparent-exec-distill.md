# 199 — Research: does Codex CLI have a hook surface like agy's hooks.json?

**Status**: Draft
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Research
**Related**: [[193-research-agy-hook-surface-for-transparent-exec-distill]]
(prior research of the same shape for agy — reuse its methodology and
question structure rather than reinventing it), [[196-agy-native-hooks-plan-alongside-claude-hooks]]
(what got built once 193 found agy's `hooks.json`), `internal/claude/apply.go`
(existing Claude Code `PreToolUse` hook wiring this would extend to a third
agent if a Codex equivalent exists), `docs/CLIDesign.md` (apply/init split
to mirror if any config gets written)

## Problem

harnez currently wires `PreToolUse`-style interception into two agents:
Claude Code (`internal/claude/apply.go`, native since the start) and agy
(`internal/agy/hooks.go`, added in ticket 196 after 193's research found
agy's real `hooks.json` lifecycle-hook system). `harnez apply` also already
writes Codex *skills* (`~/.codex/skills/<name>/SKILL.md`, per
`docs/CLIDesign.md`), so Codex is a known, already-integrated target for
harnez — but nobody has checked whether Codex CLI has any equivalent
hook/extension surface for rewriting or intercepting shelled-out tool
calls the way agy's `hooks.json` or Claude's `settings.json` hooks do.

This ticket exists because 193's own investigation is now stale in one
specific way: the user hasn't used Codex in a while, so any settings/config
surface may have changed since it was last looked at (if it ever was —
this repo's own history shows Codex-specific tickets for status bar/usage
tracking, e.g. issues 139/141/144/149/156, but none investigating a
hook/PreToolUse-equivalent mechanism specifically).

## Task — research only, no implementation yet

Mirror 193's research shape and question structure:

1. **Is there a documented or discoverable hook/extension mechanism?**
   Check `codex --help` and subcommand `--help` output, any embedded
   help/doc strings in the Codex binary (193 found agy's `hooks.json` doc
   via `strings`-grepping the binary itself — try the same technique),
   and Codex's own config file(s) (locate them first — check the obvious
   candidates like `~/.codex/config.*` before assuming a layout).
2. **If a mechanism exists, what's its contract?** Config file location(s)
   (global vs. workspace-local), supported event names (PreToolUse-
   equivalent, PostToolUse-equivalent, etc.), matcher/filter syntax,
   handler types (command/HTTP/other), and — critically — the stdin/stdout
   JSON schema for a command-type handler (agy's turned out to support a
   `overwrite.CommandLine` rewrite field; check whether Codex's does or
   doesn't have an equivalent capability).
3. **Does Codex resolve shelled tool calls via `$PATH`?** Re-run 193's Q2/Q3
   live PATH-shim experiment (shim script logging + `exec`-ing the real
   binary, prepended to `$PATH`, run against a real non-`--help` Codex
   invocation) to check whether a PATH-shim approach (195's mechanism) would
   also work for Codex, independent of whether a native hook exists.
4. **Any MCP-equivalent extension point?** Check whether Codex has an
   MCP-server registration mechanism like `agy mcp add` (193's Q4) as a
   fallback "tell Codex" integration path if no hooks-style mechanism turns up.
5. **Blast-radius/safety notes**: same considerations 193 raised for
   agy — cwd behavior for shelled commands, `$PATH` precedence, timeout
   behavior for hook handlers if any exist, disable path if a config gets
   written.

## Non-goals

- No implementation of any Codex hook wiring, PATH-shim, or MCP
  registration in this ticket — purely investigative, matching 193's own
  scope discipline (it stayed research-only and spawned 195/196 as
  follow-ups).
- Not re-litigating the existing Codex *skills* integration
  (`~/.codex/skills/`) — that's already built and out of scope here.

## Acceptance Criteria

1. Findings section written into this ticket answering the five questions
   above (or explicitly stating "no such mechanism found" with the
   evidence for that conclusion, which is itself a valid research outcome).
2. If a real hook mechanism is found, a Recommendation section states
   whether a follow-up ticket (mirroring 195/196's split) should be filed,
   and sketches which of the two shapes (quiet PATH-shim vs. native/
   documented config) fits best — without committing to full scope itself.
3. If no mechanism is found, the ticket documents that clearly enough that
   a future session doesn't waste time re-deriving the same negative
   result.
4. No code changes in this ticket.

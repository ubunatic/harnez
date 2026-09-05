# 152 — Move `agent-collector` under `usage` instead of top-level

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Cosmetic
**Category**: CLI Design
**Related**: [[153-command-tree-placement-spec-assessment]] (broader question this is the first concrete
instance of), `cmd/harnez/main.go`

## Problem

`cmd/harnez/main.go` registers 17 top-level commands on `root`
(`apply, diff, scanDocs, clean, status, usageCmd, loadStreamCmd, initCmd, assessCmd, collectorCmd,
distill, mode, release, statusline, rate, exec, stats, index`). `agent-collector` (`collectorCmd`,
main.go:316) is one of them, but its concern — collecting agent usage/quota data — is a `usage`
concept: `usageCmd` already exists as a parent with a `history` subcommand tree
(`timeline`/`fetch`/`record`/`stats`, main.go:309-310). `agent-collector` sitting at the top level
instead of under `usage` is exactly the kind of ungrouped command the top-level namespace is
accumulating too many of.

## Scope

- Move `agent-collector` to be a subcommand of `usage` (e.g. `harnez usage agent-collector`, or a
  shorter subcommand name if one reads better under the `usage` parent — decide during
  implementation).
- Update `root.AddCommand(...)` to drop `collectorCmd` from the top-level list and add it via
  `usageCmd.AddCommand(...)` instead.
- Update any docs, help text, or scripts referencing `harnez agent-collector` at the old path.
- Check `docs/README.md`/`docs/other/Spec.md` or other docs for literal `agent-collector` usage
  examples that need the new path.

## Out of Scope

- The broader command-tree/spec question — see [[153]]. This ticket is scoped to one concrete
  move, independent of whether 153 proceeds.

---

## Implementation Plan

### Research notes (2026-09-04)

- `collectorCmd` is defined at `cmd/harnez/main.go:389-421` and registered in the single
  `root.AddCommand(...)` call at `main.go:602` (which now carries ~24 commands, not 17 — the
  problem this ticket describes has grown since filing).
- `usageCmd` is the parent; `historyCmd` (`main.go:~296`) is its existing subcommand precedent.
- **The blocking constraint is the systemd unit.** `systemd/harnez-agent-collector.service`
  hardcodes `ExecStart={{HARNEZ_BIN}} agent-collector`, and `internal/claude/apply.go:336-360`
  installs/refreshes that unit from the embedded template. A bare rename breaks every *already
  enabled* unit on disk until the user reruns `harnez apply` — the unit keeps its old ExecStart
  and the service starts failing on the next boot. `internal/claude/systemd_test.go:48` asserts
  the literal `exe + " agent-collector"` string.
- Documentation references are thin: `main.go:391` Short text, `main.go:444` (`--systemd` flag
  help), `internal/usage/*.go` doc comments (~8 occurrences), `apply.go:813` (the
  "enable with: systemctl --user enable --now ..." hint), and one historical mention in
  `docs/studies/2026-08-28-usage-collector-daemon-architecture.md`.

### Steps

1. **Keep the top-level path working as a hidden alias.** Before moving anything, make the move
   non-breaking: register `collectorCmd` under `usageCmd`, and additionally register a
   `Hidden: true`, `Deprecated: "use \`harnez usage collector\`"` shim at root that delegates to
   the same `RunE`. Cobra's `Deprecated` prints a one-line notice to stderr and still runs the
   command — exactly the behaviour a systemd unit needs while it is still pointing at the old path.
2. **Pick the subcommand name.** Recommend `harnez usage collector` (not
   `usage agent-collector`) — under a `usage` parent, "agent-" is redundant. Set
   `Use: "collector"` and add `Aliases: []string{"agent-collector"}` so `harnez usage
   agent-collector` also resolves.
3. **`cmd/harnez/main.go`**: drop `collectorCmd` from the `root.AddCommand(...)` list at line 602,
   add `usageCmd.AddCommand(collectorCmd)` next to the existing `usageCmd.AddCommand(historyCmd)`,
   and add the hidden root shim from step 1. Flags (`--interval`, `--once`, `--offline`) move with
   the command unchanged.
4. **`systemd/harnez-agent-collector.service`**: change `ExecStart` to
   `{{HARNEZ_BIN}} usage collector`. Leave the unit *filename* alone — renaming it would orphan
   already-enabled units under the old name, which is a strictly worse failure than a stale
   ExecStart.
5. **`internal/claude/systemd_test.go:48`**: update the asserted substring to
   `" usage collector"`.
6. **`internal/claude/apply.go`**: update the comments at 336/338/355/554 only if they name the
   command path; update the user-facing hint at 813 if it prints the command.
7. **Doc-comment sweep**: update `internal/usage/{collector,statecache,usage,types,watch}.go` and
   `internal/usage/*_test.go` occurrences of `harnez agent-collector` → `harnez usage collector`.
   Leave `docs/studies/2026-08-28-*.md` alone — a study is a dated historical record and should
   not be retroactively edited.
8. **`docs/CLIDesign.md`**: the command-responsibilities table (lines 11-27) lists `usage` and
   `usage history` but not the collector at all — add a `usage collector` row while touching it.
9. **Verify**: `go test ./...`, then `harnez usage collector --once --offline` and
   `harnez agent-collector --once --offline` (deprecated path) both write snapshots;
   `scripts/smoke-test.sh` to confirm the regenerated unit file applies cleanly.

### Design decisions / tradeoffs

- **Alias + deprecation rather than a clean break.** This is a P3 cosmetic change; it must not
  cost anyone a broken daemon. The shim is ~6 lines and can be deleted in a later release.
- **Rename the subcommand to `collector`, keep the unit filename.** The user-facing path gets the
  cleaner name; the systemd identity (which users have typed into `systemctl --user enable`) stays
  stable.
- Deliberately does not touch the broader top-level-namespace question — that is [[153]].

### Risks / open questions

- Users with an enabled unit installed before this change keep running the deprecated path until
  their next `harnez apply`. Acceptable given the shim, but worth a line in the release notes.
- If [[153]] later decides on a different grouping convention, this move may need revisiting;
  low cost either way.
- Open question for the user: is `usage collector` preferred over keeping the full
  `usage agent-collector`? The plan assumes yes, with the alias covering the other.

### Scope estimate

**Small** — roughly one file of real change (`main.go`), one template, one test assertion, plus a
mechanical doc-comment sweep.

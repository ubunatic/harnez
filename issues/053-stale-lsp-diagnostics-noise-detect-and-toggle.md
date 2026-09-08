# 053 — Background LSP diagnostics post stale/wrong findings; harness should detect and offer to disable per-agent

**Status**: Open
**Category**: Agentic Ergonomics / Harness Behavior
**Related**: [docs/practices/AgenticLoop.md](../docs/practices/AgenticLoop.md); observed in and drawn from sibling project `weg`'s session retrospective `docs/feedback/2026-08-23-hardening-automation-and-a-production-incident.md` (not linkable across repos — see that file directly in the `weg` checkout)

---

## 1. Problem & Motivation

Observed repeatedly in a `weg` session (2026-08-23, Go codebase): after
`Edit` tool calls, Claude Code's own background Go-language-server
integration automatically posted `<system-reminder>` diagnostic blocks
into the conversation — without being invoked — reporting compiler errors
that did not reflect real `go build`/`go vet` state. Two recurring
patterns:

- **Cross-file rename lag**: renaming an exported symbol in file A (e.g.
  `occCmd` → `OCCCmd`), then immediately editing file B to call the new
  name, produced an `undefined: nextcloud.OCCCmd` diagnostic on file B —
  the LSP server hadn't reindexed file A's rename yet.
- **Stale-reference lag**: diagnostics referencing code that had been
  fixed several edits/turns earlier (a field type change from `int` to
  `*int`, resolved turns before) kept reappearing on later, unrelated
  edits to the same file.

Every instance was independently caught by re-running the real toolchain
(`go build ./... && go vet ./... && gofmt -l .`) before trusting or acting
on the diagnostic — so no incorrect action resulted this session. But it
happened often enough (at least 5 separate times in one session) that it's
a real tax on trust and turn count: each occurrence requires an explicit
"let me verify with the real toolchain" detour, and a less careful agent
session (or one under different verification discipline) could plausibly
act on a false positive — reverting correct code, or reporting a build as
broken when it isn't.

## 2. What's being requested (three related asks, priority not yet decided)

1. **Investigate whether this specific harness's background LSP
   integration can simply be disabled** for sessions/projects where its
   diagnostics are proving net-negative (noisy relative to their hit
   rate) — at minimum document how, if a toggle already exists.
2. **The harness should detect whether a Core Language Server (or
   language-specific LSP) is enabled or disabled** for a given agent/
   session, surface that state somewhere inspectable (status output,
   session metadata), rather than it being an invisible background
   behavior the agent only discovers by observing false positives.
3. **Decide what the harness does with that state** — options, not yet
   narrowed to one:
   - Emit a one-time warning/notice when LSP diagnostics disagree with a
     subsequently-run real toolchain check (a concrete, checkable signal
     of staleness) — this is naturally where an agent already re-verifies
     after being surprised, so the harness could observe the same
     disagreement itself.
   - Let a project or session opt out of background LSP diagnostics
     entirely, defaulting to whatever's least surprising.
   - Do nothing structurally, just document the "always verify with the
     real toolchain, never trust the diagnostic block alone" discipline
     more prominently (e.g. in `docs/practices/`), on the theory that the
     existing discipline already caught every instance and the tool has
     enough true-positive value to keep as-is.

This ticket intentionally does not pick one of these — that's a design
decision for whoever picks this up, informed by how often the false-
positive pattern recurs across other sibling projects, not just `weg`.

## 3. Notes

- This is a harness-level behavior (Claude Code's own background tooling),
  not something `weg` or any other managed project can fix from its own
  side — hence filing here rather than in the sibling project.
- The failure mode is specifically about *staleness* (diagnostics lagging
  the actual current file state after a burst of edits), not about the
  LSP being wrong in principle — worth distinguishing from a general
  "LSP integration is unreliable" claim, which isn't what was observed.

---

## Implementation Plan

### Framing: pick one of §2's three options first

This ticket deliberately left the design open. The plan below **picks option 3
first, option 2 second, option 1 as prerequisite research, and drops the
disagreement-detector (first bullet of §2.3)** — with the reasoning stated so a
future agent can overrule it rather than re-derive it.

The disagreement-detector is rejected as the opening move: harnez sees tool
calls via its telemetry hooks, but the LSP `<system-reminder>` blocks are
injected into the *conversation* by the harness, not routed through any hook
harnez installs — so harnez has no observation point for "the diagnostic said
X" to compare against "`go build` then said Y". Building that would require
transcript scraping, which this repo's Troubleshooting norms explicitly warn
against.

### Steps

**Step 1 — Research the toggle (blocking, cheap, do this before anything else).**
Determine whether Claude Code exposes a setting to disable background language-
server diagnostics, and at which scope (global `~/.claude/settings.json`,
project `.claude/settings.json`, or env var). Check `claude config list` / the
settings schema / release notes; do **not** guess a key name. Two outcomes:
- *A toggle exists* → continue to steps 2-4.
- *No toggle exists* → skip steps 2-3, do step 4 only, and record the negative
  finding in this ticket so it isn't re-researched.

**Step 2 — Make the setting managed (only if step 1 found a real key).**
`internal/claude/config.go` already carries per-feature scalar fields
(`StatusLine bool`, `Model`, `Effort`); add the LSP toggle the same way
(`yaml:"..."`, one field, no new subsystem), and write it in
`internal/claude/apply.go` alongside the other `settings.json` keys. Add the
key to `config.yaml`. Follow the CLI-scope rule in `docs/CLIDesign.md`: this is
a global `~/.claude` concern, so it belongs to `apply`, **not** `init`.

**Step 3 — Surface the state (the ticket's actual ask #2).**
`internal/claude/status.go` already has `hasSettingsKey(path, key)` and an
`Applied:` checklist built from `[]entry{label, check}` (`status.go:15,63`).
Add one entry reporting the LSP-diagnostics key's presence/value so the state
is inspectable via `harnez status` instead of being an invisible background
behavior. Test it in `internal/claude/integration_test.go` the same way other
applied-key checks are tested (present / absent / drifted).

**Step 4 — Document the discipline (do this regardless of steps 1-3).**
Add to `docs/practices/AgenticLoop.md` section 4 anti-patterns, next to the
existing verification bullets:
> ❌ **Trusting a Background Diagnostic Over the Real Toolchain**: acting on an
> auto-posted LSP/`<system-reminder>` diagnostic block (reverting code,
> reporting a broken build) without re-running the project's real gate
> (`go build ./... && go vet ./... && gofmt -l .`, `make check`). Language
> servers lag a burst of edits — a cross-file rename or a just-fixed type
> error reliably produces phantom errors for several turns. The diagnostic is a
> hint to verify, never a result.

Then resync (`harnez apply`, downstream `harnez init --docs AgenticLoop`).

**Step 5 — Verify.** Steps 2-3 touch Go, so `make install` and one live check:
run `harnez apply` against a scratch target, then `harnez status`, and confirm
the new line reflects the actual `settings.json` content — per AgenticLoop
Phase 3's "Live/Real-Environment Verification for hooks & env-resolution
features", passing `go test ./...` alone is not sufficient evidence here.

### Design decisions / tradeoffs

- **Report, don't decide.** harnez surfaces the setting and lets the user pin
  it in `config.yaml`; it does not default to disabling LSP diagnostics. The
  ticket itself notes the failure mode is *staleness*, not the LSP being wrong
  in principle — the true-positive value is real.
- **Doc bullet is unconditional.** It is the only part of this that works even
  if no toggle exists, and it is what actually caught every instance in the
  `weg` session.
- **No transcript scraping**, per the reasoning above and this repo's
  Troubleshooting & Log Exploration norms.

### Risks / open questions

- Step 1 may find no toggle, reducing this ticket to a one-bullet doc change —
  that is an acceptable outcome, not a failure.
- The setting key may be undocumented/unstable across Claude Code releases; if
  so, prefer reporting it read-only in `harnez status` (step 3) and skip step 2
  rather than writing an unstable key on every `apply`.
- Evidence base is one project (`weg`). Before investing in steps 2-3, a cheap
  sanity check is whether the pattern recurs in other sibling projects.

### Scope

**Small** if step 1 finds no toggle (doc bullet only). **Medium** otherwise
(one config field, one apply write, one status check, tests, live verify).

---

## Research Update (2026-09-08) — Step 1 resolved: the toggle exists

Confirmed in a live `voxi` session: Claude Code's Go and Rust diagnostics come
from two first-party **plugins**, not a hardcoded core feature —
`gopls-lsp@claude-plugins-official` and `rust-analyzer-lsp@claude-plugins-official`.
Each wires its language server into an automatic post-`Edit`/`Write` hook that
injects diagnostics as a `<system-reminder>` block with no explicit request
from the user or agent, on every edit.

The only lever found is a global `enabledPlugins` map in
`~/.claude/settings.json`:

```json
{
  "enabledPlugins": {
    "gopls-lsp@claude-plugins-official": false,
    "rust-analyzer-lsp@claude-plugins-official": false
  }
}
```

This was hand-set to `false`/`false` for this session as an immediate
workaround (outside any repo, not committed anywhere) after a batch of
auto-injected gopls diagnostics on a harnez file turned out to be stale/wrong
— `go build`/`go vet`/`go test` all passed cleanly — the same staleness
failure mode this ticket already documents. It reinforces this ticket's
existing framing: report the setting, don't default to disabling it (real
true-positive value exists), but make it a first-class, inspectable toggle
instead of a manual settings.json edit.

This is **distinct** from `docs/practices/AgenticLoop.md` §1 invariant 6
("Context Discipline & Range-Bounded Ingestion") — that principle is about
avoiding whole-file reads on active system-prompt files, not about LSP
diagnostics. Don't conflate the two when writing step 4's doc bullet.

Per-language granularity is now known to be free: since the two plugins are
already independent keys, a per-language toggle (matching `gopls`/
`rust-analyzer` separately) costs nothing extra over a blanket on/off — worth
weighing against a single flag when designing the `config.yaml` field in
step 2. Still undecided: whether anything finer-grained than per-plugin
on/off (e.g. severity-based suppression) is worth the complexity — no
evidence yet that it's needed beyond a blanket toggle.

**Unblocks**: step 1 is done; step 2 (add the config field + `apply` write
via `applySettingsJSON` in `internal/claude/apply.go:217`, following the flat
scalar/struct field pattern already used in `internal/claude/config.go`'s
`Config` struct, e.g. `StatusLine bool` / `FeedbackConfig`) can proceed
directly — no further toggle research needed. This is purely the harnez-side
tracking; no `~/.claude/settings.json` outside this repo was touched by this
update.

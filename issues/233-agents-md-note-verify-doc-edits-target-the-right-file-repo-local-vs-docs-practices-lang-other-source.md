# 233 — AGENTS.md note: verify doc edits target the right file (repo-local vs docs/practices|lang|other source)

**Status**: Draft
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Documentation
**Related**: `docs/practices/IssueTracking.md` / `docs/IssueTracking.md` (the exact pair this bit
this session), `config.yaml` (`docs:` section — `source`/`target`/`local` mapping for every
copyable doc), `AGENTS.md` (`Docs Layout` section — explains the category split but not the
repo-dogfoods-itself trap), `embed.go` (`//go:embed ... docs/lang docs/other docs/practices ...`
— why a stale binary silently re-clobbers a fresh source edit)

---

## 1. Problem & Motivation

This session edited `docs/IssueTracking.md` (repo root) to add a small Allowed-Values note. That
file turned out to be a **self-applied local copy** of `docs/practices/IssueTracking.md` — this
repo dogfoods its own `harnez init` on itself, so `docs/` root holds a mix of (a) this repo's own
evergreen docs and (b) local copies of `docs/lang|practices|other` sources installed the same way
any consuming project gets them (`config.yaml`'s `issue-tracking` entry: `source:
docs/practices/IssueTracking.md`, `local: ./docs/IssueTracking.md`).

The edit was committed to the wrong file. It would have been silently discarded on the next
`harnez init`/apply self-sync, with no error and no warning — `init` just overwrites the local
copy from source. The mistake was only caught because a later, unrelated task ran `harnez init
--docs issue-tracking` and the diff looked wrong.

A second wrinkle compounded this: docs are `//go:embed`-ed into the binary (`embed.go`), so even
after fixing the source file, the *first* resync attempt still failed — the installed/just-built
`harnez` binary was compiled before the source edit, so `init` recopied stale embedded content.
Had to `go build` again before the resync actually reflected the new source text.

`AGENTS.md`'s existing `Docs Layout` section documents the *category* split (root vs. `docs/lang`
vs. `docs/practices` vs. `docs/other`) but does not mention that root can also contain **local
copies of those same categories**, installed via this repo's own self-application — which is
exactly the trap that caused the mistake.

## 2. Deliverable

Add a short note to `AGENTS.md`'s `Docs Layout` section (or immediately after it) along these
lines:

- Before editing any file under `docs/` root, check `config.yaml`'s `docs:` entries for a
  `local: ./docs/<that-file>.md` mapping. If one exists, the file under `docs/` root is a
  **self-applied copy**, not the source — edit the `source:` file instead (typically under
  `docs/lang/`, `docs/practices/`, or `docs/other/`).
- After editing a `source:` file, remember docs are embedded at build time (`embed.go`) — `go
  build` before running `harnez init --docs <name> -d .` (or `harnez apply` for the global
  `~/.claude` copies) to resync, or the resync will silently recopy stale embedded content.
- `harnez apply` rewrites global `~/.claude` state shared by every session — do not run it
  autonomously as part of an unrelated task; confirm with the user first.

**Where exactly**: `AGENTS.md`'s `Docs Layout` section is itself a hand-maintained, repo-specific
section (not one of the `config.yaml`-generated managed blocks like `Issue Tracker Discovery` —
verify this before editing, per this very ticket's own point). If it turns out to be
config.yaml-managed after all, add the note there instead of hand-editing `AGENTS.md` directly.

## 3. Acceptance Criteria

- [ ] `AGENTS.md` (this repo's own, not a template) carries the note above, in whichever section
      is the correct owner (verified per §2's caveat, not assumed).
- [ ] No other repo's `AGENTS.md`/`CLAUDE.md` needs this — it's specific to harnez dogfooding
      itself, not a copyable-doc concern for consuming projects. Do not add it to
      `docs/templates/AGENTS.md` or any `docs/practices/*.md` source.

No implementation plan yet — filed to capture the idea from a concrete mistake made this session.

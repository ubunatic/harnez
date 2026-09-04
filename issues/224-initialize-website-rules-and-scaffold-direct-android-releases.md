# 224 — Website doc auto-installs for website-capable projects

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `docs/other/Website.md`, `internal/claude/init.go`, `config.yaml`, [[230-android-direct-release-scaffold-makefile-template-signing-checksums]] (split off — this ticket's original Part B), Smarthome issues 024/025

---

## 1. Problem & Motivation

Harnez exposes a website-building skill that requires every project agent to read
`docs/other/Website.md` before generating `website/` content. `harnez init` did not copy or bundle
that document into the Smarthome project, even though the canonical document exists in Harnez.
This left the skill unable to proceed and forced manual discovery in a sibling repository.

This ticket originally also covered a second gap (an Android direct-release scaffold) discovered
in the same session; that has been split into [[230]] since it shares no code path with the fix
below — only the motivating session was shared.

## 2. Findings

- `docs/other/Website.md` is a canonical Harnez document but is not currently included in the
  documented copyable `docs/lang`, `docs/practices`, or `docs/other` sets used by `init --docs`.
- The website skill explicitly blocks content generation when the project-local rules file is
  absent, so missing installation is a functional integration failure rather than optional docs.

## 3. Deliverables & Exit Criteria

- [ ] Make `docs/other/Website.md` auto-install: `harnez init` installs it whenever a project is
      website-capable (has a `website/` directory), without requiring the operator to know the
      `--docs website` flag.
- [ ] Add an init/smoke regression proving a newly initialized website-capable project has the
      exact rules document required by the website skill.
- [ ] Give the website skill a fallback for projects that skip re-init: if the project-local copy
      is missing, read the harness copy and tell the user to run `harnez init --docs website`.
- **Exit criteria**: a fresh website-capable project initialized by Harnez can follow the website
  skill without missing docs.

---

## Implementation Plan

### Finding that reshapes the ticket

`docs/other/Website.md` **is already copyable**: `config.yaml` (~line 489) defines a
`website` doc entry (`source: docs/other/Website.md`, `target:
~/.claude/docs/Website.md`, `local: ./docs/Website.md`) with `default: false`,
installable via `init --docs website` / `apply --docs website`. Deliverable 1 is
therefore not "make it copyable" but "make it *install itself* when the project
is website-capable, instead of requiring the operator to know the flag." That is
a two-line change against existing machinery.

### Steps

1. **`config.yaml`** — change the `website` entry's `default: false` to
   `default: auto`, and update the trailing comment (which currently explains the
   `false`) to say the doc auto-installs when a `website/` dir exists, while the
   Agent Skill still covers the "build me a site" trigger for projects that have
   none yet.
2. **`internal/claude/init.go`** — add a case to `detectDoc` (~line 186):
   `case "website": return dirExists(filepath.Join(dir, "website"))`. Add a
   small `dirExists` helper next to `fileExists`/`globExists` (`os.Stat` +
   `IsDir`), since a stray `website` *file* should not trigger it.
3. **`internal/claude/init_test.go`** — add a test asserting that
   `autoDetectDocs` returns `website` for a temp dir containing `website/` and
   does not for one without; and an end-to-end `RunInit` assertion that
   `docs/Website.md` exists in the project afterwards and that the Language
   Conventions block gained the `@docs/Website.md` ref. This is deliverable 2's
   "init regression proving a website-capable project has the exact rules
   document the skill requires".
4. **`scripts/smoke-test.sh`** — extend the init leg with the same check
   (create `website/`, run `init`, assert `docs/Website.md` landed), if the
   script already has an init stage; otherwise skip and rely on step 3 rather
   than growing the smoke script for this.
5. **`~/.claude/skills/website/SKILL.md` (source: `commands/website.md`)** —
   the skill currently hard-requires reading `docs/Website.md` and blocks when
   absent. Add an explicit fallback instruction: if the project-local copy is
   missing, read the harness copy at `~/.claude/docs/Website.md` and tell the
   user to run `harnez init --docs website` to make it project-local. This
   removes the hard failure mode even for projects that skip step 1–2 entirely,
   and is arguably the highest-value single line in this ticket.
6. **`docs/README.md`** — update the `other/Website.md` row: it is now
   auto-installed on `website/`, not `default:false` opt-in only.
7. Run `go test ./internal/claude/...`, `go test ./...`, `make check`,
   `make install`. Verify by `harnez init --dir <tmp with website/>`.

### Design decisions / tradeoffs

- **`auto` over `true`**: issue 130 deliberately removed it from every project's baseline
  AGENTS.md. `auto` restores installation exactly where it is needed without reintroducing that
  cost.
- **Skill fallback (step 5) is independent of init**: it fixes the reported functional failure
  ("skill unable to proceed") even in repos nobody re-inits. Worth landing even if steps 1–4 slip.

### Risks / open questions

- `default: auto` for `website` will silently add `docs/Website.md` to any existing project with a
  `website/` dir on its next `harnez init` — desirable, but it is a fleet-visible behavior change;
  confirm before shipping.

### Scope

**Small.**

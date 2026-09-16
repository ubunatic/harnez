# 365 — Add `harnez release --init=<lang|mode>` to bootstrap release scaffolding

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: `docs/practices/GoRelease.md`, `docs/lang/ManPages.md`, [[091-language-agnostic-release-spec-and-thin-make-release]], [[230-android-direct-release-scaffold-makefile-template-signing-checksums]]

---

## 1. Problem & Motivation

`harnez init --docs golang` scaffolds `version.yaml` (and updates `AGENTS.md` to reference
`docs/GoRelease.md`) but stops there — it does not scaffold the rest of what
`docs/GoRelease.md`'s "Project Setup Checklist" (§2) requires before `harnez release`
can actually succeed: `.goreleaser.yaml`, a minisign key pair, and (per Go convention)
the Cobra `Version` wiring / `version.go` sync target.

Concretely hit in a session against `ubunatic/voxi-clients` (2026-09-16): after
`harnez init --docs golang` scaffolded `version.yaml` referencing
`spec/schemas/version.schema.json` (a file that also doesn't get scaffolded — see
Findings), the agent had to hand-wire `version.go` + Cobra `Version:` itself, and running
`harnez release -h` surfaced that the build step (`goreleaser` or `make dist`) and
minisign signing step would both fail outright — no `.goreleaser.yaml` exists, and no
`~/.minisign/voxi-clients.key`. The gap between "docs describe 4 setup steps" and
"init scaffolds 1 of them" means every project's first `harnez release` currently
requires a human/agent to manually work through `docs/GoRelease.md` line by line.

This also matters for non-Go project shapes: `docs/GoRelease.md` §5 says the pipeline is
language-agnostic (Python/Zig/Rust/scripted), and 230 already covers a divergent,
heavier Android scaffold. A generic `--init` entry point should route to the right
scaffold per project shape rather than assuming Go everywhere.

## 2. Findings

- `docs/GoRelease.md` §2 lists 4 required setup artifacts: `version.yaml` (+ its schema),
  Cobra version wiring, a minisign key, `.goreleaser.yaml`. Only `version.yaml` itself is
  currently scaffolded by `harnez init --docs golang`, and even that omits the
  `spec/schemas/version.schema.json` its own header references.
- `docs/lang/ManPages.md` (also pulled in by `--docs golang`) describes a 3-tier man-page
  pattern (`<cmd> man`, Makefile `make man`, packaging) that's likewise documentation-only
  today — no scaffold wires a `man` subcommand into a fresh Cobra root command.
- 230 is a distinct, heavier Android scaffold (APK/AAB, keystores, `apksigner`) — this
  ticket's Go/Golib MVP should not duplicate that work, just cover the common Go CLI case
  091 already established the language-agnostic release *spec* for.
- Repos are not always single-binary-at-repo-root shaped (e.g. `voxi-clients`: multiple
  disposable Go tools under one root module, one of which — `whisper-server` — is the
  actual release target). `--init` needs to either ask/detect which package is "the"
  release artifact, or accept an explicit target flag, rather than assuming `cmd/<project>`
  or root `.` unconditionally.

## 3. Deliverables & Exit Criteria

- [ ] `harnez release --init[=<mode>]` (or a separate `harnez release init` subcommand —
      naming TBD during implementation) that scaffolds the missing setup artifacts for a
      project that doesn't have them yet, idempotently (safe to re-run, like `harnez init`).
- [ ] `mode`/`lang` values: `auto` (detect from repo contents; default), `Go`, `Golib`
      (library-only Go project — skip binary-specific steps like `.goreleaser.yaml`
      binary builds and man pages), `Android` (delegates to / aligns with 230's scaffold
      once that lands — do not duplicate its design here).
- [ ] MVP scope: `auto` resolves to `Go` detection only (existing `go.mod` at or below repo
      root); other lang branches can be stubs that report "not yet implemented" rather than
      silently no-op.
- [ ] For the Go path, scaffold: `spec/schemas/version.schema.json` alongside `version.yaml`,
      a minisign key at `~/.minisign/<project>.key` if absent (respecting `-s`/`--sign-key`),
      and a `.goreleaser.yaml` from the template in `docs/GoRelease.md` §2.4 — with the
      `main:` path filled from detected/confirmed package location, not hardcoded to
      `./cmd/<project>` or `.`.
- [ ] Must not silently overwrite an existing `.goreleaser.yaml`/minisign key/`version.yaml`
      customized by the project — same "exists (unchanged)" vs "wrote"/"scaffolded" reporting
      style as `harnez init`'s existing output.
- [ ] Decide and document whether Cobra `Version:`/man-subcommand wiring in the target
      Go source file is scaffolded automatically (risk: touching hand-written `main.go`) or
      left as a documented manual step with a clear pointer — err toward the latter unless
      there's a safe, minimal-diff way to do it.
- [ ] `harnez release -n` (dry-run) after `--init` on a fresh project should get further
      than "no `.goreleaser.yaml`" before hitting whatever's next (real network/signing
      steps), proving the scaffold is actually sufficient.

## 4. Verification Guidance

- Run `harnez release --init` (dry, if that's supported, or in a throwaway repo) against a
  project shaped like `voxi-clients` (root `go.mod`, no `cmd/` layout, one of several `main`
  packages being the actual release target) and confirm it produces a working
  `.goreleaser.yaml` + minisign key + schema, then `harnez release -n` proceeds past setup
  validation.
- Confirm idempotency: running `--init` twice on an already-scaffolded project reports
  "exists (unchanged)" for everything and makes no changes.
- Confirm it does not clobber a hand-customized `.goreleaser.yaml` (e.g. one with an
  extra build target) from a project that already adopted the convention manually.

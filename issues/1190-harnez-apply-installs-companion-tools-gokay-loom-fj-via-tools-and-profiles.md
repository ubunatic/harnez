# 1190 — harnez apply installs companion tools (gokay, loom, fj) via --tools and --profiles

**Status**: Closed — apply --tools/--profiles/--upgrade-tools (a587d6c1); argon:low review fixed; make test-q1 exit 0; deferred: remembered profiles, diff/status --profiles
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: 1068, 898, docs/CLIDesign.md

---

## Goal

/goal `harnez apply --tools gokay,loom,fj` installs each listed companion tool with its known
installer (`go install`, `cargo install`, ...), and `--profiles go,codeberg,...` selects named groups
of docs and tools. A missing installer or a failed install prints a warning and apply continues. Done
when the tool catalogue and profiles live in `spec/` with a schema, apply installs and warns as
described, and tests cover success, missing installer and failed install with a fake installer. Stop
and report when blocked on a user decision or denied permission; on a loop or no progress, clear the
goal and write a failure report here.

## Problem

Companion tools such as gokay are recommended in the docs (898) but installed by hand on every
machine. No `apply` flag installs them; `--components` and `--docs` cover only harnez's own files.

## Scope

- `spec/tools.yaml` (+ schema): per tool name, installer (`go`, `cargo`, ...), install argument
  (module/crate path and version), and the binary to check on PATH. Starting set: gokay, loom, fj;
  verify each install source before listing it.
- `spec/profiles.yaml` (+ schema), or a section in the same spec: profile name → docs and tools, for
  example go, rust, codeberg, github, web.
- `apply --tools <list>` and `apply --profiles <list>`; `--docs` keeps working and merges with
  profile docs. Read `docs/CLIDesign.md` first: this is global install, so it belongs in `apply`,
  not `init`.
- Skip a tool already on PATH unless asked to upgrade; never run an installer that is not on PATH.

## Profiles absorb --components

`--components` (internal/claude/components.go, docs/HarnezComponents.md) picks which parts of harnez
itself apply installs: docs, skills, telemetry, agents, usage, with presets full, docs-only,
telemetry-only, agents-only. No script, Makefile or Containerfile in this repo uses it. Profiles become the
user-facing selector:

- A profile may also name components, so `docs-only` or `minimal` become profiles.
- Selected profiles are stored in config.yaml the way `components:` is, so a later plain `apply`
  keeps them (this answers the "remembered" question below).
- `--components` keeps working but is hidden from `--help`; deprecate it once profiles cover the
  presets. The component registry stays as the install/prune/diff engine under profiles.

## Delivered

- `spec/tools.yaml` + schema: gokay, loom (go install @latest), fj (cargo install forgejo-cli), installers,
  profiles go, rust, codeberg, github, minimal (web dropped: placeholder until the website skill overhaul).
- `apply --tools`, `--profiles`, `--upgrade-tools`; `--components` hidden from help.
- Decisions (decide mode): one `--tools` flag, not per-ecosystem; `@latest`, no pinning yet.
- Review (argon:low): schema validation + strict decode + duplicate check at load, spec loaded once,
  mixing component-narrowing profiles refused, installer timeout, CLI wiring test. Deferred: `diff`/`status --profiles`.
- Deferred: remembering profiles in config.yaml (needs a writable user config; plain apply uses the
  embedded one). Follow-up when wanted.

## Open questions

- One flag (`--tools`) or per-ecosystem flags (`--go-tools`, `--rust-tools`)? The ticket assumes
  `--tools`, since the spec already knows each tool's installer.
- Should `--profiles` be remembered (like opted-in docs) so a later plain `apply` keeps them?
- Relation to 1068 (role profiles pinning docs, skills and tools): same word "profile"; decide whether
  these merge or need distinct names.
- Pinned versions vs `@latest`.

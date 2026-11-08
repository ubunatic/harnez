# 1190 — harnez apply installs companion tools (gokay, loom, fj) via --tools and --profiles

**Status**: Open
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

## Open questions

- One flag (`--tools`) or per-ecosystem flags (`--go-tools`, `--rust-tools`)? The ticket assumes
  `--tools`, since the spec already knows each tool's installer.
- Should `--profiles` be remembered (like opted-in docs) so a later plain `apply` keeps them?
- Relation to 1068 (role profiles pinning docs, skills and tools): same word "profile"; decide whether
  these merge or need distinct names.
- Pinned versions vs `@latest`.

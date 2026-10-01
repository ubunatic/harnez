---
title: Language Doc Pipeline
weight: 40
---

# Language Pipeline

How `--docs <name>` flows from `config.yaml` into a project via `apply` (global) and `init` (project-local).

## Harness and project outputs per doc

| Field | Command | Destination | Behaviour |
|-------|---------|-------------|-----------|
| `source` | `apply` | `~/.claude/docs/<name>.md`, `~/.prime/agent/docs/<name>.md` | Installed globally unless an existing custom doc is preserved |
| `lite_source` | `apply`/`init` | same as `source` | Optional tagline-only variant of `source`. `Language.SourceFor(variant)` resolves `lite_source` when `variant == "lite"` and it's set, else falls back to `source`. The installed copy's first-line `<!-- harnez:variant=lite -->` marker records the choice; `harnez docs variant <name> lite\|full` switches it (issue 360). |
| `local` | `init` | `<project>/docs/<name>.md` | Written/updated on every init |
| `template` | `init` | `<project>/Makefile` (basename of path) | Written **once** — skipped if file exists |
| `targets` | `init` | `<project>/Makefile` (managed section) | Injected/updated — skipped if template was just scaffolded |

`source` (global install) is driven by `apply --docs <name>` or the top-level `docs:` list in `config.yaml`. Prime copies are enabled by `prime_agent_target`.
`local`, `template`, and `targets` (project-local) are driven by `init --docs <name>` or auto-detection.

## Auto-detection (`default:` field)

Each doc entry in `config.yaml` has a `default:` field controlling whether `init` copies it without an explicit `--docs` flag:

| Value | Behaviour |
|-------|-----------|
| `true` | Always copied on `init` (e.g. git, markdown, canary) |
| `false` | Only if explicitly requested via `--docs` (e.g. gtk4) |
| `auto` | Copied when a project signal is detected (e.g. `go.mod` → golang, `Makefile` → make) |

Detection heuristics (`detectDoc` in `init.go`):
- `golang` — `go.mod` exists
- `bash` — `*.sh` files in project root or `scripts/`
- `make` — `Makefile` exists
- `rust` — `Cargo.toml` exists
- `zig` — `build.zig`, `build.zig.zon`, or `*.zig` exists
- `cpp` — `*.cpp`, `*.cc`, `*.h`, or `CMakeLists.txt`

## Template vs targets

**Template** (`docs/templates/Makefile`) — full scaffold for new projects:
- Written when the destination file does not exist
- Never overwritten — hands ownership to the user immediately
- Contains everything: phony sentinel, help, build, install, test, clean

**Targets** (`docs/templates/MakeTargets.mk`) — managed section for existing projects:
- Injected as a `# harnez:begin targets` … `# harnez:end targets` block
- Appended if the block is absent; updated in-place if present (idempotent)
- Skipped when the template was just scaffolded in the same run (avoids duplication)
- Contains only the universal targets: ⚙️ phony sentinel + self-documenting `help`

This lets users adopt conventions gradually: projects with no Makefile get the full
scaffold; projects with an existing Makefile get just the key targets injected.

**Legacy block migration & CLAUDE.md adoption:**
- Projects with legacy `# claudeconfig:begin ...` or `<!-- claudeconfig:begin ... -->` markers are automatically upgraded to `harnez:` on `init`.
- If a project contains a regular `CLAUDE.md` without an `AGENTS.md`, `init` safely renames `CLAUDE.md` → `AGENTS.md` before establishing the `CLAUDE.md -> AGENTS.md` symlink, preserving all custom project rules.

## Markers abstraction

The managed-section mechanism (`internal/markdown`) is parameterised by a `Markers`
struct so the same apply/diff/clean logic works for different comment styles:

```go
MDMarkers  // <!-- harnez:begin NAME --> — Markdown / AGENTS.md
MKMarkers  // # harnez:begin NAME       — Makefiles
```

Functions: `Apply`/`Diff`/`Clean`/`ContainsSection` (MD) and
`ApplyMK`/`DiffMK`/`CleanMK`/`ContainsSectionMK` (MK).

Adding support for a new file type (e.g. TOML `# …`, YAML `# …`, JSON `// …`) requires
only a new `Markers` value — no logic changes.

## Lite variants and root copies

A lite and a full variant install to the same `local` path, so in a consumer repo a lite doc
cannot link to "the full doc" by path: that link points back at itself (issue 667). A lite doc
names the switch instead, using the `config.yaml` key, not the file name:
`harnez docs variant issue-tracking full`.

This repo is also a consumer: `docs/*.md` holds init-written copies of the copyable sources in
`docs/lang/`, `docs/practices/` and `docs/other/`. Edit the source, then run `harnez init -d .`
(after `make install`) and `git checkout --` any unrelated rewrite. A root-only edit is silently
lost on the next init; issue 666 added the `HTO=0` rule to `docs/AgenticLoop.md` and
`docs/Bash.md` only, and the follow-up sync would have deleted it.

`TestRootDocCopiesMatchSources` (`internal/claude/root_docs_sync_test.go`) guards this. For every
root copy with a `harnez:stop` marker, it picks the source by the copy's variant marker and
requires the copy to equal what init would write (`prepareManagedDoc`). All copyable root docs
in this repo's init doc set, including `ConciseMode.md`, `Containerfile.md`, and `Website.md`,
are checked; no root copy is currently skipped (issue 669).

## Lint check

`make lint` / `scripts/lint.sh` verifies that every `commands/*.md` file has a matching
`- name:` entry in `config.yaml`. Runs forward only (file → config); the reverse direction
(config entry → file exists) is enforced by `apply`, which errors on missing `file:` paths.

## Known gaps

- `diff` and `revert --managed` do not handle the Makefile targets section — see issue #009.
- `status` does not check whether the targets block is present in the project Makefile.

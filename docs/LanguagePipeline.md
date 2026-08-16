---
title: Language Doc Pipeline
weight: 40
---

# Language Pipeline

How `--docs <name>` flows from `config.yaml` into a project via `apply` (global) and `init` (project-local).

## Four outputs per doc

| Field | Command | Destination | Behaviour |
|-------|---------|-------------|-----------|
| `source` | `apply` | `~/.claude/docs/<name>.md` | Always written/updated (global) |
| `local` | `init` | `<project>/docs/<name>.md` | Written/updated on every init |
| `template` | `init` | `<project>/Makefile` (basename of path) | Written **once** — skipped if file exists |
| `targets` | `init` | `<project>/Makefile` (managed section) | Injected/updated — skipped if template was just scaffolded |

`source` (global install) is driven by `apply --docs <name>` or the top-level `docs:` list in `config.yaml`.
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

**Legacy block migration:** projects that were set up before the migration may have a
`# claudeconfig:begin targets` / `# claudeconfig:end targets` (or `# harnez:begin targets`) marker block from the old
injection style. `ReconcileMakeTargets` detects and strips this block automatically before
structural reconciliation runs, so old projects self-migrate on the next `init`.

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

## Lint check

`make lint` / `scripts/lint.sh` verifies that every `commands/*.md` file has a matching
`- name:` entry in `config.yaml`. Runs forward only (file → config); the reverse direction
(config entry → file exists) is enforced by `apply`, which errors on missing `file:` paths.

## Known gaps

- `diff` and `clean` do not yet handle the Makefile targets section — see issue #009.
- `status` does not check whether the targets block is present in the project Makefile.

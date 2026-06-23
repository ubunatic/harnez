# Language Pipeline

How `-l <lang>` flows from `config.yaml` into a project when `apply -p <dir>` is run.

## Four outputs per language

| Field | Destination | Behaviour |
|-------|-------------|-----------|
| `source` | `~/.claude/docs/<name>.md` | Always written/updated (global) |
| `local` | `<project>/docs/<name>.md` | Written/updated on every apply |
| `template` | `<project>/Makefile` (basename of path) | Written **once** — skipped if file exists |
| `targets` | `<project>/Makefile` (managed section) | Injected/updated — skipped if template was just scaffolded |

`source` and `local` are always overwritten to keep docs in sync.
`template` and `targets` are project-setup concerns with different write semantics.

## Template vs targets

**Template** (`docs/templates/Makefile`) — full scaffold for new projects:
- Written when the destination file does not exist
- Never overwritten — hands ownership to the user immediately
- Contains everything: phony sentinel, help, build, install, test, clean

**Targets** (`docs/templates/MakeTargets.mk`) — managed section for existing projects:
- Injected as a `# claudeconfig:begin targets` … `# claudeconfig:end targets` block
- Appended if the block is absent; updated in-place if present (idempotent)
- Skipped when the template was just scaffolded in the same run (avoids duplication)
- Contains only the universal targets: ⚙️ phony sentinel + self-documenting `help`

This lets users adopt conventions gradually: projects with no Makefile get the full
scaffold; projects with an existing Makefile get just the key targets injected.

## Markers abstraction

The managed-section mechanism (`internal/markdown`) is now parameterised by a `Markers`
struct so the same apply/diff/clean logic works for different comment styles:

```go
MDMarkers  // <!-- claudeconfig:begin NAME --> — Markdown / AGENTS.md
MKMarkers  // # claudeconfig:begin NAME       — Makefiles
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

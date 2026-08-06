# 017 — `docs/Spec.md` (bundled, `default: true`) is Cati-specific, not a generic guide

**Status:** Open

## Context

Found while auditing a downstream project's (wayreel) `docs/` during an
`/evergreen` pass. `docs/Spec.md` is bundled into every project by default
(`config.yaml`: `spec: { default: true }`), and its content
(`docs/other/Spec.md`, mirrored verbatim at `docs/Spec.md`) is titled
**"Cati Spec System — Authoritative Reference"**.

The body is not a generalized "specs are the single source of truth"
principle doc — it's a literal copy of the *Cati* app's own internal
implementation docs:

- File map hardcodes Cati's actual spec files: `spec/style.yaml`,
  `spec/buttons.yaml`, `spec/views.yaml`, `spec/theme.yaml`,
  `spec/controls.yaml`, `spec/config.yaml`, `spec/about.yaml` — none of
  these exist in most projects (e.g. wayreel only has `spec/keys.yaml`).
- References Cati-specific Go function names (`loadButtons`,
  `resolveKeyName`, `loadButtonKeyDefs`, `buildViewKeyMaps`,
  `viewKeyAction`, `drawBottomMenu`, `drawHintBar`) and concepts
  (`hidden_keys:` rows, the TUI button/keyboard dispatch pipeline) that are
  meaningless outside Cati.
- Required test names in §6 (`TestSpecButtonsLoad`, `TestSpecViewsLoad`,
  etc.) assert against Cati's own loader functions.

In a project like wayreel (whose actual spec-driven mechanism is
`spec/keys.yaml` mapping vim-style key notation to xkb/xdotool sequences —
completely unrelated to buttons/views/theme), this doc reads as either
confusing (references files/functions that don't exist) or gets silently
ignored. Either way it fails at its stated purpose (steering agents toward
"spec is single source of truth" discipline) because the concrete guidance
doesn't transfer.

Compare with `docs/Canary.md` (also `default: true`), which *is* written at
the right level of generality — "write a canary before building on an
external mechanism" applies to any project, with examples drawn from
multiple different projects rather than one app's internals.

## Proposal

Rewrite `docs/other/Spec.md` to state the general principle only, with
Cati's file map/function names either removed or clearly marked as *one
concrete example*, not the reference shape every project must match:

- Keep: "spec files are the single source of truth; app code must not
  duplicate/shadow spec values" + the categories of violations (hardcoded
  fallback maps, hardcoded keys/actions that duplicate spec content) —
  these generalize fine.
- Drop or move to a Cati-specific example doc: the §2 file map, §3 key
  concepts (buttons.yaml/views.yaml/dispatch pipeline), §6's Cati test
  names, §7's Cati-specific checklist items ("Added button to
  `buttons.yaml`", "Updated `docs/Design.md` section 3").
- If per-project detail is valuable, consider auto-detecting a project's
  actual `spec/*.yaml` + `spec/schemas/*.json` layout at `init` time and
  generating the file map section from what's actually present, rather
  than hardcoding Cati's.

## Why this matters

`default: true` means every new project gets this doc without opting in.
A bundled default doc that's actually project-specific either misleads
agents (if they take the Cati details literally) or trains users to ignore
bundled docs as boilerplate noise — undermining the other `default: true`
docs (`Canary.md`, `Git.md`, etc.) that *are* correctly generic.

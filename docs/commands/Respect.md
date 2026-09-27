---
name: respect
description: Move hardcoded constants and parameters of a requested repository, module, or package into the embedded YAML spec
disable-model-invocation: true
---

# Respect the Spec

Bring implementation in line with the repository's YAML spec. The spec is the
YAML under `spec/` (validated by `spec/schemas/*.schema.json`), embedded at
compilation and included in the binary. It is the single source of truth for
values the application uses. It is **not** prose documentation and **not**
runtime user configuration; do not create config files, flags, or env vars as
a substitute.

Read `docs/Spec.md` before starting; it defines layout, schema, and
consumption rules.

## Scope

- Work only on the repository, module, or package the user names. If none is
  named, ask once which scope to cover.
- Include constants, parameters, thresholds, limits, durations, labels,
  messages, IDs, keys, action names, and fallback tables that describe
  application behaviour or content.
- Exclude values that are implementation mechanics rather than spec content:
  loop indices, buffer sizes tied to an algorithm, protocol constants defined
  by an external standard, test fixtures, and values used once for local
  arithmetic.

## Workflow

1. **Explore.** Read the existing `spec/*.yaml` files, their schemas, and the
   code that loads them (e.g. `//go:embed` in `embed.go` or package-local
   `spec/` embeds). Then scan the requested scope for hardcoded values with
   `harnez find code` (fall back to `rg`).
2. **Classify.** For each candidate, decide: belongs in an existing spec
   file, needs a new spec file, or stays in code (with the reason). Also flag
   code that shadows or duplicates a value already in the spec.
3. **Report before editing.** List the candidates as a short table: location,
   value, target spec file and key, or "stays" with reason. Stop and ask when
   the spec structure is unclear, when ownership of a value is ambiguous, or
   when a new spec file would be needed.
4. **Move.** For each accepted value:
   - Add it to the spec YAML and declare it in the matching JSON Schema.
   - Replace the hardcoded value with a read from the embedded spec. Do not
     keep a hardcoded fallback that mirrors the spec value.
   - If a new spec file is needed, add its schema and embed it the same way
     existing spec files are embedded.
5. **Verify.** Add or extend a test that loads the embedded spec and asserts
   the moved keys exist and are consumed. Run the repository's test and build
   targets (and `make install` when provided) and report the results.

## Report

End with: the values moved (spec file and key each), values deliberately left
in code with the reason, and test/build results. Do not commit unless the
user asks.

# Switch ConciseMode & Sync AGENTS.local.md Overlay

Dynamically switch the operational terseness level for the active session and synchronize `./AGENTS.local.md` (or `./AGENTS.md` with `--persist`).

Reference Practice: `@docs/practices/ConciseMode.md`

---

## Invocation Syntax

- `/mode lite` (or `/mode 1`) — Concise Lite: Professional Terse, no fluff, complete sentences
- `/mode std` (or `/mode 2`) — Concise Standard: Telegraphic fragments, Action -> Finding -> Patch
- `/mode ultra` (or `/mode 3`) — Concise Ultra: Diffs/status only, zero narrative
- `/mode off` (or `/mode reset`) — Disable ConciseMode / restore default narrative

Flags:
- `-e`, `--ephemeral`, `--no-file` — In-session directive only; do not write any file to disk
- `--persist`, `--main` — Write to `./AGENTS.md` instead of `./AGENTS.local.md`
- `-f`, `--file <path>` — Target specific instructions file

---

## Behavior

1. Executes `harnez mode <tier>` to inject the high-priority steering directive into the immediate LLM context.
2. Updates or cleans the `<!-- harnez:begin Concise Mode -->` block in `./AGENTS.local.md` (or `./AGENTS.md` with `--persist`) so descendant subagents and subsequent sessions inherit the tier automatically.
3. Automatically ensures `AGENTS.local.md` is added to `.git/info/exclude` so switching modes never leaves a dirty git working tree.


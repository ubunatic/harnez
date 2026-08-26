# Switch ConciseMode & Sync AGENTS.md

Dynamically switch the operational terseness level for the active session and synchronize `./AGENTS.md`.

Reference Practice: `@docs/practices/ConciseMode.md`

---

## Invocation Syntax

- `/mode lite` (or `/mode 1`) — Concise Lite: Professional Terse, no fluff, complete sentences
- `/mode std` (or `/mode 2`) — Concise Standard: Telegraphic fragments, Action -> Finding -> Patch
- `/mode ultra` (or `/mode 3`) — Concise Ultra: Diffs/status only, zero narrative
- `/mode off` (or `/mode reset`) — Disable ConciseMode / restore default narrative

---

## Behavior

1. Executes `harnez mode <tier>` to inject the high-priority steering directive into the immediate LLM context.
2. Updates or cleans the `<!-- harnez:begin Concise Mode -->` block in `./AGENTS.md` so descendant subagents and subsequent sessions inherit the tier automatically.

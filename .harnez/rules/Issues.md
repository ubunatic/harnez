# Harnez Issues

For a project with an `issues/` tracker. Use `harnez find` / `harnez issues`, not `ls issues/`,
`find` or grep.

- `harnez find -d <repo> issues -a status:active` — active tickets (Open, In Progress, Blocked,
  Draft). `status:backlog` lists parked ones, which active lists leave out.
- `harnez find -d <repo> issues "<query>"` — fuzzy search in titles and bodies.
  `harnez issues show -d <repo> <n>` — one ticket.
- `harnez find -d <repo> issues next` — next free number (read-only).
- `harnez issues new -d <repo> "<title>"` — run it right away: it reserves the number and
  creates the skeleton. Fill the printed file and keep its schema, run `harnez index -d <repo>`,
  commit ticket and index.
- `harnez issues <verb> -d <repo> <n> [reason]` — change status, resync the index and commit in
  one call. `close` needs a commit outside `issues/` that references the number, or `--no-code`;
  correct a closed ticket's reason with `close --amend-reason`.
- `issues/README.md` is the generated active index; it links `issues/BACKLOG.md` and
  `issues/CLOSED.md`.
- Commit docs and `issues/*.md` changes immediately; do not batch them behind code work.
- Cloud agents: `harnez issues new --remote -d <repo> "<title>"` (automatic when
  `CLAUDE_CODE_REMOTE` is set) allocates permanent GH IDs through authenticated gh; no local
  fallback.

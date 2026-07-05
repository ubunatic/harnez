# 015 — AGENTS.md must teach agents about uman (workspace glue)

**Status:** Open

## Context

No managed AGENTS.md mentions `uman`, yet it is the load-bearing workspace
tool: cross-project publishing (`uman website sync <project>`), push
orchestration, LFS manifests (`uman manifests`), and the `~/projects/.uman.toml`
config with `main_repo` and post-sync hooks.

Evidence (books session, 2026-07-05): an agent building the books library
website wrote a custom deploy script targeting `../ubunatic.com` directly and
had to be redirected by the user to `uman website sync` — the convention was
undiscoverable from any AGENTS.md, CLAUDE.md, or docs/ file, in a workspace
whose own doctrine is canary-first ("probe external mechanisms before
building features on them"). See books
`docs/adr/0002-website-publishing-via-uman-sync.md` for the outcome.

## Proposal

- Bundle a short "Workspace" section (or a registry doc, e.g.
  `docs/Workspace.md`) into managed AGENTS.md files, roughly:
  > Cross-project operations (publish, push, status, LFS manifests) go
  > through `uman` — see `~/projects/.uman.toml`. Project websites publish
  > via `uman website sync <project>` from the project's `website/` dir.
- Keep it one paragraph: enough for an agent to probe `uman --help` before
  inventing its own mechanism.
- Until bundled, projects carry the note manually (books and ubunatic.com
  AGENTS.md were adjusted by hand on 2026-07-05 — reconcile when bundling).

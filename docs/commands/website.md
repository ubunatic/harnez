# Build or Update Project Website

Triggered by requests like "build a website for this project", "update the
website", or "add a website for &lt;project&gt;".

## Before writing anything

1. Read `docs/Website.md` (or `@docs/Website.md` if already bundled in
   AGENTS.md) in full. It is the rules doc for this task — hosting
   assumptions, branding, content honesty, subpage/link constraints, and the
   static/no-CDN requirement are not optional.
2. Look at sibling projects' `website/` dirs (see `~/projects/.uman.toml` for
   the managed list) for prior art. If it isn't clear whether this project's
   site should be fancy or simple, ask the user which sibling project to use
   as inspiration — don't guess.

## While building

- Follow every rule in `docs/Website.md`: no GitHub assumptions/octocats
  unless asked, no unproven feature claims, a "Why" section, relative links
  only, static output, vendored (not CDN-pulled) JS assets, and no invented
  JS/TUI demo unless explicitly requested.
- Keep output under this project's `website/` dir so it is picked up by
  `uman website sync <project>`.

## Before publishing

- If the change includes new media (screenshots, recordings), get the
  user's explicit confirmation that it matches expectations before
  publishing or syncing (see global media-verification rule).
- Don't run `uman website sync` or push unless the user asks — building the
  content is the task; publishing is a separate, confirmed step.

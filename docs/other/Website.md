---
title: Website Building Rules
weight: 25
---

<!-- harnez:bundled -->
# Website Building Rules

Rules for any agent asked to build or update a project website (`website/` dir,
published via `uman website sync <project>` into `~/projects/ubunatic.com`).
Read this doc **before** writing any website content — it is not optional
background, it is the spec for the task.

---

## Hosting & branding

- **Do not assume GitHub.** The default host is Codeberg. Never add GitHub
  badges, "View on GitHub" links, or GitHub icons/octocats unless the project
  is explicitly hosted there and the user asked for it.
- **No octocats or GitHub-brand assets** unless explicitly requested — even
  as a generic "source code" icon.

---

## Match sibling projects, don't invent a style

- Before designing, look at sibling projects under `~/projects/*/website/`
  (see `.uman.toml` for the managed list). Some are fancy, some are plain —
  there is no single house style to assume.
- If it isn't clear which style fits (fancy vs. simple), **ask the user**
  which sibling project should act as the inspiration. Don't guess.

---

## Content honesty

- **Do not promise what cannot be delivered.** Only describe features that
  are implemented and proven — well-tested, not aspirational or planned.
- **Always include a "Why" section**: the problem being solved, and why this
  tool/approach was chosen to solve it. This is the anchor for the rest of
  the page — write it first, or at least before publishing.

---

## Subpage awareness — relative links only

- Assume this page may be mounted as a subpage of another site
  (`ubunatic.com` provides shared navigation and a top bar via
  `uman website sync` / `make inject-nav`).
- **Use relative links**, not absolute paths, so the page still works when
  hosted under a subdirectory (e.g. `ubunatic.com/<project>/`).
- Don't build your own top nav/header that would conflict with or duplicate
  the hosting site's navigation.

---

## Static only, no CDN pulls

- Websites are **static** — no server-side rendering, no build-time
  dependency on a running backend.
- **Do not pull JavaScript from a CDN.** If a library is genuinely needed
  (e.g. Mermaid for diagrams, a Markdown renderer), vendor a pinned, stable
  version as a local static asset instead of a `<script src="https://...">`
  reference.

---

## JS demos / TUI simulations — opt-in only

- Local JavaScript demos/simulations (e.g. a simplified in-browser replica
  of a terminal UI) are **optional** for most websites — don't add one
  unless asked.
- If asked, base the simulation on the real app's actual TUI/behavior —
  **do not invent features that don't exist** in the real app. Look at the
  actual terminal UI/app first, then build a smaller, faithful simulation.

---

## Anti-patterns

**Assuming GitHub hosting by default.** Check the project or ask — Codeberg
is the default in this workspace.

**Designing from a blank slate.** Sibling `website/` dirs already encode
working decisions (nav injection, subdir hosting, static asset patterns) —
reuse them instead of reinventing.

**Marketing copy for unproven features.** If it isn't tested, it doesn't go
on the website yet.

**Absolute-path navigation.** Breaks the moment the page is mounted as a
subpage under `ubunatic.com/<project>/`.

**A CDN `<script>` tag "just for now".** Vendor the asset instead — it's the
same effort and doesn't create an untracked external dependency.

**An unrequested TUI simulation with imagined features.** Demos are opt-in
and must reflect the real app, not an idealized version of it.

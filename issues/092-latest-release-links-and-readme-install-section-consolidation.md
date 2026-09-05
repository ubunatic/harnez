# Latest release links & README/website install section consolidation

**Status:** Open — proposed assessment & exploration
**Severity:** N/A (DX / Documentation Architecture Proposal)
**Category:** Documentation / Release Integration

---

## 1. Context & Motivation

Following the implementation of the spec-driven `harnez release` command (Issue 091), repositories now cut signed, multi-arch releases to Codeberg (`codeberg.org/ubunatic/<project>/releases/latest`).

However, the user-facing documentation surfaces (project `README.md` files and `website/index.html`) currently lack a consolidated convention for:
1. **Latest Release Links**: Consistently pointing to `https://codeberg.org/ubunatic/<project>/releases/latest` (web view) or the POSIX `curl | sh` static installer pattern.
2. **Installation Instructions**: Standardizing install guides across sibling projects (`curl | sh`, prebuilt tarballs, `make install`, distro packages, AppImages) while keeping project-specific details intact.

---

## 2. Exploration & Approaches to Assess

We need to assess how much `harnez` CLI should automate vs. how much remains hand-crafted or agent-assisted across the workspace:

### Option A: Managed Marker Section in `README.md`
- Introduce a managed block in project `README.md` (e.g. `<!-- harnez:begin Installation --> ... <!-- harnez:end Installation -->`).
- `harnez init` or `harnez sync` automatically generates and updates the installation commands based on `version.yaml` and project metadata (e.g. static binaries, package formats).
- **Pros**: Zero manual drift when changing standard installer scripts, adding Debian/RPM packages, or introducing AppImages across 20–50 repositories.
- **Cons**: Less flexibility for projects with idiosyncratic prerequisites or bespoke setup steps.

### Option B: Linter / Doctor Probe (`harnez assess` / `harnez status`)
- Keep READMEs and websites hand-crafted or written by pairing agents.
- `harnez` (or `uman`) only acts as an auditor/validator:
  - Checks if `README.md` links to the canonical `https://codeberg.org/ubunatic/<project>/releases/latest` URL.
  - Warns if the installation instructions mention stale tag versions or dead download links.
  - Verifies that `website/index.html` references the active installer script.
- **Pros**: Preserves custom layout and tone per project; zero risk of clobbering hand-crafted README narratives.
- **Cons**: Updating a global convention (e.g. new package repo) requires an agent pass across each repo rather than a single `harnez init` sync.

### Option C: Opt-In Template Doc (`docs/templates/INSTALL.md`)
- Provide a standard `INSTALL.md` or snippet template that projects can include or symlink.
- Projects opt in via `harnez init --docs install` if they want standard prebuilt binary instructions.

---

## 3. Assessment Questions for Follow-up

1. **Website vs. README boundary**: Websites are highly visual and individualized (`website/index.html`), whereas `README.md` is developer-facing. Should `harnez` only manage README install blocks, leaving websites completely to project design?
2. **Marker vs. Linter**: Is a managed marker block in `README.md` desirable, or is a diagnostic check (warning when release URL is missing) sufficient?
3. **Packaging evolution**: When `.deb`, `.rpm`, or AppImage packaging is added to `harnez release`, how should projects declare which package types appear in their install snippet?

---

## 4. References

- `issues/091-language-agnostic-release-spec-and-thin-make-release.md` — Core `harnez release` implementation.
- `emojig/scripts/install.sh` & `emojig/website/index.html` — Canonical Codeberg latest release fetch pattern.
- `spriteview/docs/studies/2026-08-28-releasing-a-pure-python-project-with-goreleaser.md` — Retrospective on release links and installer bugs.

---

## Implementation Plan

### Decision: Option B (linter probe), not Option A (managed README block)

Recommend **Option B only**, and explicitly reject Option A for now. Rationale:

- The README's install section is prose the user writes; `harnez init`'s managed-marker
  machinery (`internal/markdown`, `harnez:begin/end`) is already used for AGENTS.md
  conventions and Makefile targets — extending it into `README.md`, the most hand-crafted,
  most user-visible file in each repo, is exactly the "clobber a hand-written narrative"
  risk the ticket names. No project in the workspace has yet reported drift pain here.
- Option C (`docs/templates/INSTALL.md`) is premature: there is no second consumer yet and
  no evidence the standard snippet is stable. Revisit once `.deb`/`.rpm`/AppImage packaging
  actually lands in `harnez release` (question 3), which is the event that would make a
  generated snippet worth its cost.
- Answer to assessment question 1: **README only, never `website/index.html`.** Websites
  are per-project design surfaces (`docs/Website.md` already governs them); a linter that
  greps HTML for a URL is fine, generating HTML is not.

So: `harnez` warns, never rewrites.

### Steps

1. `internal/release/docscheck.go` (new). One exported function:
   ```go
   func CheckReleaseDocs(dir string, forge *ForgeInfo) []string
   ```
   returning zero or more human-readable warning lines. Checks, all best-effort and all
   silent when the file is absent:
   - `README.md` contains `https://<forge.Host>/<Owner>/<Repo>/releases/latest` — warn if
     the repo has releases configured but the README never links the canonical latest URL.
   - `README.md` / `website/index.html` contain no hardcoded stale tag (`/releases/tag/vX.Y.Z`
     or a literal `vX.Y.Z` in a download/curl line) that is *older* than `version.yaml`'s
     current version — this is the drift that actually bites.
   - `website/index.html`, if present, references either the canonical latest URL or
     `scripts/install.sh`.
   Version comparison: reuse whatever `internal/release` already uses to read `version.yaml`
   (see `runner.go`'s version resolution) rather than adding a semver dependency; a plain
   string inequality against the current version is sufficient to flag "mentions an old tag".

2. `internal/release/docscheck_test.go` — table test over a `t.TempDir()` fixture repo:
   README with canonical link (no warnings), README with a stale `v0.1.0` tag while
   `version.yaml` says `v0.4.0` (one warning), missing README (no warnings, no panic),
   website present without installer reference (one warning).

3. `internal/release/runner.go` — call `CheckReleaseDocs` in the preflight block, next to
   the existing forge step (~line 214-226, before step 8 push), printing each warning as
   `  [docs]      Warning: ...`. **Non-fatal always** — same posture as the existing
   `has_releases` warning. Gate behind `!opt.DryRun == false`? No: run it in dry-run too,
   since `--dry-run` is precisely where a user wants to see doc problems before cutting.

4. `cmd/harnez/release.go` — no new flags. If a standalone probe is wanted later, add
   `harnez release --check-docs` as a no-op-except-checks mode; do **not** add it in this
   pass (YAGNI, and it duplicates what the preflight already prints).

5. Docs: one short subsection in `docs/practices/GoRelease.md` stating the convention
   (canonical latest URL in README; websites hand-crafted; harnez warns only) so the rule
   is discoverable without reading this ticket.

### Tradeoffs / open questions

- **Warning fatigue**: every release of every repo will print these lines until each README
  is fixed. Acceptable (there are only a handful of release-cutting repos), but if it gets
  noisy, the escape hatch is a `release.yaml`/spec key like `skip_docs_check: true` rather
  than removing the check.
- **Stale-tag detection is heuristic** — a README may legitimately mention an old version
  (changelog excerpt, migration note). Keep the check narrow: only flag a version string
  that appears inside a line also containing `curl`, `wget`, `download`, or `releases/`.
- Assessment question 3 (declaring package types) is **deferred** — it only becomes real
  once `harnez release` produces `.deb`/`.rpm`/AppImage artifacts. Note in the ticket that
  revisiting Option A/C is gated on that, and close this ticket on the linter alone.

### Scope

**Small** — one new file (~120 lines), one test file, ~5 lines of wiring in `runner.go`,
one docs subsection. No new CLI surface, no `init`/`apply` changes, no marker machinery.

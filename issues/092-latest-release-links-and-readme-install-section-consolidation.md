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

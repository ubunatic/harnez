# 658 — issues new slug mangles German umlauts

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**: issue 202 (placeholder slug divergence), issue 238 (README.md.lock gitignore)

---

## 1. Problem & Motivation

`harnez issues new` turns non-ASCII letters in the title into `-`, so German titles produce broken
file names. Seen in a German-language tracker repo (`~/work/fv`):

- `"Vorfinanzierung Liegestühle …"` → `001-vorfinanzierung-liegest-hle-….md`
- `"Neue Mitgliedsanträge …"` → `004-neue-mitgliedsantr-ge-….md`

The agent then deletes the placeholder and writes a hand-named file, which is exactly the divergence
issue 202 tried to prevent.

Side note from the same repo: `issues/README.md.lock` was left behind and committed once, so the
issue 238 gitignore fix did not reach that repo at its `harnez init` time.

## 2. Technical Specification / Findings

Expected slugs: transliterate German (`ä→ae`, `ö→oe`, `ü→ue`, `ß→ss`, uppercase too) and strip
other diacritics via Unicode NFD before dropping remaining non-ASCII runes. No new dependency needed
for the German table; NFD would need `golang.org/x/text` or a small hand table.

## 3. Implementation & Verification Plan

1. Add transliteration to the slug function used by `issues new`.
2. Table test: `Liegestühle` → `liegestuehle`, `Anträge` → `antraege`, `Größe` → `groesse`,
   `café` → `cafe`.
3. Check whether `harnez init` in existing repos adds the `issues/*.lock` ignore (issue 238).

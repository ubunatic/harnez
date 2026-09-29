# 639 — Recommend `loom eval` for .ansi Asset Validation in Agent Guidelines

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Normal
**Category**: Documentation / Agent UX

---

## 1. Problem & Motivation

When agents in loom-adjacent projects (or harnez-managed sprints on the loom repo) produce or
modify `.ansi` design asset files, they have no guidance to validate them. Issues encountered:

- Static `.ansi` files may accidentally embed VS16 variation selectors (`\uFE0F`) from the
  terminal the author used, causing terminal-specific line-width drift (e.g. foot vs tilix).
- Ragged row widths in ANSI art go unnoticed until the file is `cat`-ed in a different terminal.
- Box border misalignments are only visible when rendered, not when grepping the raw bytes.

`loom eval` and `loom check-box` (available since loom issue #166) detect all of these conditions.

## 2. Proposed Change

Add a note to the relevant harnez agent guideline (e.g. a `docs/` evergreen or the managed
`.harnez/rules/` stubs) recommending:

```
After creating or modifying .ansi files, validate them with:

  loom eval <file.ansi>        # check row widths and box geometry
  loom check-box <file.ansi>   # check box border alignment
  loom eval -a <file.ansi>     # annotated view for debugging ragged rows

If loom is not installed: go install ubunatic.com/loom/cmd/loom@latest
```

Also note the VS16 portability invariant: `.ansi` assets must not contain VS16 (`\uFE0F`) on
symbols intended as 1-column icons. Strip with:

```bash
python3 -c "
data = open('file.ansi','rb').read()
fixed = data.replace('\u2139\ufe0f'.encode('utf-8'), '\u2139'.encode('utf-8'))
open('file.ansi','wb').write(fixed)
"
```

## 3. Context

- Discovered during loom session 2026-09-29: `docs/data/ansiedit-design-005.ansi` had `ℹ️` with
  VS16 that caused `loom eval` to flag 23 ragged rows. Fixed by stripping VS16 (loom commit
  `381d9b8`).
- `loom eval -a` annotated output mode added in loom issue #166 (sprint 2026-09-29).
- VS16 portability documented in `loom/docs/EmojiWidth.md` §"VS16 Portability in Static .ansi
  Asset Files".
- Related loom issues: #048 (emoji rune width discrepancy), #166 (annotated output mode).

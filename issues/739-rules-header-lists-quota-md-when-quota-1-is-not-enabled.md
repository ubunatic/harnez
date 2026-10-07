# 739 — Rules header lists Quota.md when quota-1 is not enabled

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Init
**Related**: #625, #701

---

## 1. Problem & Motivation

In consumer repo `uman` (initialized without `--quota-1`), `harnez init` writes this
first line into `AGENTS.md:1` and `CLAUDE.md:1`:

```
**Before any work, read all Harnez rules in one call: `harnez read .harnez/rules/Tools.md
.harnez/rules/Issues.md .harnez/rules/Quota.md .harnez/rules/Subagents.md
.harnez/rules/Output.md .harnez/rules/Local.md`. ...**
```

`.harnez/rules/Quota.md` is not generated (init wrote only Index, Tools, Issues,
Subagents, Output). Running the command prints:

```
harnez read: .harnez/rules/Quota.md: open .harnez/rules/Quota.md: no such file or directory
```

and exits 0. Agents see an error on every session start and may treat it as a failure.
(`Local.md` is also absent but is optional and user-owned; it may deserve silent skip.)

## 2. Proposed Fix

Generate the header from the rule files init actually writes: omit `Quota.md` unless
quota-1 is enabled. Optionally let `harnez read` skip a missing `Local.md` silently.

## 3. Verification

- `harnez init` in a repo without quota-1: header has no `Quota.md`; the header
  command prints no errors.
- With `--quota-1`: header includes `Quota.md` and the file exists.

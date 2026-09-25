# 586 — Managed docs: after harnez issues new, open and fill the created file

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Docs
**Related**: Issue 585

---

## 1. Problem & Motivation

The "Issue Tracker Discovery" part of the Harnez Managed Conventions block says
`harnez issues new` reserves a number and creates a placeholder ticket, then
"write the ticket to the printed path". It does not say plainly that the file
`issues new` creates is the template to fill in.

So an agent thinks about the ticket format first and goes looking for a template
(for example by copying an archived ticket's header), instead of simply
running the command and editing what it produced.

## 2. Proposed Fix

Reword the managed block so the flow is explicit and comes first:

1. Run `harnez issues new -d <repo> "<title>"` right away; do not look for a
   template beforehand.
2. Open the printed file and fill it in, keeping its schema.
3. Run `harnez index -d <repo>` and commit.

## 3. Observation While Filing

In this session (harnez 585 and 586, created from voxi), the file
`issues new` wrote contained only the title, `**Status**: Draft` and
"Reserved placeholder ticket." — no Priority/Severity/Category fields or
sections. If the full template is meant to be written, check why it was not
here, as a separate bug.

## 4. Acceptance

A fresh session after `harnez init` files an issue by running `issues new`
and editing the created file, without searching for a template.

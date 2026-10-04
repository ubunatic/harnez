# 702 — Handle issue discovery when the issues directory is missing

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Bug
**Related**:

---

## 1. Problem & Motivation

In Tilix, which had Harnez guidance but no `issues/` directory yet, `harnez find -d . issues "Pixman"` failed with `find: scan issue files: stat .: no such file or directory`. Creating the first ticket with `harnez issues new -d /home/uwe/projects/tilix "..."` succeeded and created `issues/` plus its README. The first-use workflow works through `issues new`, but discovery fails unclearly before the tracker exists.

**Goal**: `/goal` Make the no-tracker case clear and predictable: issue discovery reports that no tracker exists without a path error, and creating the first ticket still initializes `issues/` and its index; verify both cases or stop and report if they cannot be reproduced.

## 2. Technical Specification / Findings

The failure was reproduced on 2026-10-04 in `/home/uwe/projects/tilix`. `harnez issues new` then allocated ticket 001 and created the tracker. No existing Harnez issue matched this first-use behavior.

## 3. Implementation & Verification Plan

Trace issue discovery and first-ticket initialization. Define a friendly missing-tracker result for `harnez find`, retain automatic directory/index creation in `harnez issues new`, document the bootstrap behavior, and add regression coverage for both cases.

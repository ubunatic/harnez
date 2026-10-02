# 684 — agents list discovers externally hosted sessions

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: [Jev compaction](../docs/JevCompaction.md)

---

## 1. Problem & Motivation
`harnez agent list` currently lists Harnez-managed sessions only. Agents started
directly with a provider CLI, such as an open Claude Code session, are invisible,
so Harnez cannot help users find or inspect them.

This is the first step in shifting `harnez agent` from primarily wrapping agent
launches to orchestrating agent sessions. Later commands should be able to attach
to discovered sessions and inspect their context.

## 2. Technical Specification / Findings
The session registry is not a complete source: externally hosted sessions may
have no Harnez record. Explore provider-supported local session metadata and
report discovered sessions with enough identity and state to select them safely.
Record provider-specific discovery limits and any ambiguity; do not assume every
provider exposes live context through the same interface.

## 3. Implementation & Verification Plan
**Goal**: Make `harnez agents list` discover and identify non-Harnez-hosted
sessions alongside managed sessions, establishing a reliable selection path for
future inspection commands. Stop and report if a provider offers no safe,
supported discovery mechanism or requires a user decision.

Done when listing can surface an external session with provider, stable session
identity, and available status/project metadata; managed-session listing remains
usable; and provider coverage and limitations are documented. Verify discovery
against a live external session and automated tests for parsing and deduplication.

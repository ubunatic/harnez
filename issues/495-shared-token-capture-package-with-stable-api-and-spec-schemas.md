# 495 — Shared token capture package with stable API and spec schemas

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Architecture
**Related**: 446, 445, 490, `docs/HarnezComponents.md` §8.6, §8.9

## /goal

One shared Go package for capturing and parsing token and cost data, with a stable API and
record schemas in `spec/`, so every component can capture what it needs and store it its
own way, auto opting in to an active peer.

## 1. Problem & Motivation

Token data is captured in several places (Codex rollouts, Claude transcripts, agent
provider responses, hooks), and 446 plans to persist agent-reported cost into the
telemetry database, which would couple agents to telemetry and sqlite. Decision (§8.9):
capture is shared, storage is per component.

## 2. Technical Specification

- Package (e.g. `internal/tokens`, later public if components split): capture/parse
  functions per provider, a `Record` type, no storage.
- Schemas in `spec/` (single source of truth per `docs/Spec.md`); the Go types must not
  duplicate spec values.
- Stable API: versioned record schema; additive changes only within a version.
- Storage stays with the component: agents in session records, telemetry in
  `tool_catalog.sqlite`, others as needed.
- Peer detection: a component checks whether another is active (installed and enabled by
  the selection) and hands records to it (auto opt-in); otherwise keeps its own state
  (auto opt-out / fallback).
- 446 becomes a consumer: agent responses → shared `Record` → session record, plus
  telemetry when active.

## 3. Implementation & Verification Plan

- [ ] Inventory existing capture code (usage, codex events, subagent counters)
- [ ] Record schema in `spec/`, package with parsers and tests
- [ ] Peer detection helper (depends on 491's resolved selection)
- [ ] Migrate agents and telemetry to the package; update 446

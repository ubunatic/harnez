# 1260 — Tests leak model blocks into the shared test state dir

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Bug
**Related**: 1252

---

## 1. Problem & Motivation

`cmd/harnez` `TestMain` (`main_env_test.go:159`) fails the run with "tests left model blocks in
the shared test state dir" after some broad `-run` selections, and the leaked
`<state>/policy/model-blocks.json` then breaks later runs until someone deletes it by hand.

## 2. Technical Specification / Findings

- Seen twice on 2026-11-20/21 in taia sessions. A fresh run after deleting the file passes, so
  some test writes a model block to the shared path without its own state dir.

## 3. Implementation & Verification Plan

- Find the writer: run `cmd/harnez` packages by name with the guard on and bisect.
- Give that test `t.Setenv` of the state dir (or `modelblock` path override) so it never touches
  the shared file; keep the `TestMain` guard as the regression check.

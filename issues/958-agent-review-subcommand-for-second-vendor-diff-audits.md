# 958 — agent review subcommand for second-vendor diff audits

**Status**: Open
**Priority**: P3 (Low)
**Severity**: Minor
**Category**: Feature
**Related**: ubunatic.com 120 (30-minute live-build talk)

---

## 1. Problem & Motivation
Asked for a live-stage script, agy:argon:low invented `harnez agent review --vendor second`
and `harnez agent resume --apply-review`. Neither exists. That models reach for them
suggests a natural gap: the second-vendor review gate is a core harnez pattern but
needs a hand-built pipe (`git diff | harnez agent --model <other vendor> -p ...`).

## 2. Technical Specification / Findings
Possible shape: `harnez agent review [--model M] [REF]` pipes `git diff REF` to a
model from a different vendor than the session that wrote the change, read-only,
and prints `APPROVE` or line-numbered `REVISE`. Exit code reflects the verdict.

## 3. Implementation & Verification Plan
Decide first whether this belongs in harnez or stays a Makefile recipe. If built,
test vendor selection and verdict exit codes.

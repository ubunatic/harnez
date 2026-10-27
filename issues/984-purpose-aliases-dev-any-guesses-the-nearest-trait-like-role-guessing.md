# 984 — purpose aliases: dev:<any> guesses the nearest trait, like --role guessing

**Status**: Open
**Priority**: P3 (Low) — after 983
**Severity**: Minor
**Category**: Feature
**Related**: 983 (purpose aliases, must land first), 692 (role aliases and one-line docs for --role guesses), 686 (unknown roles resolved through the decider, prompt inference), 959 (role inference bug), 942 (epic)

---

/goal `--model <role>:<any-trait>` with a trait that is not in the spec resolves to the nearest defined purpose alias
(e.g. `dev:clever` → `dev:smart`), the way `--role` resolves unknown role names, and says which alias it picked; or
stop and report when blocked on an owner decision or a denied permission. Circuit breaker: on a loop or no progress,
clear the goal and write a failure report here.

## 1. Problem & Motivation

Owner, 2026-10-27, during 983: "later we can allow dev:<any> model and guess the role like we do with the
--role <role>". 983 rejects an unknown trait (`dev:clever`) as an error. Agents and users will type traits that are
close to a defined one; guessing like `--role` does avoids a failed start.

## 2. Findings

- `--role` guessing: `resolveRoleIn` (`internal/subagent/agentspec.go:580`) uses role aliases and one-line
  descriptions (692); unknown roles go through the decider with prompt inference (686).
- 983 adds `purpose_traits` (one-line meaning per trait), which can play the role descriptions play for `--role`.

## 3. Open questions

- Is the guess printed and recorded (session `model_alias` from 983 M5 plus the guessed alias)?
- Low-confidence guess: fail with suggestions, or fall back to a default per role (e.g. `dev:std`)?
- Owner may also mean the reverse: `--model dev:...` implies `--role developer`. Confirm before building.

## 4. Verification

Tests for exact, alias-close and decider-guessed traits and a no-match case; a live `harnez agent start --model
dev:clever` prints the alias it picked; `make test-q1`; `make install`.

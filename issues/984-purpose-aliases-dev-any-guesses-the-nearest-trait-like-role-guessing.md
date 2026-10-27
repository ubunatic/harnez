# 984 — purpose aliases: dev:<any> guesses the nearest trait, like --role guessing

**Status**: Open
**Priority**: P3 (Low) — after 983
**Severity**: Minor
**Category**: Feature
**Related**: 983 (purpose aliases, must land first), 692 (role aliases and one-line docs for --role guesses), 686 (unknown roles resolved through the decider, prompt inference), 959 (role inference bug), 942 (epic)

---

**Update (2026-10-27, after 983 design B):** 983 dropped `<role>:<trait>` model names. Picks live per role
(`roles.<role>.models`), the role resolves first, then `--model <trait>` inside it. This ticket now means: an
unknown trait (`--role dev --model clever`) guesses the nearest trait of the resolved role (`smart`), using the
role's `model_traits` descriptions. The reverse question below (`dev:...` implying a role) is settled: the role
always comes from `--role` or prompt inference.

/goal `--model <any-trait>` with a trait the resolved role does not define resolves to that role's nearest trait
(e.g. `--role dev --model clever` → `smart`), the way `--role` resolves unknown role names, and says which trait it picked; or
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
- ~~Owner may also mean the reverse: `--model dev:...` implies `--role developer`.~~ Settled by 983 design B.

## 4. Verification

Tests for exact, alias-close and decider-guessed traits and a no-match case; a live `harnez agent start --role dev
--model clever` prints the trait it picked; `make test-q1`; `make install`.

# 383 — Disable queued question tool prompts in Codex sessions

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Agentic Ergonomics
**Related**: [Codex configuration](../config.yaml)

---

## 1. Problem & Motivation

During a Codex session, the agent called the interactive question tool for a
routine clarification. The client displayed a queued prompt:

```text
Queued ...
? 1 question
  alt + ↑ to answer
```

The user wants this question-tool flow disabled for Codex. A queued prompt is
easy to miss and interrupts the normal chat flow. In the reported session, the
agent could have continued with the user's subsequent clarification in chat.

## 2. Desired Behavior

Codex agents should not invoke the queued question tool. They should make a
reasonable assumption for optional details and ask any truly necessary
clarification in the ordinary conversation. Determine whether the installed
Codex version supports disabling this tool in configuration or permissions;
otherwise provide the narrowest effective Codex-specific instruction. Keep
other agents' interaction tools unchanged.

## 3. Acceptance Criteria & Verification

- [ ] Identify the available Codex control for the queued question tool, or
      document that no supported control exists in the installed version.
- [ ] Apply the Codex-specific setting or instruction through harnez's managed
      configuration so it survives `harnez apply`.
- [ ] In a live Codex canary, an optional clarification does not create a
      `Queued ... ? 1 question` prompt; the agent proceeds or asks in chat.
- [ ] Verify that necessary user approval or clarification remains possible
      through ordinary conversation.

# 455 — Add shell autocomplete for agent names with prompt descriptions

**Status**: Closed — implemented shell completion with privacy-safe prompt descriptions
**Priority**: P2 (Medium)
**Severity**: Moderate
**Category**: Feature
**Related**: —

## 1. Problem & Motivation

Agent lifecycle commands that accept a session reference require users to
remember or inspect agent names manually. Shell completion should make named
agents discoverable while providing enough prompt context to distinguish them.

## 2. Technical Specification / Findings

Add shell autocomplete for agent names on commands that reference a specific
agent, including `resume`, `stop`, `delete`, and other applicable lifecycle
commands. Completion entries should use the registered agent name and include
the first 40–100 characters of its start prompt as the completion description,
with safe handling for missing, short, or multiline prompts.

## 3. Implementation & Verification Plan

/goal: Users can tab-complete valid agent names for every agent command that
accepts a specific agent reference, and see a concise 40–100-character start
prompt description that helps select the intended session.

- Identify the supported shell completion mechanism and all agent-reference
  command paths.
- Add completion and prompt metadata storage/exposure as needed, preserving
  lineage and privacy behavior.
- Test names, descriptions, truncation, missing prompts, and each applicable
  command.

**Status**: Draft

---

Reserved placeholder ticket.

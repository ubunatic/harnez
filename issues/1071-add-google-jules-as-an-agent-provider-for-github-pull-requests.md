# 1071 — Add Google Jules as an agent provider for GitHub pull requests

**Status**: Open
**Priority**: P2 (Medium)
**Severity**: Minor
**Category**: Feature
**Related**: [Jules API](https://developers.google.com/jules/api)

---

## 1. Problem & Motivation

The owner has Jules connected to GitHub repositories and wants Harnez to send work from `./issues` to Jules, await a GitHub pull request, and then pull and integrate the result before pushing to the canonical Codeberg upstream.

## 2. Goal & Acceptance Criteria

/goal Add Jules as an agent provider so a user can submit a local issue to the connected GitHub repository and await a usable PR, verify the workflow, or stop and report when blocked on a user decision or denied permission. On a loop, hard architectural or OS blocker, or lack of progress, clear the goal and write a failure report with the reason and resume point.

- Select an issue and connected GitHub source and starting branch; submit the issue context to Jules and surface the session and eventual PR URL for the owner to pull and integrate.
- Handle ongoing, failed, and action-required sessions clearly, including bounded waiting and a way to resume tracking. Keep credentials secret and document setup and authentication.
- Preserve the owner's control over review, integration, and pushing to Codeberg; do not assume the GitHub PR is already merged upstream.
- Verify the provider lifecycle and error handling with meaningful tests; validate the real API workflow when credentials and access are available, or document the blocker.

## 3. Notes

The API is alpha. Its documentation describes connected GitHub sources, API-key authentication, session tracking, and `AUTO_CREATE_PR`; verify current behavior before implementation. Check live code and recent commits before starting, and resolve any provider-interface or GitHub-to-Codeberg workflow uncertainties against the current project.

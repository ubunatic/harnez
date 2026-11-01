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

**Shutdown (checked 2026-11-01):** [Google's changelog](https://jules.google/docs/changelog/2026-10-29) says Jules winds down on 2026-12-31. Sessions can be finished until 2026-11-11, then Jules is read-only until the end of the year. A real workflow can only be validated until 2026-11-11. The owner chose to keep this ticket open anyway.

**API findings (2026-11-01, one research pass, not exercised):**

- Base URL `https://jules.googleapis.com/v1alpha`, header `X-Goog-Api-Key`, key created at `https://jules.google.com/settings#api`.
- `POST /v1alpha/sessions` takes `prompt`, `sourceContext` (`source`, `githubRepoContext.startingBranch`), `title`, `automationMode` (`AUTO_CREATE_PR`) and `requirePlanApproval`. Sessions also have `:sendMessage` and `:approvePlan`.
- Session `state`: `QUEUED`, `PLANNING`, `IN_PROGRESS`, `AWAITING_PLAN_APPROVAL`, `AWAITING_USER_FEEDBACK`, `PAUSED`, `FAILED`, `COMPLETED`. The docs do not describe `PAUSED`.
- The PR link is at `outputs[].pullRequest.url` on the session. No webhooks or streaming were found, so tracking means polling.
- No model or effort field was found, which does not fit the `provider:model:effort` spec form directly.
- Plan limits per rolling 24 hours: 15 tasks and 3 concurrent, Pro 100 and 15, Ultra 300 and 60. Whether API sessions count against them is unconfirmed.

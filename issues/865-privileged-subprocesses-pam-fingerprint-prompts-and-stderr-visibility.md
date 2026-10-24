# 865 — Privileged subprocesses, PAM biometric prompts, and stderr visibility

**Status**: Open
**Priority**: P2 (Medium) — silent auth timeouts in wrappers break biometric sudo flows and conceal failure causes
**Severity**: Moderate
**Category**: Convention
**Related**: docs/Canary.md, docs/lang/Go.md, docs/lang/Bash.md

---

/goal Standardize conventions and provide a canary for privileged subprocess execution (`sudo`, `pkexec`,
package managers) across Go, Bash, and agents: ensure PAM conversation cues (such as fingerprint reader or
hardware token touch prompts) are visible to the user without requiring interactive `stdin`, and ensure
`stderr` is never swallowed on failure so that errors carry the exact failure reason instead of opaque
`exit status 1`. Verified by a standalone canary demonstrating the failure mode and confirming fingerprint
authentication without stdin on host hardware.

## 1. Problem & Motivation

Many modern Linux developer environments configure PAM biometric authentication (such as `pam_fprintd`,
or hardware security keys via FIDO2 / `pam_u2f`) to authorize privileged actions (`sudo`) without requiring
passwords to be typed into `stdin`.

When authenticating via fingerprint:
- Verification events arrive asynchronously over D-Bus (`net.reactivated.Fprint`); **no keystrokes or `stdin` are required**.
- However, PAM conversation functions print user prompts (e.g. `Place your finger on the fingerprint reader`,
  `Touch device`, or fallback notices) directly to **`stderr`**.

When CLI tools, installer routines, or agent commands wrap `sudo` (e.g. Go's `exec.CommandContext`, Python's
`subprocess`, or Bash scripts redirecting output):
1. **The Stderr Blindspot**: If `stderr` is redirected to `/dev/null`, discarded, or silently buffered until
   process exit, the user receives **no visual cue** that the sensor is waiting for a finger.
2. **Inevitable Timeout**: The user does not know to touch the reader; after the PAM timeout (typically 10–30s),
   `sudo` aborts with exit code 1.
3. **Diagnostic Loss**: Because `stderr` was discarded or not attached to the error message on failure, the
   wrapper surfaces only a generic `exit status 1` (e.g., `sudo install modifier binary: exit status 1`).
   The developer cannot tell whether the failure was a PAM timeout, missing sudoers privilege, invalid flag,
   or missing binary.

While interactive password prompts require both `stdin` and a TTY, biometric authentication requires **only
visible output (`stderr`)**. Suppressing `stderr` needlessly breaks biometric authorization even in headless
or non-interactive pipelines.

## 2. Technical Findings

- Probing `sudo` under biometric PAM on host:
  `python3 -c "import subprocess; res = subprocess.run(['sudo', 'true'], stdin=subprocess.DEVNULL, capture_output=True, timeout=2)"`
  Stderr captures: `Place your finger on the fingerprint reader`.
- If `stderr` is streamed to the user's terminal while `stdin` is closed (`/dev/null`), touching the fingerprint
  reader succeeds immediately and the command exits 0.
- When `stderr` is discarded, the prompt is swallowed, blocking until PAM times out and exiting 1.
- In Go `exec.Cmd`:
  - Default `cmd.Stderr = nil` directs to `/dev/null` if not connected to the parent process.
  - `cmd.Output()` captures only `stdout`, completely dropping `cmd.Stderr` unless explicitly routed.
  - `cmd.CombinedOutput()` buffers everything together, but if the command is waiting for an action prompted
    on stderr, the buffer is not displayed until the process exits.
  - To support biometric auth, the wrapper must either stream `stderr` to `os.Stderr` (or an interactive
    logger/prefixer), or use a tee buffer that streams live output while capturing for error reporting.

## 3. Subtask: Standalone Canary (Proving the Problem & Host Confirmation)

Create a standalone canary probe (e.g., `scripts/canary/pam_prompt.sh` or Go canary probe) that tests:

- **Phase A (Demonstrate the Failure Mode)**:
  Runs `sudo true` with `stdin=DEVNULL` and `stderr=DEVNULL` (swallowed).
  Demonstrates that execution stalls without feedback, user has no indication to touch the sensor, and
  times out into a generic exit status 1.
- **Phase B (Demonstrate Biometric Success without Stdin)**:
  Runs `sudo true` with `stdin=DEVNULL`, streaming `stderr` directly to terminal (`os.Stderr` or prefixed).
  The user sees `Place your finger on the fingerprint reader` (or configured prompt), touches the sensor,
  and the command exits 0 immediately without any `stdin` interaction.
- **Phase C (Diagnostic Capture on Failure)**:
  Runs an invalid command or lets PAM time out while teeing `stderr`; asserts that when exit status is non-zero,
  the captured stderr output (`pam_fprintd: verification timed out`, `a password is required`, etc.) is
  included in the returned error message rather than a bare `exit status 1`.

The user can run this canary directly on their host to confirm the biometric behavior and verify the fix.

## 4. Proposed Conventions & Rules

1. **`docs/lang/Go.md` (Subprocesses & Privileged Commands)**:
   - When invoking privileged helpers (`sudo`, `pkexec`, `doas`, package managers):
     - **Always keep `stderr` visible or live-streamed**: Do not discard `stderr` or silently buffer it without
       displaying prompts.
     - **Always capture `stderr` for error returns**: If a subprocess exits non-zero, format the returned error
       with the captured `stderr` content (e.g., `fmt.Errorf("%w: %s", err, strings.TrimSpace(stderrBuf.String()))`).
     - **Do not assume headless implies password stdin**: Biometric prompts (fingerprint, FIDO2/U2F) only need
       display visibility (`stderr`), not keyboard input.

2. **`docs/lang/Bash.md` (Privileged Recipes & Scripts)**:
   - Never silence privileged commands with `sudo ... 2>/dev/null` or `sudo ... &>/dev/null` in scripts or
     Makefiles where user elevation or PAM auth can take place.
   - If output redirection is needed for stdout, leave stderr attached to the console (`sudo ... >/dev/null`).

3. **Agent Practice / Tooling**:
   - Agent command runners and background jobs should ensure PAM/sudo cues are not hidden in a background pipe
     without notifying the orchestrator or user.

## Status check (2026-10-24)

The canary exists (`scripts/canary-865-pam-fingerprint.py`, 9879553b, 112527d2); no host run result is
recorded here. Still open: run it on the host (does fingerprint auth work with stdin closed?), then write the
rules into `docs/lang/Go.md` and `docs/lang/Bash.md` (sources first, then root copies).

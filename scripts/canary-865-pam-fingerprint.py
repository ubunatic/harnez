#!/usr/bin/env python3
"""Canary for Issue 865: Privileged subprocess PAM prompts and stderr visibility.

Proves:
1. Phase 1 (Swallowed Stderr): Discarding/capturing stderr conceals the PAM prompt
   ("Place your finger on the fingerprint reader"), leaving the user blind.
2. Phase 2 (Confirmation without Stdin): Running sudo with stdin=DEVNULL and
   streaming stderr allows seamless fingerprint authentication with ZERO keystrokes.
"""

import argparse
import subprocess
import sys
import time


def test_swallowed_stderr():
    print("--- Phase 1: Probing swallowed stderr behavior ---")
    subprocess.run(["sudo", "-k"], check=True)
    start = time.time()
    try:
        proc = subprocess.run(
            ["sudo", "true"],
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=True,
            timeout=2.5,
        )
        elapsed = time.time() - start
        print(f"Command unexpectedly finished in {elapsed:.2f}s with returncode {proc.returncode}")
    except subprocess.TimeoutExpired as exc:
        elapsed = time.time() - start
        stderr = (exc.stderr or b"").decode("utf-8", errors="replace") if isinstance(exc.stderr, bytes) else (exc.stderr or "")
        print(f"Demonstrated: sudo stalled for {elapsed:.2f}s with NO visible terminal prompt.")
        print(f"Captured hidden stderr was: {stderr.strip()!r}")
        if "fingerprint" in stderr.lower() or "finger" in stderr.lower():
            print("Confirmed: The PAM biometric prompt was emitted on stderr, but swallowed by the wrapper.")
        return True
    return False


def test_interactive_fingerprint(timeout=15):
    print("\n--- Phase 2: Live Biometric Confirmation (No Stdin) ---")
    subprocess.run(["sudo", "-k"], check=True)
    print("Executing `sudo true` with stdin=DEVNULL (no keyboard input can be received).")
    print(f"Streaming stderr live... You have {timeout}s to touch the fingerprint reader:\n")
    sys.stdout.flush()

    start = time.time()
    try:
        # Run sudo true with stdin closed, stderr directed straight to terminal
        proc = subprocess.run(
            ["sudo", "true"],
            stdin=subprocess.DEVNULL,
            stderr=sys.stderr,
            timeout=timeout,
        )
        elapsed = time.time() - start
        if proc.returncode == 0:
            print(f"\n[PASS] SUCCESS in {elapsed:.2f}s! Authenticated via fingerprint with zero stdin interaction.")
            return True
        else:
            print(f"\n[FAIL] Process exited with code {proc.returncode} in {elapsed:.2f}s.")
            return False
    except subprocess.TimeoutExpired:
        elapsed = time.time() - start
        print(f"\n[TIMEOUT] Timed out after {elapsed:.2f}s waiting for biometric input.")
        return False


def main():
    parser = argparse.ArgumentParser(description="Canary 865: PAM fingerprint & stderr visibility")
    parser.add_argument("--probe-only", action="store_true", help="Run only Phase 1 (automated probe)")
    parser.add_argument("--confirm-only", action="store_true", help="Run only Phase 2 (live sensor confirmation)")
    parser.add_argument("--timeout", type=int, default=15, help="Timeout in seconds for confirmation")
    args = parser.parse_args()

    if args.confirm_only:
        success = test_interactive_fingerprint(args.timeout)
        sys.exit(0 if success else 1)

    if args.probe_only:
        test_swallowed_stderr()
        sys.exit(0)

    # Full run
    test_swallowed_stderr()
    print("\nReady for live confirmation test.")
    success = test_interactive_fingerprint(args.timeout)
    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()

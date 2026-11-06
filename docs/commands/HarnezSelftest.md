---
name: harnez-selftest
description: Test and verify native agent background task execution flow and self-test command progression
disable-model-invocation: true
---

# Harnez Agent Background Self-Test

When invoked, execute the self-test flow to verify that your native background task/job execution and progression work correctly:

1. **Briefly summarise the task** to the user.
2. **Execute the initial step**:
   Run `harnez agent selftest --step hello`.
3. **Follow the instructions returned by the CLI**:
   Copy each printed command exactly, including its `--session` and any `--state-dir` values. Run every command in its own tool shell as instructed, keeping the same session and state directory throughout. Run the printed background command as a native background Task/Job, then execute the printed confirmation and completion commands in sequence without skipping or guessing steps.

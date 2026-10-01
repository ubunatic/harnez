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
   Follow each instruction printed by the command output at runtime, executing the requested native background actions and progression commands in sequence without skipping or guessing steps.

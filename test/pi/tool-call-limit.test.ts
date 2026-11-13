import { describe, it } from "node:test";
import assert from "node:assert/strict";
import registerToolCallLimit from "../../internal/subagent/tool-call-limit.ts";

function setupExtension(envOverrides: Record<string, string | undefined>) {
  const prevLimit = process.env.HARNEZ_TOOL_CALL_LIMIT;
  const prevPrompt = process.env.HARNEZ_TOOL_CALL_LIMIT_PROMPT;
  if (envOverrides.limit === undefined) {
    delete process.env.HARNEZ_TOOL_CALL_LIMIT;
  } else {
    process.env.HARNEZ_TOOL_CALL_LIMIT = envOverrides.limit;
  }
  if (envOverrides.prompt === undefined) {
    delete process.env.HARNEZ_TOOL_CALL_LIMIT_PROMPT;
  } else {
    process.env.HARNEZ_TOOL_CALL_LIMIT_PROMPT = envOverrides.prompt;
  }

  const handlers = new Map<string, (event?: unknown) => unknown>();
  const fakePi = {
    on(event: string, handler: (event?: unknown) => unknown) {
      handlers.set(event, handler);
    },
  };

  try {
    registerToolCallLimit(fakePi);
  } finally {
    if (prevLimit === undefined) delete process.env.HARNEZ_TOOL_CALL_LIMIT;
    else process.env.HARNEZ_TOOL_CALL_LIMIT = prevLimit;
    if (prevPrompt === undefined) delete process.env.HARNEZ_TOOL_CALL_LIMIT_PROMPT;
    else process.env.HARNEZ_TOOL_CALL_LIMIT_PROMPT = prevPrompt;
  }
  return handlers;
}

describe("tool-call-limit extension", () => {
  it("rejects missing or invalid env config", () => {
    assert.throws(
      () => setupExtension({ limit: undefined, prompt: "stop" }),
      /HARNEZ_TOOL_CALL_LIMIT/,
    );
    assert.throws(
      () => setupExtension({ limit: "0", prompt: "stop" }),
      /positive integer/,
    );
    assert.throws(
      () => setupExtension({ limit: "2", prompt: "   " }),
      /HARNEZ_TOOL_CALL_LIMIT_PROMPT/,
    );
  });

  it("allows calls under the limit, blocks crossing batch with reason, and terminates on later turn", () => {
    const prompt = "Tool-call limit reached. Answer now.";
    const handlers = setupExtension({ limit: "2", prompt });
    const toolCall = handlers.get("tool_call")!;
    const turnStart = handlers.get("turn_start")!;
    const turnEnd = handlers.get("turn_end")!;

    turnStart();
    assert.equal(toolCall(), undefined);
    assert.equal(toolCall(), undefined);

    // Parallel batch crossing limit: blocks without terminating so the model can synthesize.
    assert.deepEqual(toolCall(), { block: true, reason: prompt, terminate: false });
    assert.deepEqual(toolCall(), { block: true, reason: prompt, terminate: false });

    // Next assistant turn attempting another tool call must set terminate: true.
    turnEnd();
    turnStart();
    assert.deepEqual(toolCall(), { block: true, reason: prompt, terminate: true });

    // Separate case: next turn_start without prior turn_end also terminates on tool_call.
    const handlers2 = setupExtension({ limit: "1", prompt });
    handlers2.get("turn_start")!();
    assert.equal(handlers2.get("tool_call")!(), undefined);
    assert.deepEqual(handlers2.get("tool_call")!(), { block: true, reason: prompt, terminate: false });
    handlers2.get("turn_start")!();
    assert.deepEqual(handlers2.get("tool_call")!(), { block: true, reason: prompt, terminate: true });
  });
});

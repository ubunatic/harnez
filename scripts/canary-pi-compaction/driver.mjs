// Canary for issue 1047: drives real Pi over RPC with a local protocol-compatible stub as its only model.
// Run through run.sh, which isolates it in a loopback-only network namespace. Exits non-zero when an expectation fails.
// The expectations pin Pi 1.4.0 behaviour; a failure after a Pi update means the runtime contract needs a fresh look.
import { spawn } from "node:child_process";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import readline from "node:readline";

const [workDir, piBin, extension, fixture, selection] = process.argv.slice(2);
const REJECTED = "Cannot submit a prompt while compaction is in progress";
const WAIT_MS = 8000;
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const latch = () => {
  let release;
  const promise = new Promise((resolve) => (release = resolve));
  return { promise, release };
};
let failures = 0;
let ran = 0;

function startStub(state) {
  const server = http.createServer((req, res) => {
    let body = "";
    req.on("data", (data) => (body += data));
    req.on("end", () => answer(state, JSON.parse(body), res));
  });
  return new Promise((resolve) => server.listen(0, "127.0.0.1", () => resolve(server)));
}

function lastText(request) {
  const content = request.messages?.at(-1)?.content;
  return typeof content === "string" ? content : JSON.stringify(content ?? "");
}

async function answer(state, request, res) {
  const summary = !request.tools?.length && /summar/i.test(JSON.stringify(request.messages ?? []));
  const text = lastText(request);
  const completion = text.match(/Background command #(\d+) finished|FIXTURE (\S+) done/);
  state.log({ e: "stub", kind: summary ? "summary" : "turn", completion: completion?.[1] ?? completion?.[2] });
  if (summary && state.failSummary) {
    res.writeHead(400, { "content-type": "application/json" });
    res.end(JSON.stringify({ error: { message: "stub summary failure" } }));
    return;
  }
  if (summary && state.holdSummary) await state.holdSummary.promise;
  res.writeHead(200, { "content-type": "text/event-stream" });
  const chunk = (delta, finish, usage) =>
    res.write(`data: ${JSON.stringify({ id: "c", object: "chat.completion.chunk", model: "m", choices: [{ index: 0, delta, finish_reason: finish ?? null }], usage })}\n\n`);
  chunk({ role: "assistant", content: summary ? "stub summary" : "ok" });
  const holdStream = !summary && (text.includes("HOLDSTREAM") || (completion && state.holdCompletion));
  if (holdStream && state.holdStream) await state.holdStream.promise;
  const tool = !summary && text.includes("RUNTOOL") ? state.toolCommand : undefined;
  if (tool) {
    chunk({ tool_calls: [{ index: 0, id: "t1", type: "function", function: { name: "background_task", arguments: JSON.stringify({ command: tool, label: "c" }) } }] });
  }
  chunk({}, tool ? "tool_calls" : "stop", { prompt_tokens: state.usage, completion_tokens: 2, total_tokens: state.usage + 2 });
  res.write("data: [DONE]\n\n");
  res.end();
}

async function scenario(name, options, body) {
  if (selection && !selection.split(",").includes(name)) return;
  ran++;
  const home = path.join(workDir, name);
  const agentDir = path.join(home, ".pi/agent");
  const cwd = path.join(home, "work");
  fs.mkdirSync(agentDir, { recursive: true });
  fs.mkdirSync(cwd);
  const events = [];
  const extensionErrors = [];
  const log = (event) => {
    events.push(event);
    if (event.e === "extension_error") extensionErrors.push(event);
  };
  const state = { log, usage: 50 };
  const server = await startStub(state);
  fs.writeFileSync(path.join(agentDir, "models.json"), JSON.stringify({
    providers: { stub: { baseUrl: `http://127.0.0.1:${server.address().port}/v1`, api: "openai-completions", apiKey: "local", models: [{ id: "m", contextWindow: 1000, maxTokens: 200, input: ["text"] }] } },
  }));
  fs.writeFileSync(path.join(agentDir, "settings.json"), JSON.stringify({
    compaction: { enabled: !!options.auto, reserveTokens: 100, keepRecentTokens: 1 },
    retry: { enabled: false, provider: { maxRetries: 0 } },
  }));
  // Handler order follows load order: "fixtureFirst" makes the fixture's hook hold before the extension's hook runs.
  const loads = options.noFixture ? [extension] : options.fixtureFirst ? [fixture, extension] : [extension, fixture];
  const args = ["--mode", "rpc", "--provider", "stub", "--model", "m", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-themes", "--session-dir", path.join(home, "sessions")];
  for (const load of loads) args.push("-e", load);
  const pi = spawn(piBin, args, {
    cwd,
    stdio: ["pipe", "pipe", "pipe"],
    env: { PATH: process.env.PATH, HOME: home, PI_CODING_AGENT_DIR: agentDir, PI_OFFLINE: "1", PI_SKIP_VERSION_CHECK: "1", PI_TELEMETRY: "0", TMPDIR: home },
  });
  let stderr = "";
  let exited = false;
  pi.stderr.on("data", (data) => (stderr += data));
  // Wait for stdio to drain as well as process exit, so shutdown errors are observed.
  pi.on("close", () => (exited = true));

  let nextId = 0;
  const pending = new Map();
  readline.createInterface({ input: pi.stdout }).on("line", (line) => {
    const message = JSON.parse(line);
    if (message.type === "response") {
      pending.get(message.id)?.(message);
      pending.delete(message.id);
    } else if (message.type === "extension_ui_request") {
      if (String(message.message).startsWith("CANARY ")) log(JSON.parse(message.message.slice(7)));
      else if (message.method === "notify") log({ e: "notify", level: message.notifyType, message: message.message });
    } else if (["compaction_start", "compaction_end", "agent_start", "agent_end", "extension_error", "queue_update"].includes(message.type)) {
      log({ e: message.type, reason: message.reason, aborted: message.aborted, error: message.error, followUp: message.followUp?.length });
    }
  });
  const send = (command) =>
    new Promise((resolve) => {
      const id = `r${++nextId}`;
      pending.set(id, resolve);
      pi.stdin.write(JSON.stringify({ ...command, id }) + "\n");
    });
  const until = async (done, what) => {
    const deadline = Date.now() + WAIT_MS;
    while (Date.now() < deadline) {
      if (done()) return;
      await sleep(5);
    }
    throw new Error(`timed out waiting for ${what}`);
  };
  const count = (e) => events.filter((event) => event.e === e).length;
  const see = (e) => until(() => count(e) > 0, e);
  const prompt = (message) => send({ type: "prompt", message });
  const turn = async (message) => {
    const ends = count("agent_end");
    prompt(message);
    await until(() => count("agent_end") > ends, `turn ${message}`);
  };
  const c = {
    state, events, home, send, prompt, turn, until, see, count, sleep,
    configure: async (config) => {
      await prompt(`/cfg ${JSON.stringify(config)}`);
      await see("cfg");
    },
    seed: async () => {
      if (!options.noFixture) await see("session_start");
      await turn("seed one");
      await turn("seed two");
    },
    // Starts real background tasks that wait for a file each; returns a function that finishes task n.
    startTasks: async (n) => {
      for (let i = 1; i <= n; i++) {
        state.toolCommand = `until test -e ${home}/go${i}; do sleep 0.02; done; echo task-${i}`;
        await turn("RUNTOOL start");
      }
      await sleep(100);
      events.length = 0;
      return (i) => fs.writeFileSync(`${home}/go${i}`, "");
    },
    // Waits until every listed completion reached the model.
    delivered: (ids) => until(() => ids.every((id) => events.some((event) => event.e === "stub" && event.completion === String(id))), `delivery of ${ids}`),
    settle: () => sleep(400),
  };

  const problems = [];
  let result = {};
  try {
    result = (await body(c)) ?? {};
  } catch (error) {
    problems.push(String(error));
  }
  const messages = (await Promise.race([send({ type: "get_messages" }), sleep(3000)]))?.data?.messages ?? [];
  pi.stdin.end();
  pi.kill("SIGTERM");
  for (let i = 0; i < 300 && !exited; i++) await sleep(10);
  if (!exited) {
    pi.kill("SIGKILL");
    problems.push("Pi needed SIGKILL");
  }
  server.closeAllConnections();
  server.close();

  const text = (message) => (typeof message.content === "string" ? message.content : (message.content ?? []).map((part) => part.text ?? "").join(""));
  const transcript = messages.flatMap((message) => {
    if (message.role === "assistant") return [];
    const match = text(message).match(/^Background command #(\d+) finished|^FIXTURE (\S+) done/);
    return match ? [match[1] ?? match[2]] : [];
  });
  const rejected = extensionErrors.filter((event) => event.error?.includes(REJECTED)).length;
  const check = (label, actual, expected) => {
    if (JSON.stringify(actual) !== JSON.stringify(expected)) problems.push(`${label}: got ${JSON.stringify(actual)}, want ${JSON.stringify(expected)}`);
  };
  check("rejections", rejected, options.rejected ?? 0);
  check("extension errors", extensionErrors.length, options.extensionErrors ?? 0);
  check("transcript completions", transcript, options.transcript ?? []);
  for (const pattern of options.notes ?? []) {
    if (!events.some((event) => event.e === "notify" && pattern.test(event.message))) problems.push(`missing note ${pattern}`);
  }
  for (const pattern of options.noNotes ?? []) {
    if (events.some((event) => event.e === "notify" && pattern.test(event.message))) problems.push(`unexpected note ${pattern}`);
  }
  if (options.stderr && !options.stderr.test(stderr)) problems.push(`stderr ${JSON.stringify(stderr.slice(0, 200))} does not match ${options.stderr}`);
  for (const [label, actual, expected] of result.checks ?? []) check(label, actual, expected);

  console.log(`\n=== ${problems.length ? "FAIL" : "PASS"} ${name}`);
  for (const event of events) {
    console.log("  " + JSON.stringify(Object.fromEntries(Object.entries(event).filter(([, value]) => value !== undefined))).slice(0, 200));
  }
  console.log("  FINAL " + JSON.stringify({ rejected, extensionErrors, transcript, stderr: stderr.slice(0, 160) }));
  for (const problem of problems) console.log(`  PROBLEM ${problem}`);
  if (problems.length) failures++;
}

const compact = (c) => c.send({ type: "compact" });
const HELD = /held until compaction ends/;
const SUBMITTED = /once each: .*no acknowledgement/;

// Task finish order 2, 1, 3 proves delivery follows finish order, not task IDs.
const threeTasksDuringHold = (holdOptions = {}) => async (c) => {
  await c.seed();
  const finish = await c.startTasks(3);
  await c.configure({ holdBefore: true, ...holdOptions.config });
  c.state.failSummary = holdOptions.failSummary;
  compact(c);
  await c.see("held");
  for (const id of [2, 1, 3]) {
    finish(id);
    await c.until(() => c.events.filter((event) => event.e === "notify" && HELD.test(event.message)).length >= [2, 1, 3].indexOf(id) + 1 || c.count("extension_error") > [2, 1, 3].indexOf(id), `task ${id} finish`);
  }
  await c.prompt("/release before");
  await c.see("compaction_end");
  await c.delivered([2, 1, 3]);
  await c.settle();
};

await scenario("normal-idle-and-streaming", { transcript: ["1", "2"], noNotes: [HELD, SUBMITTED] }, async (c) => {
  await c.seed();
  const finish = await c.startTasks(2);
  finish(1);
  await c.delivered([1]);
  await c.settle();
  c.state.holdStream = latch();
  c.prompt("HOLDSTREAM");
  await c.until(() => c.events.some((event) => event.e === "stub" && !event.completion && event.kind === "turn"), "streaming");
  finish(2);
  await c.until(() => c.events.some((event) => event.e === "queue_update" && event.followUp === 1), "follow-up queued during the stream");
  const checks = [["model turns before stream release", c.count("agent_end"), 1]];
  c.state.holdStream.release();
  await c.delivered([2]);
  await c.settle();
  return { checks };
});

await scenario("runtime-contract-rejects-during-hold", { rejected: 1, extensionErrors: 1 }, async (c) => {
  await c.seed();
  await c.configure({ holdBefore: true });
  compact(c);
  await c.see("held");
  await c.prompt("/fire direct");
  await c.see("extension_error");
  await c.prompt("/release before");
  await c.see("compaction_end");
  await c.settle();
});

await scenario("multi-success", { transcript: ["2", "1", "3"], notes: [HELD, SUBMITTED] }, threeTasksDuringHold());
await scenario("multi-summary-failure", { transcript: ["2", "1", "3"], notes: [HELD, SUBMITTED] }, threeTasksDuringHold({ failSummary: true }));
await scenario("multi-extension-cancel", { transcript: ["2", "1", "3"], notes: [HELD, SUBMITTED] }, threeTasksDuringHold({ config: { cancelBefore: true } }));
await scenario("earlier-extension-hook-holds", { fixtureFirst: true, transcript: ["2", "1", "3"], notes: [HELD, SUBMITTED] }, threeTasksDuringHold());

await scenario("rpc-abort-during-hold", { transcript: ["1"], notes: [HELD, SUBMITTED] }, async (c) => {
  await c.seed();
  const finish = await c.startTasks(1);
  await c.configure({ holdBefore: true });
  compact(c);
  await c.see("held");
  finish(1);
  await c.until(() => c.events.some((event) => event.e === "notify" && HELD.test(event.message)), "hold");
  await c.send({ type: "abort" });
  await c.see("compaction_end");
  await c.delivered([1]);
  await c.settle();
  return { checks: [["compaction aborted", c.events.find((event) => event.e === "compaction_end").aborted, true]] };
});

await scenario("completion-during-summary", { transcript: ["1"], notes: [HELD, SUBMITTED] }, async (c) => {
  await c.seed();
  const finish = await c.startTasks(1);
  c.state.holdSummary = latch();
  compact(c);
  await c.until(() => c.events.some((event) => event.e === "stub" && event.kind === "summary"), "summary request");
  finish(1);
  await c.until(() => c.events.some((event) => event.e === "notify" && HELD.test(event.message)), "hold");
  c.state.holdSummary.release();
  await c.see("compaction_end");
  await c.delivered([1]);
  await c.settle();
});

await scenario("drain-queues-rest-as-follow-ups", { transcript: ["1", "2", "3"], notes: [SUBMITTED] }, async (c) => {
  await c.seed();
  const finish = await c.startTasks(3);
  await c.configure({ holdBefore: true });
  c.state.holdStream = latch();
  c.state.holdCompletion = true;
  compact(c);
  await c.see("held");
  for (const id of [1, 2, 3]) {
    finish(id);
    await c.until(() => c.events.filter((event) => event.e === "notify" && HELD.test(event.message)).length >= id, `task ${id} finish`);
  }
  await c.prompt("/release before");
  await c.see("compaction_end");
  await c.until(() => c.events.some((event) => event.e === "queue_update" && event.followUp === 2), "both later messages queued in Pi during the first run");
  c.state.holdStream.release();
  await c.delivered([1, 2, 3]);
  await c.settle();
});

await scenario("compact-aborts-run-then-earlier-hook-holds", { fixtureFirst: true, transcript: ["1"], notes: [HELD] }, async (c) => {
  await c.seed();
  const finish = await c.startTasks(1);
  await c.configure({ holdBefore: true });
  c.state.holdStream = latch();
  c.prompt("HOLDSTREAM");
  await c.until(() => c.events.some((event) => event.e === "stub" && event.kind === "turn"), "streaming");
  compact(c);
  await c.see("held");
  finish(1);
  await c.until(() => c.events.some((event) => event.e === "notify" && HELD.test(event.message)), "hold");
  await c.prompt("/release before");
  await c.see("compaction_end");
  c.state.holdStream.release();
  await c.delivered([1]);
  await c.settle();
});

await scenario("slow-later-compact-hook", { transcript: ["1"], notes: [HELD, SUBMITTED] }, async (c) => {
  await c.seed();
  const finish = await c.startTasks(1);
  await c.configure({ holdCompact: true });
  compact(c);
  await c.see("held");
  finish(1);
  await c.until(() => c.events.some((event) => event.e === "notify" && HELD.test(event.message)), "hold");
  await c.settle();
  const checks = [["agent runs while the hook still holds Pi's guard", c.count("agent_start"), 0]];
  await c.prompt("/release compact");
  await c.see("compaction_end");
  await c.delivered([1]);
  await c.settle();
  return { checks };
});

await scenario("automatic-compaction-not-held", { auto: true, transcript: ["1"], noNotes: [HELD] }, async (c) => {
  await c.seed();
  const finish = await c.startTasks(1);
  await c.configure({ holdBefore: true });
  c.state.usage = 950;
  c.prompt("trigger");
  await c.see("held");
  c.state.usage = 50;
  finish(1);
  await c.until(() => c.events.some((event) => event.e === "queue_update" && event.followUp === 1), "Pi queues the follow-up itself");
  await c.prompt("/release before");
  await c.see("compaction_end");
  await c.delivered([1]);
  await c.settle();
});

await scenario("stranded-follow-up-warns", { transcript: [], notes: [/still has a queued message/] }, async (c) => {
  await c.seed();
  const finish = await c.startTasks(1);
  c.state.holdStream = latch();
  c.prompt("HOLDSTREAM");
  await c.until(() => c.events.some((event) => event.e === "stub" && event.kind === "turn"), "streaming");
  finish(1);
  await c.until(() => c.events.some((event) => event.e === "queue_update" && event.followUp === 1), "queued");
  compact(c);
  await c.see("compaction_end");
  c.state.holdStream.release();
  await c.until(() => c.events.some((event) => event.e === "notify" && /still has a queued message/.test(event.message)), "warning");
});

await scenario("shutdown-while-held", { noFixture: true, stderr: /not submitted before shutdown: #1/, notes: [HELD] }, async (c) => {
  await c.seed();
  const finish = await c.startTasks(1);
  c.state.holdSummary = latch();
  compact(c);
  await c.until(() => c.events.some((event) => event.e === "stub" && event.kind === "summary"), "summary");
  finish(1);
  await c.until(() => c.events.some((event) => event.e === "notify" && HELD.test(event.message)), "hold");
});

console.log(`\nscenarios run: ${ran}, failed: ${failures}`);
process.exit(failures || ran === 0 ? 1 : 0);

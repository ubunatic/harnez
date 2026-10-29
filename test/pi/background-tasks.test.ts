// Run: node --import ./test/pi/register.mjs --test test/pi/background-tasks.test.ts
// Fake children never reach a real signal function: the default kill in these tests only records calls.
// Real-process tests pass realKill and use a POSIX shell (/bin/sh).
import { after, test } from "node:test";
import assert from "node:assert/strict";
import { spawn, spawnSync } from "node:child_process";
import { EventEmitter } from "node:events";
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import backgroundTasksExtension, { installBackgroundTasks } from "../../.pi/extensions/background-tasks.ts";
import {
  BackgroundTaskRunner,
  liveProcessGroupMembers,
  parseBackgroundTaskConfig,
  parseProcStat,
  parseTaskId,
  type BackgroundTask,
  type BackgroundTaskConfig,
  type RunnerOptions,
} from "../../.pi/lib/background-task-runner.ts";

const SPEC_PATH = new URL("../../spec/pi-background-tasks.yaml", import.meta.url);
const TIMEOUT_MS = 10_000;
const realKill = (pid: number, signal: NodeJS.Signals | 0) => process.kill(pid, signal);
// Real processes: real signals and the real /proc group reader (Linux), or unknown liveness elsewhere.
const realProcess = { kill: realKill, liveGroupMembers: liveProcessGroupMembers };
const fastConfig: BackgroundTaskConfig = {
  outputLimitChars: 16_000,
  finishedRetentionLimit: 50,
  stop: { graceMs: 20, killWaitMs: 60, pollMs: 5 },
};
const escalationConfig: BackgroundTaskConfig = { ...fastConfig, stop: { graceMs: 50, killWaitMs: 5_000, pollMs: 10 } };
const IS_LINUX = process.platform === "linux";
const HAS_SETSID = spawnSync("sh", ["-c", "command -v setsid"]).status === 0;

const runners: BackgroundTaskRunner[] = [];
after(async () => {
  for (const runner of runners) await runner.shutdown();
});

/** Fake stream: the runner destroys a stream when its output is drained after a held pipe. */
function fakeStream() {
  return Object.assign(new EventEmitter(), { destroy() {} });
}

class FakeChild extends EventEmitter {
  pid: number;
  stdout = fakeStream();
  stderr = fakeStream();
  constructor(pid: number) {
    super();
    this.pid = pid;
  }
}

function fakePi() {
  const handlers = new Map<string, (...args: any[]) => unknown>();
  const tools = new Map<string, any>();
  const commands = new Map<string, any>();
  const messages: Array<{ text: string; options: unknown }> = [];
  const pi = {
    on: (name: string, handler: (...args: any[]) => unknown) => handlers.set(name, handler),
    registerTool: (tool: any) => tools.set(tool.name, tool),
    registerCommand: (name: string, definition: any) => commands.set(name, definition),
    sendUserMessage: (text: string, options: unknown) => messages.push({ text, options }),
  };
  return { pi, handlers, tools, commands, messages };
}

function fakeCtx() {
  const statuses: Array<string | undefined> = [];
  const notes: Array<{ message: string; level: string }> = [];
  return {
    cwd: process.cwd(),
    statuses,
    notes,
    ui: {
      setStatus: (_key: string, text: string | undefined) => statuses.push(text),
      notify: (message: string, level: string) => notes.push({ message, level }),
    },
  };
}

function setup(overrides: Partial<RunnerOptions> = {}) {
  const signals: Array<[number, string | number]> = [];
  const fake = fakePi();
  const { runner } = installBackgroundTasks(fake.pi as any, {
    config: fastConfig,
    shell: "/bin/sh",
    kill: (pid, signal) => {
      signals.push([pid, signal]);
    },
    // Fake groups have unknown liveness unless a test says otherwise, so no fake group is confirmed empty by accident.
    liveGroupMembers: () => undefined,
    ...overrides,
  });
  runners.push(runner);
  const ctx = fakeCtx();
  fake.handlers.get("session_start")?.({}, ctx);
  return {
    ...fake,
    runner,
    ctx,
    signals,
    tool: (name: string) => fake.tools.get(name),
    command: (name: string) => fake.commands.get(name),
  };
}

/** Resolves with the event arguments; rejects after TIMEOUT_MS so a missed event fails the test. */
function once(emitter: EventEmitter, event: string, predicate: (...args: any[]) => boolean = () => true): Promise<any[]> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      emitter.off(event, onEvent);
      reject(new Error(`timed out waiting for "${event}"`));
    }, TIMEOUT_MS);
    const onEvent = (...args: any[]) => {
      if (!predicate(...args)) return;
      clearTimeout(timer);
      emitter.off(event, onEvent);
      resolve(args);
    };
    emitter.on(event, onEvent);
  });
}

/**
 * Live members of process group pgid, read with ps (independent of the runner's /proc reader).
 * Zombies (state Z) have exited and run nothing, so they are not counted.
 */
function liveMembers(pgid: number): number {
  const listing = spawnSync("ps", ["-A", "-o", "pgid=", "-o", "stat="], { encoding: "utf8" }).stdout;
  return listing.split("\n").filter((line) => {
    const [group, state = ""] = line.trim().split(/\s+/);
    return Number(group) === pgid && !state.startsWith("Z");
  }).length;
}

/** A PID is dead when signal 0 finds nothing. A group (negative PID) is dead when it has no live member. */
function isDead(pid: number): boolean {
  if (pid < 0) return liveMembers(-pid) === 0;
  try {
    process.kill(pid, 0);
    return false;
  } catch (error) {
    return (error as NodeJS.ErrnoException).code === "ESRCH";
  }
}

function text(result: { content: Array<{ text: string }> }): string {
  return result.content.map((part) => part.text).join("\n");
}

test("lifecycle: exit 0 completes, exit 7 fails, stderr captured, status updates, one follow-up per task", async () => {
  const s = setup({ ...realProcess, config: escalationConfig });
  const first = once(s.runner, "finished", (task) => task.id === 1);
  await s.tool("background_task").execute("c1", { command: "echo out; echo err >&2" }, undefined, undefined, s.ctx);
  await first;
  const ok = s.runner.tasks.get(1)!;
  assert.equal(ok.status, "completed");
  assert.equal(ok.exitCode, 0);
  assert.ok(ok.output.includes("out") && ok.output.includes("err"), `output was ${JSON.stringify(ok.output)}`);

  const second = once(s.runner, "finished", (task) => task.id === 2);
  await s.tool("background_task").execute("c2", { command: "echo bad >&2; exit 7" }, undefined, undefined, s.ctx);
  await second;
  const failed = s.runner.tasks.get(2)!;
  assert.equal(failed.status, "failed");
  assert.equal(failed.exitCode, 7);
  assert.ok(failed.output.includes("bad"));

  assert.ok(s.ctx.statuses.includes("Tasks: 1"), "status bar showed a running task");
  assert.equal(s.ctx.statuses.at(-1), undefined, "status bar cleared after the last task");
  assert.equal(s.messages.length, 2, "one follow-up per task");
  assert.ok(s.messages[0].text.includes("#1 finished (completed, exit 0)"));
  assert.ok(s.messages[1].text.includes("#2 finished (failed, exit 7)"));
  assert.deepEqual(s.messages[0].options, { deliverAs: "followUp" });
});

test("real process escalation: SIGTERM ignored, SIGKILL reaches the OS, status cancelled, no follow-up", async () => {
  const s = setup({ ...realProcess, config: escalationConfig });
  const ready = once(s.runner, "output", (task, chunk) => task.id === 1 && chunk.includes("ready"));
  await s.tool("background_task").execute("c", { command: "trap '' TERM; echo ready; while :; do sleep 30; done" }, undefined, undefined, s.ctx);
  const [task] = await ready;
  const pid = task.pid!;

  const outcome = await s.runner.cancel(task);
  assert.equal(outcome, "cancelled");
  assert.equal(task.status, "cancelled");
  assert.equal(task.signal, "SIGKILL");
  assert.equal(task.exitCode, null);
  assert.ok(isDead(pid), "shell leader is dead at the OS level");
  assert.ok(isDead(-pid), "process group has no members left");
  assert.equal(s.messages.length, 0, "explicit cancellation sends no follow-up");
});

test("async session_shutdown awaits real tasks, including a SIGTERM-ignoring one, and clears the list", async () => {
  const s = setup({ ...realProcess, config: escalationConfig });
  const readyOne = once(s.runner, "output", (task, chunk) => task.id === 1 && chunk.includes("ready"));
  const readyTwo = once(s.runner, "output", (task, chunk) => task.id === 2 && chunk.includes("ready"));
  await s.tool("background_task").execute("c1", { command: "echo ready; exec sleep 30" }, undefined, undefined, s.ctx);
  await s.tool("background_task").execute("c2", { command: "trap '' TERM; echo ready; while :; do sleep 30; done" }, undefined, undefined, s.ctx);
  await Promise.all([readyOne, readyTwo]);
  const pids = [...s.runner.tasks.values()].map((task) => task.pid!);
  assert.equal(pids.length, 2);

  await s.handlers.get("session_shutdown")!({}, s.ctx);
  for (const pid of pids) {
    assert.ok(isDead(pid), `pid ${pid} is dead after shutdown`);
    assert.ok(isDead(-pid), `group ${pid} is empty after shutdown`);
  }
  assert.equal(s.runner.tasks.size, 0);
  assert.equal(s.messages.length, 0, "shutdown sends no follow-up");
});

test("termination failure: tool and slash keep the task running, report an error, and never mark it cancelled", async () => {
  const child = new FakeChild(4242);
  const s = setup({ spawn: () => child as any });
  await s.tool("background_task").execute("c", { command: "sleep 1000" }, undefined, undefined, s.ctx);
  const task = s.runner.tasks.get(1)!;

  const toolError = await s.tool("background_task_cancel")
    .execute("c", { id: 1 }, undefined, undefined, s.ctx)
    .then(() => null, (error: Error) => error);
  assert.ok(toolError instanceof Error, "tool reports failure");
  assert.match(toolError.message, /Could not confirm/);
  assert.equal(task.status, "running");
  assert.equal(s.runner.tasks.get(1), task, "task retained");

  await s.command("task-cancel").handler("1", s.ctx);
  assert.equal(s.ctx.notes.at(-1)?.level, "error");
  assert.match(s.ctx.notes.at(-1)!.message, /Could not confirm/);
  assert.equal(task.status, "running");
  assert.ok(s.signals.some(([pid, signal]) => pid === -4242 && signal === "SIGTERM"));
  assert.ok(s.signals.some(([pid, signal]) => pid === -4242 && signal === "SIGKILL"));
  assert.equal(s.messages.length, 0);
});

test("split multibyte UTF-8 across stdout and stderr chunks decodes exactly, with no replacement characters", async () => {
  const child = new FakeChild(5151);
  const s = setup({ spawn: () => child as any });
  const done = once(s.runner, "finished");
  await s.tool("background_task").execute("c", { command: "emit" }, undefined, undefined, s.ctx);
  child.stdout.emit("data", Buffer.from([0xf0, 0x9f]));
  child.stderr.emit("data", Buffer.from([0xe2, 0x82]));
  child.stdout.emit("data", Buffer.from([0x9a, 0x80]));
  child.stderr.emit("data", Buffer.from([0xac, 0x0a]));
  child.emit("exit", 0, null);
  child.emit("close", 0, null);
  await done;
  const output = s.runner.tasks.get(1)!.output;
  assert.equal(output, "🚀€\n");
  assert.ok(!output.includes("�"));
});

test("signal reporting: a child killed by a signal is failed with null exit, the signal in list, result and follow-up", async () => {
  const s = setup({ ...realProcess, config: escalationConfig });
  const ready = once(s.runner, "output", (task, chunk) => task.id === 1 && chunk.includes("ready"));
  const finished = once(s.runner, "finished");
  await s.tool("background_task").execute("c", { command: "echo ready; exec sleep 30" }, undefined, undefined, s.ctx);
  const [task] = await ready;
  process.kill(task.pid!, "SIGKILL");
  await finished;

  assert.equal(task.status, "failed");
  assert.equal(task.exitCode, null);
  assert.equal(task.signal, "SIGKILL");
  const listed = text(await s.tool("background_task_list").execute("l", {}, undefined, undefined, s.ctx));
  assert.match(listed, /\[failed\].*signal SIGKILL/);
  const result = await s.tool("background_task_result").execute("r", { id: 1 }, undefined, undefined, s.ctx);
  assert.match(text(result), /signal SIGKILL/);
  assert.equal(result.details.signal, "SIGKILL");
  assert.equal(s.messages.length, 1);
  assert.match(s.messages[0].text, /failed, signal SIGKILL/);
});

test("stable completed duration: elapsed time grows while running, then freezes at settlement", async () => {
  const child = new FakeChild(6161);
  let clock = 1_000;
  const s = setup({ spawn: () => child as any, now: () => clock });
  const done = once(s.runner, "finished");
  await s.tool("background_task").execute("c", { command: "work" }, undefined, undefined, s.ctx);
  clock += 1_500;
  const running = await s.tool("background_task_result").execute("r", { id: 1 }, undefined, undefined, s.ctx);
  assert.equal(running.details.elapsedMs, 1_500);

  child.emit("exit", 0, null);
  child.emit("close", 0, null);
  await done;
  clock += 60_000;
  const settled = await s.tool("background_task_result").execute("r", { id: 1 }, undefined, undefined, s.ctx);
  assert.equal(settled.details.elapsedMs, 1_500, "duration froze at settlement");
  clock += 60_000;
  const later = await s.tool("background_task_list").execute("l", {}, undefined, undefined, s.ctx);
  assert.equal(later.details.tasks[0].elapsedMs, 1_500);
});

test("bounded retention keeps running tasks: limit 3 with #1 running and #2-#5 finished retains #1, #3, #4, #5", async () => {
  const children = [1, 2, 3, 4, 5].map((n) => new FakeChild(7000 + n));
  let next = 0;
  const s = setup({ spawn: () => children[next++] as any, config: { ...fastConfig, finishedRetentionLimit: 3 } });
  for (let i = 0; i < 5; i++) {
    await s.tool("background_task").execute(`c${i}`, { command: `task ${i + 1}` }, undefined, undefined, s.ctx);
  }
  for (const child of children.slice(1)) {
    child.emit("exit", 0, null);
    child.emit("close", 0, null);
  }
  assert.deepEqual([...s.runner.tasks.keys()], [1, 3, 4, 5]);
  assert.equal(s.runner.tasks.get(1)!.status, "running");
});

test("strict ID validation: tool and commands reject malformed IDs; unknown 999 is reported as unknown", async () => {
  const s = setup();
  const cases: Array<[unknown, RegExp]> = [
    ["", /Invalid background task ID/],
    [0, /Invalid background task ID/],
    [-1, /Invalid background task ID/],
    [1.5, /Invalid background task ID/],
    ["01", /Invalid background task ID/],
    ["abc", /Invalid background task ID/],
    [NaN, /Invalid background task ID/],
    [999, /No background task #999/],
  ];
  for (const [id, pattern] of cases) {
    for (const name of ["background_task_result", "background_task_cancel"]) {
      await assert.rejects(s.tool(name).execute("c", { id }, undefined, undefined, s.ctx), pattern, `${name} with ${String(id)}`);
    }
    const before = s.ctx.notes.length;
    await s.command("task-result").handler(String(id), s.ctx);
    await s.command("task-cancel").handler(String(id), s.ctx);
    const notes = s.ctx.notes.slice(before);
    assert.equal(notes.length, 2, `two notices for ${String(id)}`);
    for (const note of notes) {
      assert.equal(note.level, "error");
      assert.match(note.message, pattern);
    }
  }
  assert.equal(s.signals.length, 0, "rejected IDs send no signals");
});

test("parseTaskId accepts canonical positive safe integers only", () => {
  assert.equal(parseTaskId(1), 1);
  assert.equal(parseTaskId("12"), 12);
  assert.equal(parseTaskId(Number.MAX_SAFE_INTEGER), Number.MAX_SAFE_INTEGER);
  for (const bad of ["", " 1", "+1", "1e2", "0x1", Number.MAX_SAFE_INTEGER + 2, "9007199254740993", -0]) {
    assert.throws(() => parseTaskId(bad), /Invalid background task ID/, `accepted ${String(bad)}`);
  }
});

test("model controls: inspect a completed and a running task, cancel the running one, idempotent second cancel", async () => {
  const s = setup({ ...realProcess, config: escalationConfig });
  const finished = once(s.runner, "finished", (task) => task.id === 1);
  await s.tool("background_task").execute("c1", { command: "echo done" }, undefined, undefined, s.ctx);
  await finished;
  const completed = text(await s.tool("background_task_result").execute("r1", { id: 1 }, undefined, undefined, s.ctx));
  assert.match(completed, /^#1 \[completed\]/);
  assert.match(completed, /done/);

  const ready = once(s.runner, "output", (task, chunk) => task.id === 2 && chunk.includes("ready"));
  await s.tool("background_task").execute("c2", { command: "echo ready; exec sleep 30" }, undefined, undefined, s.ctx);
  await ready;
  const running = text(await s.tool("background_task_result").execute("r2", { id: 2 }, undefined, undefined, s.ctx));
  assert.match(running, /^#2 \[running\]/);

  const cancelled = await s.tool("background_task_cancel").execute("x1", { id: 2 }, undefined, undefined, s.ctx);
  assert.match(text(cancelled), /Cancelled background command #2/);
  assert.equal(s.runner.tasks.get(2)!.status, "cancelled");
  const again = await s.tool("background_task_cancel").execute("x2", { id: 2 }, undefined, undefined, s.ctx);
  assert.match(text(again), /already cancelled/);
  assert.equal(s.runner.tasks.get(2)!.status, "cancelled");
  assert.equal(s.messages.length, 1, "only the completed task produced a follow-up");
});

test("duplicate error then close: one notification, frozen end time, and no signals sent to a possibly reused PID", async () => {
  const child = new FakeChild(7777);
  let clock = 0;
  const s = setup({ spawn: () => child as any, now: () => clock });
  const done = once(s.runner, "finished");
  await s.tool("background_task").execute("c", { command: "sleep 1000" }, undefined, undefined, s.ctx);
  clock = 100;
  child.emit("error", new Error("spawn boom"));
  child.emit("close", null, null);
  await done;

  const task = s.runner.tasks.get(1)!;
  assert.equal(task.status, "failed");
  assert.ok(task.output.includes("spawn boom"));
  assert.equal(s.messages.length, 1, "single notification");
  const frozen = s.runner.elapsedMs(task);
  clock = 5_000;
  assert.equal(s.runner.elapsedMs(task), frozen, "end time frozen");

  const outcome = await s.runner.cancel(task);
  assert.equal(outcome, "already-ended");
  assert.equal(s.signals.length, 0, "no signal after the leader was reported gone");
  assert.equal(s.messages.length, 1);
});

test("config: the spec file loads with the documented bounds", () => {
  const config = parseBackgroundTaskConfig(readFileSync(SPEC_PATH, "utf8"), "spec/pi-background-tasks.yaml");
  assert.deepEqual(config, {
    outputLimitChars: 16_000,
    finishedRetentionLimit: 50,
    stop: { graceMs: 1_000, killWaitMs: 2_000, pollMs: 25 },
  });
});

test("config: unknown, missing, non-positive and non-JSON-compatible values fail loudly", () => {
  const valid = {
    output_limit_chars: 1,
    finished_retention_limit: 1,
    stop: { grace_ms: 1, kill_wait_ms: 1, poll_ms: 1 },
  };
  const parse = (value: unknown) => parseBackgroundTaskConfig(JSON.stringify(value), "test");
  assert.ok(parse(valid));
  assert.throws(() => parse({ ...valid, extra: 1 }), /unknown key "extra"/);
  assert.throws(() => parse({ ...valid, stop: { grace_ms: 1, kill_wait_ms: 1 } }), /missing key "poll_ms"/);
  assert.throws(() => parse({ ...valid, output_limit_chars: 0 }), /positive safe integer/);
  assert.throws(() => parse({ ...valid, finished_retention_limit: 1.5 }), /positive safe integer/);
  assert.throws(() => parseBackgroundTaskConfig("output_limit_chars: 1\n", "test"), /not JSON-compatible YAML/);
});

test("extension entry registers the four tools and four commands", () => {
  const fake = fakePi();
  backgroundTasksExtension(fake.pi as any);
  assert.deepEqual([...fake.tools.keys()].sort(), [
    "background_task",
    "background_task_cancel",
    "background_task_list",
    "background_task_result",
  ]);
  assert.deepEqual([...fake.commands.keys()].sort(), ["task", "task-cancel", "task-result", "tasks"]);
});

test("zombie group members do not block confirmation: the group probe still finds the group, but no live process is in it", async () => {
  const child = new FakeChild(9100);
  const sent: Array<[number, string | number]> = [];
  const s = setup({
    spawn: () => child as any,
    kill: (pid, signal) => {
      sent.push([pid, signal]);
      // The leader dies on SIGTERM. Its grandchild is an unreaped zombie, so signal 0 to the group still succeeds.
      if (signal === "SIGTERM") child.emit("exit", null, "SIGTERM");
    },
    liveGroupMembers: () => 0,
  });
  await s.tool("background_task").execute("c", { command: "sleep 1000" }, undefined, undefined, s.ctx);
  const task = s.runner.tasks.get(1)!;

  const outcome = await s.runner.cancel(task);
  child.emit("close", null, "SIGTERM");
  assert.equal(outcome, "cancelled");
  assert.equal(task.status, "cancelled");
  assert.deepEqual(sent, [[-9100, "SIGTERM"]], "confirmed empty at once: no escalation to SIGKILL");
  assert.equal(s.messages.length, 0, "explicit cancellation sends no follow-up");
});

test("unconfirmed cancel: the task stays running, is watched, and settles with its real exit and a follow-up", async () => {
  const child = new FakeChild(9200);
  let live = 1;
  const s = setup({ spawn: () => child as any, liveGroupMembers: () => live });
  await s.tool("background_task").execute("c", { command: "sleep 1000" }, undefined, undefined, s.ctx);
  const task = s.runner.tasks.get(1)!;

  assert.equal(await s.runner.cancel(task), "not-confirmed");
  assert.equal(task.status, "running");

  // The shell leader dies by SIGKILL, but a live member keeps the group: the task must stay running.
  child.emit("exit", null, "SIGKILL");
  assert.equal(task.status, "running", "a live group member keeps the task running");
  const done = once(s.runner, "finished", (finished) => finished.id === 1);
  live = 0;
  child.emit("close", null, "SIGKILL");
  await done;

  assert.equal(task.status, "failed", "settled by its real exit, not hidden as cancelled");
  assert.equal(task.signal, "SIGKILL");
  assert.equal(s.messages.length, 1, "the follow-up is not suppressed after an unconfirmed cancel");
  assert.match(s.messages[0].text, /failed, signal SIGKILL/);
});

test("reaped leader: the group is signalled only while a live member is seen, and never after it is empty", async () => {
  const child = new FakeChild(9300);
  let live = 1;
  const sent: Array<[number, string | number]> = [];
  const s = setup({
    spawn: () => child as any,
    kill: (pid, signal) => sent.push([pid, signal]),
    liveGroupMembers: () => live,
  });
  await s.tool("background_task").execute("c", { command: "sleep 1000" }, undefined, undefined, s.ctx);
  const task = s.runner.tasks.get(1)!;
  assert.equal(await s.runner.cancel(task), "not-confirmed");

  child.emit("exit", null, "SIGKILL");
  sent.length = 0;
  // Leader reaped, live member still present: the PGID is held, so a retry may signal the group.
  assert.equal(await s.runner.cancel(task), "not-confirmed");
  assert.deepEqual(sent.map(([pid]) => pid), [-9300, -9300], "retry signals the group while live");

  sent.length = 0;
  live = 0;
  const done = once(s.runner, "finished", (finished) => finished.id === 1);
  child.emit("close", null, "SIGKILL");
  await done;
  assert.equal(task.status, "failed");
  assert.equal(await s.runner.cancel(task), "already-ended");
  assert.deepEqual(sent, [], "no signal once the group is empty: its ID may be reused");
  assert.equal(s.messages.length, 1);
});

test("unknown end: a spawn error reports exit unknown in list, result and follow-up, never exit null", async () => {
  const child = new FakeChild(9400);
  const s = setup({ spawn: () => child as any });
  const done = once(s.runner, "finished");
  await s.tool("background_task").execute("c", { command: "broken" }, undefined, undefined, s.ctx);
  child.emit("error", new Error("spawn boom"));
  child.emit("close", null, null);
  await done;

  const listed = text(await s.tool("background_task_list").execute("l", {}, undefined, undefined, s.ctx));
  assert.match(listed, /\[failed\].*exit unknown/);
  const result = text(await s.tool("background_task_result").execute("r", { id: 1 }, undefined, undefined, s.ctx));
  assert.match(result, /exit unknown/);
  assert.match(s.messages[0].text, /failed, exit unknown/);
  assert.doesNotMatch(`${listed}\n${result}\n${s.messages[0].text}`, /exit null/);
});

test("held pipe: a child that outlives the shell keeps stdout open; output is finalized after the bounded drain with a note", async () => {
  const s = setup({ ...realProcess, config: fastConfig });
  const finished = once(s.runner, "finished", (task) => task.id === 1);
  await s.tool("background_task").execute("c", { command: "echo held; sleep 2 &" }, undefined, undefined, s.ctx);
  await finished;

  const task = s.runner.tasks.get(1)!;
  assert.equal(task.status, "completed");
  assert.equal(task.exitCode, 0);
  assert.match(task.output, /held/);
  assert.match(task.output, /output streams stayed open after the command exited/);
  assert.equal(s.messages.length, 1, "one follow-up once output is final");
});

const ESCAPE_SKIP = !IS_LINUX ? "setsid escape test is Linux-only" : !HAS_SETSID ? "setsid is missing; install util-linux" : false;
test("escaped child: cancellation confirms only the task's group; the escaped process is outside it and not reported dead", { skip: ESCAPE_SKIP }, async () => {
  const s = setup({ ...realProcess, config: escalationConfig });
  // The escaped shell reports its own $$: setsid may fork, so $! of the background job need not be the live process.
  const reported = once(s.runner, "output", (task) => task.id === 1 && /escaped \d+/.test(task.output));
  await s.tool("background_task").execute("c", { command: 'setsid sh -c \'echo "escaped $$"; exec sleep 30\' & exec sleep 30' }, undefined, undefined, s.ctx);
  const [task] = await reported;
  const escaped = Number(/escaped (\d+)/.exec(task.output)![1]);

  assert.equal(await s.runner.cancel(task), "cancelled");
  assert.equal(task.status, "cancelled");
  assert.ok(isDead(-task.pid!), "the task's group has no live member");
  assert.ok(!isDead(escaped), "the escaped process is still alive; cancelled describes the group only");
  process.kill(escaped, "SIGKILL"); // clean up our own process
});

test("live group probe: counts a live member, and reports zero once the group is empty (Linux)", { skip: !IS_LINUX && "/proc group reader is Linux-only" }, async () => {
  const child = spawn("sleep", ["30"], { detached: true, stdio: "ignore" });
  const exited = once(child, "exit");
  assert.equal(liveProcessGroupMembers(child.pid!), 1, "a running member is live");
  process.kill(-child.pid!, "SIGKILL");
  await exited;
  assert.equal(liveProcessGroupMembers(child.pid!), 0, "an empty group has no live members");
});

test("UTF-16 truncation boundaries: every output limit keeps a valid suffix with no lone surrogate", async () => {
  // "a🚀b" is four UTF-16 code units: a, high surrogate, low surrogate, b.
  const expected: Record<number, string> = { 1: "b", 2: "b", 3: "🚀b", 4: "a🚀b", 5: "a🚀b" };
  for (const [limit, want] of Object.entries(expected)) {
    const child = new FakeChild(9500 + Number(limit));
    const s = setup({ spawn: () => child as any, config: { ...fastConfig, outputLimitChars: Number(limit) } });
    await s.tool("background_task").execute("c", { command: "emit" }, undefined, undefined, s.ctx);
    child.stdout.emit("data", Buffer.from("a🚀b", "utf8"));
    const output = s.runner.tasks.get(1)!.output;
    assert.equal(output, want, `limit ${limit}`);
    assert.doesNotMatch(output, /[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/, `limit ${limit} has a lone surrogate`);
  }
});

test("slash actions with valid IDs: /task starts, /tasks lists, /task-result shows output, /task-cancel stops a running task", async () => {
  const s = setup({ ...realProcess, config: escalationConfig });
  const finished = once(s.runner, "finished", (task) => task.id === 1);
  await s.command("task").handler("echo slash-ok", s.ctx);
  assert.match(s.ctx.notes.at(-1)!.message, /Started background command #1: echo slash-ok/);
  await finished;

  await s.command("tasks").handler("", s.ctx);
  assert.match(s.ctx.notes.at(-1)!.message, /#1 \[completed\]/);
  await s.command("task-result").handler("1", s.ctx);
  assert.match(s.ctx.notes.at(-1)!.message, /#1 \[completed\][\s\S]*slash-ok/);

  await s.command("task").handler("exec sleep 30", s.ctx);
  const running = s.runner.tasks.get(2)!;
  await s.command("task-cancel").handler("2", s.ctx);
  assert.equal(running.status, "cancelled");
  assert.match(s.ctx.notes.at(-1)!.message, /Cancelled background command #2/);
  assert.equal(s.ctx.notes.at(-1)!.level, "info");
  assert.equal(s.messages.length, 1, "only the completed task produced a follow-up");
});

test("shutdown releases a task whose escaped child keeps the output pipe open: no output, timer or stream activity after it", { skip: ESCAPE_SKIP }, async () => {
  const s = setup({ ...realProcess, config: escalationConfig });
  const seen: string[] = [];
  s.runner.on("output", (task: BackgroundTask, chunk: string) => {
    if (task.id === 1) seen.push(chunk);
  });
  const ticked = once(s.runner, "output", (task, chunk) => task.id === 1 && chunk.includes("tick"));
  const exited = once(s.runner, "changed", (task) => task.id === 1 && task.status !== "running");
  // The shell exits at once; the setsid child keeps writing to the inherited pipe, so output stays open.
  await s.tool("background_task").execute("c", { command: 'setsid sh -c \'echo "escaped $$"; while :; do echo tick; sleep 0.05; done\' & exit 0' }, undefined, undefined, s.ctx);
  await Promise.all([ticked, exited]);
  const task = s.runner.tasks.get(1)!;
  assert.equal(task.status, "completed");
  assert.equal(task.outputDone, false, "the escaped child holds the pipe: output is not final");
  assert.ok(task.drainTimer, "the drain timer is pending");
  const escaped = Number(/escaped (\d+)/.exec(task.output)![1]);

  assert.deepEqual(await s.runner.shutdown(), []);
  const atShutdown = seen.length;
  assert.equal(task.outputDone, true, "shutdown finalized the output");
  assert.equal(task.drainTimer, undefined, "shutdown cleared the drain timer");
  assert.equal(task.child.stdout!.destroyed, true, "shutdown destroyed the stream");
  // One event-loop turn: a destroyed stream cannot deliver more data, so no output event may follow.
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(seen.length, atShutdown, "no output event after shutdown");
  assert.equal(s.messages.length, 0, "shutdown sends no follow-up");
  try {
    process.kill(escaped, "SIGKILL"); // clean up our own process; it may already have died on SIGPIPE
  } catch {
    // already gone
  }
});

test("retry boundary: a group that empties after the leader's exit settles as a natural end once, and the retry reports already-ended", async () => {
  const child = new FakeChild(9600);
  let live = 1;
  const s = setup({ spawn: () => child as any, liveGroupMembers: () => live });
  await s.tool("background_task").execute("c", { command: "sleep 1000" }, undefined, undefined, s.ctx);
  const task = s.runner.tasks.get(1)!;
  assert.equal(await s.runner.cancel(task), "not-confirmed");

  child.emit("exit", 1, null);
  assert.equal(task.status, "running", "a group member keeps the task running");
  // The group empties, and the retry runs before any watcher tick: no timer turn separates the two calls.
  live = 0;
  const signalsBefore = s.signals.length;
  assert.equal(await s.runner.cancel(task), "already-ended");
  assert.equal(task.status, "failed", "settled by its real exit, not as cancelled");
  assert.equal(s.signals.length, signalsBefore, "no signal to a group that is already empty");
  assert.equal(s.messages.length, 0, "output is not final yet: no follow-up");

  child.emit("close", 1, null);
  assert.equal(s.messages.length, 1, "one follow-up once output is final");
  assert.match(s.messages[0].text, /failed, exit 1/);
  assert.equal(await s.runner.cancel(task), "already-ended");
  assert.equal(s.messages.length, 1, "no second follow-up");
});

test("concurrent retries share one attempt: one SIGTERM and one SIGKILL, the same promise and the same outcome", async () => {
  const child = new FakeChild(9700);
  const s = setup({ spawn: () => child as any, liveGroupMembers: () => 1 });
  await s.tool("background_task").execute("c", { command: "sleep 1000" }, undefined, undefined, s.ctx);
  const task = s.runner.tasks.get(1)!;

  const first = s.runner.cancel(task);
  const second = s.runner.cancel(task);
  assert.strictEqual(second, first, "a call during an attempt joins it");
  assert.deepEqual(await Promise.all([first, second]), ["not-confirmed", "not-confirmed"]);
  assert.deepEqual(s.signals.filter(([pid]) => pid === -9700), [[-9700, "SIGTERM"], [-9700, "SIGKILL"]]);
  assert.equal(task.status, "running");
});

test("successful retry: a leader that already exited, then a group emptied by the retry's SIGTERM, settles cancelled with no follow-up", async () => {
  const child = new FakeChild(9800);
  let live = 1;
  let drainOnTerm = false;
  const sent: Array<[number, string | number]> = [];
  const s = setup({
    spawn: () => child as any,
    kill: (pid, signal) => {
      sent.push([pid, signal]);
      if (signal === "SIGTERM" && drainOnTerm) live = 0;
    },
    liveGroupMembers: () => live,
  });
  await s.tool("background_task").execute("c", { command: "sleep 1000" }, undefined, undefined, s.ctx);
  const task = s.runner.tasks.get(1)!;
  assert.equal(await s.runner.cancel(task), "not-confirmed");

  child.emit("exit", 0, null);
  assert.equal(task.status, "running", "a group member keeps the task running after its leader exits");

  drainOnTerm = true;
  sent.length = 0;
  assert.equal(await s.runner.cancel(task), "cancelled");
  assert.equal(task.status, "cancelled");
  assert.deepEqual(sent, [[-9800, "SIGTERM"]], "the SIGTERM confirmed the group: no SIGKILL");
  child.emit("close", 0, null);
  assert.equal(s.messages.length, 0, "a cancelled task sends no follow-up");
});

test("unknown liveness: a successful group probe keeps the task running; a probe that finds nothing settles it", async () => {
  const child = new FakeChild(9900);
  let probeFinds = true;
  const s = setup({
    spawn: () => child as any,
    kill: (_pid, signal) => {
      if (signal === 0 && !probeFinds) throw Object.assign(new Error("no such group"), { code: "ESRCH" });
    },
    liveGroupMembers: () => undefined,
  });
  await s.tool("background_task").execute("c", { command: "sleep 1000" }, undefined, undefined, s.ctx);
  const task = s.runner.tasks.get(1)!;
  assert.equal(await s.runner.cancel(task), "not-confirmed", "the leader is still alive");

  child.emit("exit", null, "SIGTERM");
  assert.equal(task.status, "running", "unknown liveness: the probe finds the group, so it is not empty");
  const done = once(s.runner, "finished", (finished) => finished.id === 1);
  probeFinds = false;
  child.emit("close", null, "SIGTERM");
  await done;
  assert.equal(task.status, "failed");
  assert.equal(task.signal, "SIGTERM");
  assert.equal(s.messages.length, 1);
});

test("shutdown releases an unconfirmed task: its child's later exit and close produce no events and no follow-up", async () => {
  const child = new FakeChild(9950);
  const s = setup({ spawn: () => child as any, liveGroupMembers: () => 1 });
  await s.tool("background_task").execute("c", { command: "sleep 1000" }, undefined, undefined, s.ctx);
  const task = s.runner.tasks.get(1)!;
  assert.equal(await s.runner.cancel(task), "not-confirmed");

  const events: string[] = [];
  for (const name of ["changed", "output", "finished"]) s.runner.on(name, () => events.push(name));
  assert.deepEqual(await s.runner.shutdown(), [1], "reported as not confirmed");
  const atShutdown = events.length;
  child.emit("exit", null, "SIGKILL");
  child.emit("close", null, "SIGKILL");
  assert.equal(events.length, atShutdown, "no event from the released child");
  assert.equal(task.drainTimer, undefined, "no drain timer was armed after release");
  assert.equal(s.messages.length, 0);
});

test("parseProcStat reads the state and group after the command name, which may contain spaces and parentheses", () => {
  assert.deepEqual(parseProcStat("123 (a) b) S 1 9 0 0"), { state: "S", group: 9 });
  assert.deepEqual(parseProcStat("77 (sh) Z 1 42 42 0\n"), { state: "Z", group: 42 });
});

test("parseProcStat rejects malformed lines instead of reading a missing or non-numeric field", () => {
  for (const line of ["", "123", "123 sh S 1 9", "123 (sh)S 1 9", "123 (sh) 1 9", "123 (sh) S x 9", "123 (sh) S 1", "123 (sh) S 1 -4", "123 (sh) S 1 9x", "123 (sh) S  9"]) {
    assert.equal(parseProcStat(line), undefined, `accepted ${JSON.stringify(line)}`);
  }
});

/** Fake /proc roots for the group reader: a directory per PID, with its stat file as given. */
const DIRECTORY = "<directory>";
const MISSING = "<missing>";
const procRoots: string[] = [];
after(() => {
  for (const root of procRoots) rmSync(root, { recursive: true, force: true });
});

function fakeProc(entries: Record<string, string>): string {
  const root = mkdtempSync(join(tmpdir(), "pi-proc-"));
  procRoots.push(root);
  for (const [pid, text] of Object.entries(entries)) {
    mkdirSync(join(root, pid), { recursive: true });
    if (text === DIRECTORY) mkdirSync(join(root, pid, "stat"));
    else if (text !== MISSING) writeFileSync(join(root, pid, "stat"), text);
  }
  return root;
}

test("liveProcessGroupMembers counts live members, skips vanished entries, and is unknown for unreadable or malformed data", () => {
  const healthy = fakeProc({
    "1": "1 (init) S 0 42 42 0",
    "2": "2 (dead) Z 1 42 42 0", // zombie: not live
    "3": "3 (other) S 1 43 43 0", // another group
    "4": MISSING, // vanished between readdir and read
  });
  assert.equal(liveProcessGroupMembers(42, healthy), 1);
  assert.equal(liveProcessGroupMembers(44, healthy), 0, "a readable group with no member is empty");
  assert.equal(liveProcessGroupMembers(42, fakeProc({ "1": "1 (init) S 0 42 42 0", "5": DIRECTORY })), undefined, "unreadable entry: unknown, not empty");
  assert.equal(liveProcessGroupMembers(42, fakeProc({ "1": "1 (init) S 0 42 42 0", "6": "garbage" })), undefined, "malformed entry: unknown, not empty");
  assert.equal(liveProcessGroupMembers(42, fakeProc({ "7": "7 (x) S 0" })), undefined, "truncated before the group field: unknown, not empty");
  assert.equal(liveProcessGroupMembers(42, join(tmpdir(), "pi-proc-no-such-root")), undefined, "no /proc root: unknown");
});

// Deterministic timers: no wall-clock waits or sleeps.
import { test } from "node:test";
import assert from "node:assert/strict";
import { CompletionDelivery, type DeliveryConfig, type DeliveryTimers } from "../../.pi/lib/background-task-runner.ts";

const config: DeliveryConfig = { holdWarnMs: 1000, flushPollMs: 5, batchWindowMs: 100, batchMaxTasks: 8, batchMaxChars: 8192, entryMaxChars: 768 };
function harness(overrides: Partial<DeliveryConfig> = {}, send?: (text: string) => void) {
  const sent: string[] = [];
  const released: number[] = [];
  const notes: Array<{ message: string; level: string }> = [];
  let now = 0;
  const pending = new Set<{ callback: () => void; due: number }>();
  const timers: DeliveryTimers = {
    set: (callback, ms) => {
      const handle = { callback, due: now + ms };
      pending.add(handle);
      return handle;
    },
    clear: (handle) => pending.delete(handle as { callback: () => void; due: number }),
  };
  const ctx = {
    idle: true, queued: false,
    isIdle: () => ctx.idle,
    hasPendingMessages: () => ctx.queued,
    ui: { notify: (message: string, level: string) => notes.push({ message, level }) },
  };
  const delivery = new CompletionDelivery((text) => { send?.(text); sent.push(text); }, { ...config, ...overrides }, timers, (id) => released.push(id));
  delivery.attach(ctx);
  const advance = (ms: number) => {
    const target = now + ms;
    for (;;) {
      const next = [...pending].filter((h) => h.due <= target).sort((a, b) => a.due - b.due)[0];
      if (!next) break;
      now = next.due;
      pending.delete(next);
      next.callback();
    }
    now = target;
  };
  return { sent, released, notes, pending, ctx, delivery, advance };
}

function identities(messages: string[]): number[] {
  return [...messages.join("\n").matchAll(/^#(\d+): /gm)].map((m) => Number(m[1]));
}

test("first arrival fixes the window; simultaneous and staggered results form one FIFO batch", () => {
  const h = harness();
  h.delivery.submit(2, "success");
  h.advance(60);
  h.delivery.submit(1, "failed, exit 7");
  h.delivery.submit(3, "success");
  h.advance(39);
  assert.equal(h.sent.length, 0);
  assert.deepEqual(h.released, []);
  h.advance(1);
  assert.equal(h.sent.length, 1, "later arrivals did not restart the window");
  assert.deepEqual(identities(h.sent), [2, 1, 3]);
  assert.match(h.sent[0], /failed, exit 7/);
  assert.match(h.sent[0], /background_task_result/);
  assert.match(h.sent[0], /do not automatically summarize the session/);
  assert.deepEqual(h.released, [2, 1, 3]);
  h.advance(5);
  assert.equal(h.notes.filter((n) => /once each: #2, #1, #3\. Pi gives no acknowledgement/.test(n.message)).length, 1);
  assert.equal(h.pending.size, 0);
  h.advance(1000);
  assert.equal(h.sent.length, 1, "no retry without acknowledgement");
});

test("each fresh queue gets a full window even with a pending drain warning", () => {
  const h = harness();
  h.delivery.submit(1, "one");
  h.advance(100);
  h.delivery.submit(2, "two");
  h.advance(99);
  assert.equal(h.sent.length, 1);
  h.advance(1);
  assert.deepEqual(identities(h.sent), [1, 2]);
});

test("task-count splitting preserves FIFO, rechecks guard each poll, and includes new arrivals at the tail", () => {
  const h = harness({ batchMaxTasks: 2 });
  for (const id of [3, 1, 2, 4, 5]) h.delivery.submit(id, `task ${id}`);
  h.advance(100);
  assert.deepEqual(identities(h.sent), [3, 1]);
  h.ctx.idle = false;
  h.advance(5);
  assert.equal(h.sent.length, 1, "busy without a known run may be compaction");
  h.delivery.submit(6, "six");
  h.delivery.agentStarted();
  h.advance(5);
  assert.deepEqual(identities(h.sent), [3, 1, 2, 4]);
  h.advance(5);
  assert.deepEqual(identities(h.sent), [3, 1, 2, 4, 5, 6]);
  assert.equal(h.sent.length, 3);
  h.advance(5);
  assert.equal(h.pending.size, 0);
});

test("character splitting clips huge entries, preserves even maximum safe task IDs, and bounds every message", () => {
  const h = harness({ entryMaxChars: 256, batchMaxChars: 512 });
  for (const id of [Number.MAX_SAFE_INTEGER, 1, 2]) h.delivery.submit(id, "x".repeat(100000));
  h.advance(110);
  assert.equal(h.sent.length, 3);
  assert.deepEqual(identities(h.sent), [Number.MAX_SAFE_INTEGER, 1, 2]);
  assert.ok(h.sent.every((message) => message.length <= 512 && message.includes("background_task_result")));
});

test("invalid bounds fail before accepting tasks", () => {
  assert.throws(() => harness({ entryMaxChars: 255 }), /entry_max_chars/);
  assert.throws(() => harness({ batchMaxChars: 511 }), /batch_max_chars/);
  assert.throws(() => harness({ entryMaxChars: 768, batchMaxChars: 800 }), /fit entry_max_chars/);
});

test("manual compaction holds FIFO until the end hook and the guard both clear", () => {
  const h = harness();
  h.ctx.idle = false;
  h.delivery.begin(h.ctx);
  for (const id of [2, 1, 3]) h.delivery.submit(id, `task ${id}`);
  h.advance(100);
  assert.equal(h.sent.length, 0);
  assert.equal(h.notes.filter((n) => /held until compaction ends/.test(n.message)).length, 3);
  h.delivery.end(h.ctx);
  assert.equal(h.sent.length, 0, "never send inside the end hook");
  h.advance(5);
  assert.equal(h.sent.length, 0, "later handlers can still hold the guard");
  h.ctx.idle = true;
  h.advance(5);
  assert.deepEqual(identities(h.sent), [2, 1, 3]);
  h.advance(5);
  assert.equal(h.pending.size, 0);
});

test("compaction starting during collection or between split batches pauses delivery until end", () => {
  const h = harness({ batchMaxTasks: 1 });
  h.delivery.submit(1, "one");
  h.delivery.submit(2, "two");
  h.advance(50);
  h.delivery.begin(h.ctx);
  h.advance(50);
  assert.equal(h.sent.length, 0);
  h.delivery.end(h.ctx);
  h.advance(5);
  assert.deepEqual(identities(h.sent), [1]);
  h.delivery.begin(h.ctx);
  h.advance(100);
  assert.deepEqual(identities(h.sent), [1]);
  h.delivery.end(h.ctx);
  h.advance(5);
  assert.deepEqual(identities(h.sent), [1, 2]);
});

test("end without a manual hold does nothing; compaction ending early cannot shorten collection", () => {
  const h = harness();
  h.delivery.end(h.ctx);
  assert.equal(h.pending.size, 0);
  h.delivery.begin(h.ctx);
  h.delivery.submit(1, "one");
  h.delivery.end(h.ctx);
  h.advance(99);
  assert.equal(h.sent.length, 0);
  h.advance(1);
  assert.deepEqual(identities(h.sent), [1]);
});

test("a long hold warns once; a missed end hook recovers when idle", () => {
  const h = harness();
  h.ctx.idle = false;
  h.delivery.begin(h.ctx);
  h.delivery.submit(1, "one");
  h.advance(2000);
  assert.equal(h.sent.length, 0);
  assert.equal(h.notes.filter((n) => /still held/.test(n.message)).length, 1);
  h.ctx.idle = true;
  h.advance(1010);
  assert.deepEqual(identities(h.sent), [1]);
  assert.equal(h.pending.size, 0);
});

test("shutdown clears window, poll and watch timers, releases pins, and returns only unsent IDs", () => {
  const h = harness({ batchMaxTasks: 1 });
  h.delivery.submit(1, "one");
  h.delivery.submit(2, "two");
  h.advance(100);
  h.ctx.idle = false;
  h.delivery.submit(3, "three");
  assert.deepEqual(h.delivery.shutdown(), [2, 3]);
  assert.deepEqual(h.released, [1, 2, 3]);
  assert.equal(h.pending.size, 0);
  assert.deepEqual(h.delivery.shutdown(), []);
  h.delivery.submit(4, "four");
  assert.deepEqual(h.released, [1, 2, 3, 4]);
  h.advance(2000);
  assert.deepEqual(identities(h.sent), [1]);
  const collecting = harness();
  collecting.delivery.submit(1, "one");
  assert.deepEqual(collecting.delivery.shutdown(), [1]);
  assert.equal(collecting.pending.size, 0);
});

test("throw releases all batch pins without retry; later batches still send", () => {
  const h = harness({ batchMaxTasks: 2 }, (text) => { if (text.includes("#1:")) throw new Error("boom"); });
  for (const id of [1, 2, 3]) h.delivery.submit(id, `task ${id}`);
  h.advance(110);
  assert.deepEqual(identities(h.sent), [3]);
  assert.deepEqual(h.released, [1, 2, 3]);
  assert.equal(h.notes.filter((n) => n.level === "error" && /#1, #2: Error: boom.*not retried/.test(n.message)).length, 1);
  assert.equal(h.pending.size, 0);
});

test("busy before any hook holds; a later hook takes over without duplicates", () => {
  const h = harness();
  h.ctx.idle = false;
  h.delivery.submit(1, "one");
  h.advance(100);
  assert.equal(h.sent.length, 0);
  h.delivery.begin(h.ctx);
  h.delivery.submit(2, "two");
  h.ctx.idle = true;
  h.delivery.end(h.ctx);
  h.advance(10);
  assert.deepEqual(identities(h.sent), [1, 2]);
});

test("a known run permits batched streaming, but agent_settled restores the guard", () => {
  const h = harness();
  h.ctx.idle = false;
  h.delivery.submit(1, "one");
  h.advance(99);
  assert.equal(h.sent.length, 0);
  h.delivery.agentStarted();
  h.advance(1);
  assert.deepEqual(identities(h.sent), [1]);
  h.delivery.agentSettled();
  h.delivery.submit(2, "two");
  h.advance(100);
  assert.deepEqual(identities(h.sent), [1]);
  h.ctx.idle = true;
  h.advance(5);
  assert.deepEqual(identities(h.sent), [1, 2]);
});

test("a stale context never sends held messages or throws from a timer", () => {
  const h = harness();
  h.delivery.begin(h.ctx);
  h.delivery.submit(1, "one");
  h.delivery.end(h.ctx);
  h.ctx.isIdle = () => { throw new Error("stale"); };
  h.advance(1005);
  assert.equal(h.sent.length, 0);
  assert.deepEqual(h.delivery.shutdown(), [1]);
});

test("the stranded check survives a slow later handler, warns once, and never resends", () => {
  const h = harness();
  h.ctx.idle = false;
  h.ctx.queued = true;
  h.delivery.begin(h.ctx);
  h.delivery.end(h.ctx);
  h.advance(5);
  assert.equal(h.notes.length, 0);
  h.ctx.idle = true;
  h.advance(5);
  assert.equal(h.notes.filter((n) => /still has a queued message/.test(n.message)).length, 1);
  assert.equal(h.sent.length, 0);
  assert.equal(h.pending.size, 0);
});

test("a run starting after compaction cancels the stranded check", () => {
  const h = harness();
  h.ctx.queued = true;
  h.delivery.begin(h.ctx);
  h.delivery.end(h.ctx);
  h.delivery.agentStarted();
  h.advance(5);
  assert.deepEqual(h.notes, []);
  assert.equal(h.pending.size, 0);
});

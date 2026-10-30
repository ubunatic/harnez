// Run: node --import ./test/pi/register.mjs --test test/pi/compaction-delivery.test.ts
// Timers are stepped by hand, so every ordering here is deterministic and no test sleeps.
import { test } from "node:test";
import assert from "node:assert/strict";
import { CompletionDelivery, type DeliveryTimers } from "../../.pi/lib/background-task-runner.ts";

function harness() {
  const sent: string[] = [];
  const notes: Array<{ message: string; level: string }> = [];
  const pending = new Set<{ callback: () => void; ms: number }>();
  const timers: DeliveryTimers = {
    set: (callback, ms) => {
      const handle = { callback, ms };
      pending.add(handle);
      return handle;
    },
    clear: (handle) => pending.delete(handle as { callback: () => void; ms: number }),
  };
  const ctx = {
    idle: false,
    queued: false,
    isIdle: () => ctx.idle,
    hasPendingMessages: () => ctx.queued,
    ui: { notify: (message: string, level: string) => notes.push({ message, level }) },
  };
  const delivery = new CompletionDelivery((text) => sent.push(text), { holdWarnMs: 1000, flushPollMs: 5 }, timers);
  // Runs the timers currently due at this interval; a timer they arm waits for the next step.
  const step = (ms: number) => {
    for (const handle of [...pending]) {
      if (handle.ms !== ms) continue;
      pending.delete(handle);
      handle.callback();
    }
  };
  return { sent, notes, pending, ctx, delivery, step, flush: () => step(5), watch: () => step(1000) };
}

test("no compaction: messages go straight out, streaming or not, with no timer or note", () => {
  const h = harness();
  h.delivery.submit(1, "one");
  h.ctx.idle = true;
  h.delivery.submit(2, "two");
  assert.deepEqual(h.sent, ["one", "two"]);
  assert.equal(h.pending.size, 0);
  assert.deepEqual(h.notes, []);
});

test("manual compaction success: held in arrival order, nothing sent before the guard clears, each submitted once", () => {
  const h = harness();
  h.delivery.begin(h.ctx);
  h.delivery.submit(2, "two");
  h.delivery.submit(1, "one");
  h.delivery.submit(3, "three");
  assert.deepEqual(h.sent, []);
  assert.equal(h.notes.filter((n) => n.level === "warning" && /held until compaction ends/.test(n.message)).length, 3);

  h.delivery.end(h.ctx);
  assert.deepEqual(h.sent, [], "the session_compact hook runs with Pi's guard still set: never send inside it");
  h.flush();
  assert.deepEqual(h.sent, [], "not idle: another session_compact handler may still hold the guard");
  h.ctx.idle = true;
  h.flush();
  assert.deepEqual(h.sent, ["two"], "one message per poll");
  h.ctx.idle = false;
  h.flush();
  h.flush();
  assert.deepEqual(h.sent, ["two", "one", "three"], "the first run is active, so the rest queue as follow-ups");
  assert.equal(h.notes.filter((n) => /once\. Pi gives no acknowledgement/.test(n.message)).length, 3);
  h.flush();
  h.watch();
  assert.equal(h.sent.length, 3, "no retry");
  assert.equal(h.pending.size, 0, "no timer left");
});

test("failure or abort uses the same path, and a message finishing during the drain keeps its place", () => {
  const h = harness();
  h.delivery.begin(h.ctx);
  h.delivery.submit(1, "one");
  h.delivery.submit(2, "two");
  h.ctx.idle = true;
  h.delivery.end(h.ctx);
  h.flush();
  h.delivery.submit(3, "three");
  assert.deepEqual(h.sent, ["one"], "must not overtake the held message");
  h.flush();
  h.flush();
  assert.deepEqual(h.sent, ["one", "two", "three"]);
  h.delivery.submit(4, "four");
  assert.deepEqual(h.sent.at(-1), "four", "drained: back to direct sends");
});

test("a second manual compaction during the drain stops it until that one ends", () => {
  const h = harness();
  h.delivery.begin(h.ctx);
  h.delivery.submit(1, "one");
  h.delivery.submit(2, "two");
  h.ctx.idle = true;
  h.delivery.end(h.ctx);
  h.flush();
  h.delivery.begin(h.ctx);
  h.flush();
  assert.deepEqual(h.sent, ["one"]);
  h.delivery.end(h.ctx);
  h.flush();
  assert.deepEqual(h.sent, ["one", "two"]);
});

test("end without a manual hold (automatic compaction) does nothing", () => {
  const h = harness();
  h.delivery.end(h.ctx);
  assert.equal(h.pending.size, 0);
  h.delivery.submit(1, "one");
  assert.deepEqual(h.sent, ["one"]);
});

test("a long hold warns once and keeps holding while Pi is busy", () => {
  const h = harness();
  h.delivery.begin(h.ctx);
  h.delivery.submit(1, "one");
  h.watch();
  h.watch();
  assert.deepEqual(h.sent, []);
  assert.equal(h.notes.filter((n) => /still held/.test(n.message)).length, 1);
});

test("a missed end hook recovers once Pi reports idle, because idle rules compaction out", () => {
  const h = harness();
  h.delivery.begin(h.ctx);
  h.delivery.submit(1, "one");
  h.ctx.idle = true;
  h.watch();
  h.flush();
  assert.deepEqual(h.sent, ["one"]);
});

test("shutdown clears timers, returns unsubmitted IDs, and later completions are not held", () => {
  const h = harness();
  h.delivery.begin(h.ctx);
  h.delivery.submit(1, "one");
  h.delivery.submit(2, "two");
  assert.deepEqual(h.delivery.shutdown(), [1, 2]);
  assert.equal(h.pending.size, 0);
  assert.deepEqual(h.sent, []);
  h.delivery.submit(3, "three");
  assert.deepEqual(h.sent, ["three"]);
});

test("a send that throws is reported as an error and not retried, and the rest still go out", () => {
  const notes: Array<{ message: string; level: string }> = [];
  const sent: string[] = [];
  const handles: Array<() => void> = [];
  const ctx = { isIdle: () => true, hasPendingMessages: () => false, ui: { notify: (message: string, level: string) => notes.push({ message, level }) } };
  const delivery = new CompletionDelivery((text) => {
    if (text === "one") throw new Error("boom");
    sent.push(text);
  }, { holdWarnMs: 1000, flushPollMs: 5 }, { set: (callback, ms) => { if (ms === 5) handles.push(callback); return callback; }, clear: () => {} });
  delivery.begin(ctx);
  delivery.submit(1, "one");
  delivery.submit(2, "two");
  delivery.end(ctx);
  while (handles.length) handles.shift()!();
  assert.deepEqual(sent, ["two"]);
  assert.equal(notes.filter((n) => n.level === "error" && /#1: boom\. It is not retried/.test(n.message)).length, 1);
});

test("nothing held but a follow-up stranded in Pi's queue: warn, do not resend", () => {
  const h = harness();
  h.delivery.begin(h.ctx);
  h.ctx.idle = true;
  h.ctx.queued = true;
  h.delivery.end(h.ctx);
  h.flush();
  assert.deepEqual(h.sent, []);
  assert.equal(h.notes.filter((n) => /still has a queued message/.test(n.message)).length, 1);
});

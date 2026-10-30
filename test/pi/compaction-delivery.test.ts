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
  assert.deepEqual(h.sent, ["two"], "busy with no known run: the guard may be set again, so every step rechecks");
  h.delivery.agentStarted();
  h.flush();
  h.flush();
  assert.deepEqual(h.sent, ["two", "one", "three"], "the first run is active, so the rest queue as follow-ups");
  h.flush();
  assert.equal(h.notes.filter((n) => /once each: #2, #1, #3\. Pi gives no acknowledgement/.test(n.message)).length, 1, "one summary, not one warning per task");
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

test("before any compaction hook: busy with no known run holds, and the later hook takes over without a duplicate", () => {
  const h = harness();
  h.delivery.attach(h.ctx);
  h.ctx.idle = false;
  h.delivery.submit(1, "one");
  assert.deepEqual(h.sent, []);
  h.flush();
  assert.deepEqual(h.sent, [], "an earlier extension's hook may be holding with Pi's guard set");
  h.delivery.begin(h.ctx);
  h.delivery.submit(2, "two");
  h.ctx.idle = true;
  h.delivery.end(h.ctx);
  h.flush();
  h.flush();
  assert.deepEqual(h.sent, ["one", "two"]);
});

test("a known agent run sends at once with no timer, so streaming follow-ups are not delayed", () => {
  const h = harness();
  h.delivery.attach(h.ctx);
  h.delivery.agentStarted();
  h.ctx.idle = false;
  h.delivery.submit(1, "one");
  assert.deepEqual(h.sent, ["one"]);
  assert.equal(h.pending.size, 0);
  assert.deepEqual(h.notes, []);
});

test("run starting before agent_start: held only until the run is known", () => {
  const h = harness();
  h.delivery.attach(h.ctx);
  h.delivery.submit(1, "one");
  assert.deepEqual(h.sent, []);
  h.delivery.agentStarted();
  h.flush();
  assert.deepEqual(h.sent, ["one"]);
});

test("after agent_settled, busy means compaction may be active again", () => {
  const h = harness();
  h.delivery.attach(h.ctx);
  h.delivery.agentStarted();
  h.delivery.agentSettled();
  h.delivery.submit(1, "one");
  assert.deepEqual(h.sent, []);
  h.ctx.idle = true;
  h.flush();
  assert.deepEqual(h.sent, ["one"]);
});

test("a stale context never sends a held message and does not throw from the timer", () => {
  const h = harness();
  h.delivery.begin(h.ctx);
  h.delivery.submit(1, "one");
  h.delivery.end(h.ctx);
  h.ctx.isIdle = () => {
    throw new Error("stale");
  };
  h.flush();
  assert.deepEqual(h.sent, []);
  assert.deepEqual(h.delivery.shutdown(), [1]);
});

test("the stranded check survives a slow later session_compact handler", () => {
  const h = harness();
  h.delivery.begin(h.ctx);
  h.ctx.queued = true;
  h.delivery.end(h.ctx);
  h.flush();
  assert.equal(h.notes.length, 0, "the guard is still set: not decided yet");
  h.ctx.idle = true;
  h.flush();
  assert.equal(h.notes.filter((n) => /still has a queued message/.test(n.message)).length, 1);
  assert.equal(h.pending.size, 0);
});

test("a run that starts after compaction cancels the stranded check", () => {
  const h = harness();
  h.delivery.begin(h.ctx);
  h.ctx.queued = true;
  h.delivery.end(h.ctx);
  h.delivery.agentStarted();
  h.flush();
  assert.deepEqual(h.notes, []);
  assert.equal(h.pending.size, 0);
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

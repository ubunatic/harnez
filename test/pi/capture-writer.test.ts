import { test } from "node:test";
import assert from "node:assert/strict";
import { CaptureWriter, type CaptureIO, type CaptureResult } from "../../.pi/lib/background-task-capture.ts";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
async function turn() { for (let i = 0; i < 8; i++) await Promise.resolve(); }
function fixture(overrides: Partial<CaptureIO<number>> = {}) {
  const timers = new Map<number, () => void>();
  let id = 0;
  const calls: string[] = [];
  const settled: CaptureResult[] = [];
  const writes: number[] = [];
  const io: CaptureIO<number> = {
    open: async () => 1,
    write: async (_h, data) => { writes.push(...data); return data.length; },
    close: async () => { calls.push("close"); },
    ...overrides,
  };
  const writer = new CaptureWriter(io, { set: (fn) => { timers.set(++id, fn); return id; }, clear: (handle) => { timers.delete(handle as number); } },
    (fn) => queueMicrotask(fn), { low: 2, high: 4, hard: 8, stallMs: 10 },
    { pause: () => calls.push("pause"), resume: () => calls.push("resume"), settled: (result) => settled.push(result) });
  const fire = () => {
    const entry = timers.entries().next().value;
    assert.ok(entry);
    timers.delete(entry[0]);
    entry[1]();
  };
  return { writer, timers, calls, settled, writes, fire };
}
function balanced(result: CaptureResult) {
  assert.equal(result.accepted, result.committed + result.queued + result.inflight + result.dropped);
}

test("serial partial writes preserve interleaved raw bytes and backpressure", async () => {
  const pending = deferred<number>();
  const out: number[] = [];
  let calls = 0;
  const f = fixture({ write: async (_h, data) => {
    if (calls++ === 0) { const n = await pending.promise; out.push(...data.subarray(0, n)); return n; }
    out.push(...data); return data.length;
  } });
  await turn();
  f.writer.append(Uint8Array.from([0xff, 0, 1, 2]));
  await turn();
  f.writer.append(Uint8Array.from([0xf0, 0x9f, 0x80]));
  assert.deepEqual(f.calls, ["pause"]);
  const done = f.writer.finish();
  assert.strictEqual(f.writer.finish(), done);
  pending.resolve(2);
  const result = await done;
  assert.deepEqual(out, [0xff, 0, 1, 2, 0xf0, 0x9f, 0x80]);
  assert.deepEqual(f.calls, ["pause", "resume", "close"]);
  assert.equal(result.complete, true);
  balanced(result);
  assert.equal(f.timers.size, 0);
});

test("hard ceiling drops pending bytes and zero writes are incomplete", async () => {
  const pending = deferred<number>();
  const f = fixture({ write: () => pending.promise });
  await turn();
  f.writer.append(Uint8Array.of(1, 2, 3, 4));
  await turn();
  f.writer.append(Uint8Array.of(5, 6, 7, 8, 9));
  pending.resolve(4);
  const result = await f.writer.finish();
  assert.equal(result.complete, false);
  assert.equal(result.committed, 4);
  assert.equal(result.dropped, 5);
  balanced(result);
  const zero = fixture({ write: async () => 0 });
  zero.writer.append(Uint8Array.of(1));
  const z = await zero.writer.finish();
  assert.match(z.reason!, /zero/);
  balanced(z);
});

test("stalled write settles synchronously, releases no descriptor until late success or rejection", async () => {
  for (const reject of [false, true]) {
    const pending = deferred<number>();
    const f = fixture({ write: () => pending.promise });
    await turn();
    f.writer.append(Uint8Array.of(1, 2, 3, 4));
    await turn();
    f.fire();
    assert.equal(f.settled.length, 1);
    assert.deepEqual(f.calls, ["pause", "resume"]);
    const result = await f.writer.finish();
    assert.equal(result.dropped, 4);
    balanced(result);
    if (reject) pending.reject(new Error("late")); else pending.resolve(4);
    await turn();
    assert.deepEqual(f.calls, ["pause", "resume", "close"]);
    assert.equal(f.settled.length, 1);
    assert.equal(f.timers.size, 0);
  }
});

test("hung open, late open and hung close settle without double close", async () => {
  const open = deferred<number>();
  const a = fixture({ open: () => open.promise });
  a.writer.append(Uint8Array.of(1));
  a.fire();
  assert.equal((await a.writer.finish()).dropped, 1);
  open.resolve(9);
  await turn();
  assert.deepEqual(a.calls, ["close"]);
  const close = deferred<void>();
  const b = fixture({ close: () => { b.calls.push("close"); return close.promise; } });
  await turn();
  const done = b.writer.finish();
  await turn();
  b.fire();
  assert.equal((await done).complete, false);
  close.resolve();
  await turn();
  assert.deepEqual(b.calls, ["close"]);
  assert.equal(b.timers.size, 0);
});

test("open, write and close failures are incomplete; truncate flushes accepted bytes", async () => {
  for (const io of [
    { open: async () => { throw Error("open"); } },
    { write: async () => { throw Error("write"); } },
    { close: async () => { throw Error("close"); } },
  ]) {
    const f = fixture(io);
    await turn();
    if (f.settled.length === 0) f.writer.append(Uint8Array.of(1));
    const result = await f.writer.finish();
    assert.equal(result.complete, false);
    balanced(result);
    assert.equal(f.timers.size, 0);
  }
  const f = fixture();
  f.writer.append(Uint8Array.of(1, 2));
  f.writer.truncate("drain timeout");
  const result = await f.writer.finish();
  assert.deepEqual(f.writes, [1, 2]);
  assert.equal(result.reason, "drain timeout");
  balanced(result);
});

test("abort is idempotent and detach suppresses callbacks", async () => {
  const f = fixture();
  f.writer.detach();
  f.writer.append(Uint8Array.of(1, 2, 3, 4));
  const first = f.writer.abort("stop");
  assert.deepEqual(f.writer.abort("again"), first);
  assert.equal(f.settled.length, 0);
  balanced(first);
  await turn();
  assert.deepEqual(f.calls, ["close"]);
  assert.equal(f.timers.size, 0);
});

test("callback throw aborts without escaping or leaking timers", async () => {
  const timers = new Map<number, () => void>();
  const writer = new CaptureWriter<number>({ open: async () => 1, write: async (_h, b) => b.length, close: async () => {} },
    { set: (fn) => { timers.set(1, fn); return 1; }, clear: () => { timers.delete(1); } }, queueMicrotask,
    { low: 1, high: 2, hard: 4, stallMs: 1 },
    { pause: () => { throw Error("callback"); }, resume: () => {}, settled: () => {} });
  writer.append(Uint8Array.of(1, 2));
  const result = await writer.finish();
  assert.match(result.reason!, /callback/);
  balanced(result);
  await turn();
  assert.equal(timers.size, 0);
});

test("Buffer input is copied, and truncation flushes accepted bytes even with an empty reason", async () => {
  const open = deferred<number>();
  const f = fixture({ open: () => open.promise });
  const bytes = Buffer.from([0xff, 1, 2]);
  f.writer.append(bytes);
  bytes.fill(0);
  f.writer.truncate("");
  assert.throws(() => f.writer.append(Uint8Array.of(3)));
  open.resolve(1);
  const result = await f.writer.finish();
  assert.deepEqual(f.writes, [0xff, 1, 2]);
  assert.equal(result.complete, false);
  assert.match(result.reason!, /truncated/);
  assert.throws(() => f.writer.truncate("late"));
  assert.deepEqual(await f.writer.cleanup(), { pending: false, errors: [] });
});

test("detach resumes a paused producer before suppressing callbacks", async () => {
  const pending = deferred<number>();
  const f = fixture({ write: () => pending.promise });
  await turn();
  f.writer.append(Uint8Array.of(1, 2, 3, 4));
  await turn();
  f.writer.detach();
  assert.deepEqual(f.calls, ["pause", "resume"]);
  pending.resolve(4);
  assert.equal((await f.writer.finish()).complete, true);
  assert.equal(f.settled.length, 0);
  assert.deepEqual(await f.writer.cleanup(), { pending: false, errors: [] });
});

test("cleanup gate exposes residual operations and late failures", async () => {
  const pending = deferred<number>();
  const f = fixture({ write: () => pending.promise, close: () => Promise.reject(undefined) });
  await turn();
  f.writer.append(Uint8Array.of(1, 2, 3, 4));
  await turn();
  const result = f.writer.abort("cancel");
  assert.equal(result.uncertain, 4);
  assert.equal(result.confirmedDropped, 0);
  assert.equal(f.writer.cleanupStatus().pending, true);
  pending.reject(undefined);
  const cleanup = await f.writer.cleanup();
  assert.equal(cleanup.pending, false);
  assert.match(cleanup.errors[0], /late write failed: undefined/);
  assert.match(cleanup.errors[1], /close failed: undefined/);
  assert.equal(f.timers.size, 0);
});

test("undefined close rejection and synchronous I/O throws fail capture", async () => {
  const cases: Partial<CaptureIO<number>>[] = [
    { open: () => { throw undefined; } },
    { write: () => { throw undefined; } },
    { close: () => { throw undefined; } },
    { close: () => Promise.reject(undefined) },
  ];
  for (const io of cases) {
    const f = fixture(io);
    await turn();
    if (!f.settled.length) f.writer.append(Uint8Array.of(1));
    const result = await f.writer.finish();
    assert.equal(result.complete, false);
    assert.match(result.reason!, /failed: undefined/);
    balanced(result);
    assert.equal((await f.writer.cleanup()).pending, false);
    assert.equal(f.timers.size, 0);
  }
});

test("invalid write counts fail, and independent writers settle separately", async () => {
  for (const count of [-1, 0, 2, 0.5, NaN]) {
    const f = fixture({ write: async () => count });
    f.writer.append(Uint8Array.of(1));
    const result = await f.writer.finish();
    assert.equal(result.complete, false);
    assert.match(result.reason!, /invalid or zero/);
    assert.equal(result.uncertain, 1);
    assert.equal(result.confirmedDropped, 0);
    balanced(result);
    await f.writer.cleanup();
  }
  const pending = deferred<number>();
  const slow = fixture({ open: () => pending.promise });
  const fast = fixture();
  fast.writer.append(Uint8Array.of(7));
  assert.equal((await fast.writer.finish()).complete, true);
  assert.equal(slow.writer.cleanupStatus().pending, true);
  slow.writer.abort("stop");
  pending.reject(new Error("late open"));
  assert.match((await slow.writer.cleanup()).errors[0], /late open/);
});

test("a rejected write may have persisted a prefix and is classified as uncertain", async () => {
  const persisted: number[] = [];
  const f = fixture({ write: async (_handle, bytes) => {
    persisted.push(...bytes.subarray(0, 2));
    throw new Error("write failed after prefix");
  } });
  f.writer.append(Uint8Array.of(1, 2, 3, 4));
  const result = await f.writer.finish();
  assert.deepEqual(persisted, [1, 2]);
  assert.equal(result.complete, false);
  assert.equal(result.committed, 0);
  assert.equal(result.uncertain, 4);
  assert.equal(result.confirmedDropped, 0);
  balanced(result);
  assert.equal((await f.writer.cleanup()).pending, false);
});

test("late close rejection is reported after bounded settlement", async () => {
  const close = deferred<void>();
  const f = fixture({ close: () => close.promise });
  await turn();
  const done = f.writer.finish();
  await turn();
  f.fire();
  assert.equal((await done).complete, false);
  assert.equal(f.writer.cleanupStatus().pending, true);
  close.reject(undefined);
  assert.match((await f.writer.cleanup()).errors[0], /close failed: undefined/);
  assert.equal(f.timers.size, 0);
});

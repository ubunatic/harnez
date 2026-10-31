import { test } from "node:test";
import assert from "node:assert/strict";
import * as fs from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { CaptureStore, nodeStoreFS, type StoreFS } from "../../.pi/lib/background-task-store.ts";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
async function turn() { for (let i = 0; i < 12; i++) await Promise.resolve(); }
async function temporary(run: (root: string) => Promise<void>) {
  const root = await fs.mkdtemp(join(tmpdir(), "harnez-store-test-"));
  try { await run(root); } finally { await fs.rm(root, { recursive: true, force: true }); }
}

test("private exclusive captures contain exact bytes and clean up independently", async () => temporary(async (root) => {
  const a = new CaptureStore({ tmpdir: root, concurrency: 1 });
  const b = new CaptureStore({ tmpdir: root, concurrency: 1 });
  const one = await a.create();
  const two = await b.create();
  assert.equal((await fs.stat(dirname(one.path))).mode & 0o777, 0o700);
  assert.equal((await fs.stat(one.path)).mode & 0o777, 0o600);
  assert.equal(await one.write(Uint8Array.of(0xff, 0, 42)), 3);
  await one.close();
  assert.deepEqual(await fs.readFile(one.path), Buffer.from([0xff, 0, 42]));
  await one.remove();
  assert.equal((await fs.stat(two.path)).isFile(), true);
  await two.remove();
  assert.deepEqual(await fs.readdir(root), []);
}));

test("replacement file and directory symlinks are refused without touching their targets", async () => temporary(async (root) => {
  const store = new CaptureStore({ tmpdir: root, concurrency: 1 });
  const target = join(root, "target");
  await fs.writeFile(target, "safe");
  const file = await store.create();
  await file.close();
  await fs.unlink(file.path);
  await fs.symlink(target, file.path);
  await assert.rejects(file.remove(), /replaced/);
  assert.equal(await fs.readFile(target, "utf8"), "safe");
  assert.ok(store.status().orphans.includes(dirname(file.path)));
  const other = await store.create();
  await other.close();
  const old = `${dirname(other.path)}-old`;
  await fs.rename(dirname(other.path), old);
  await fs.symlink(root, dirname(other.path));
  await assert.rejects(other.remove(), /replaced/);
  assert.equal(await fs.readFile(target, "utf8"), "safe");
}));

test("shutdown rejects queued opens and cleans a late open without waiting for the limiter", async () => temporary(async (root) => {
  const gate = deferred<void>();
  const entered = deferred<void>();
  const io: StoreFS = { ...nodeStoreFS, mkdtemp: async (prefix) => { entered.resolve(); await gate.promise; return nodeStoreFS.mkdtemp(prefix); } };
  const store = new CaptureStore({ tmpdir: root, concurrency: 1, io });
  const first = store.create();
  await entered.promise;
  const second = store.create();
  const secondResult = assert.rejects(second, /shut down/);
  const status = store.shutdown();
  assert.equal(status.pending, 1);
  await secondResult;
  await assert.rejects(store.create(), /shut down/);
  const firstResult = assert.rejects(first, /shut down/);
  gate.resolve();
  await firstResult;
  await turn();
  assert.deepEqual(await fs.readdir(root), []);
  assert.equal(store.status().active, 0);
}));

test("shutdown waits to close an in-flight write and reports an active lease", async () => temporary(async (root) => {
  const gate = deferred<void>();
  const entered = deferred<void>();
  let closes = 0;
  const io: StoreFS = { ...nodeStoreFS, open: async (...args) => {
    const h = await nodeStoreFS.open(...args);
    return { stat: () => h.stat(), write: async (data) => { entered.resolve(); await gate.promise; return h.write(data); }, close: async () => { closes++; await h.close(); } };
  } };
  const store = new CaptureStore({ tmpdir: root, concurrency: 1, io });
  const file = await store.create();
  const write = file.write(Uint8Array.of(7));
  await entered.promise;
  assert.equal(store.shutdown().active, 1);
  assert.equal(closes, 0);
  const removed = file.remove();
  gate.resolve();
  await write;
  await removed;
  assert.equal(closes, 1);
  assert.deepEqual(await fs.readdir(root), []);
}));

test("creation stat and close failures retain and report unidentified paths", async () => temporary(async (root) => {
  let closed = 0;
  const io: StoreFS = { ...nodeStoreFS, open: async (...args) => {
    const h = await nodeStoreFS.open(...args);
    return { stat: async () => { throw Error("stat failed"); }, write: (data) => h.write(data), close: async () => { closed++; await h.close(); } };
  } };
  const store = new CaptureStore({ tmpdir: root, concurrency: 1, io });
  await assert.rejects(store.create(), /stat failed/);
  assert.equal(closed, 1);
  assert.equal(store.status().orphans.length, 1);
  assert.match(store.status().errors[0], /identity unavailable/);
}));

test("close failure retains file and reports residual path", async () => temporary(async (root) => {
  const io: StoreFS = { ...nodeStoreFS, open: async (...args) => {
    const h = await nodeStoreFS.open(...args);
    return { stat: () => h.stat(), write: (data) => h.write(data), close: async () => { await h.close(); throw Error("close failed"); } };
  } };
  const store = new CaptureStore({ tmpdir: root, concurrency: 1, io });
  const file = await store.create();
  await assert.rejects(file.remove(), /close failed/);
  assert.equal((await fs.stat(file.path)).isFile(), true);
  assert.ok(store.status().orphans.includes(file.path));
}));

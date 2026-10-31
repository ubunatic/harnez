import { constants } from "node:fs";
import * as fs from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

// Identity checks are best effort: same-user path replacement between check and unlink is not atomic.
export interface StoreStat { dev: number | bigint; ino: number | bigint; isDirectory(): boolean; isFile(): boolean; isSymbolicLink(): boolean }
export interface StoreHandle {
  stat(): Promise<StoreStat>;
  write(data: Uint8Array): Promise<{ bytesWritten: number }>;
  close(): Promise<void>;
}
export interface StoreFS {
  mkdtemp(prefix: string): Promise<string>;
  lstat(path: string): Promise<StoreStat>;
  open(path: string, flags: number, mode: number): Promise<StoreHandle>;
  unlink(path: string): Promise<void>;
  rmdir(path: string): Promise<void>;
}
export const nodeStoreFS: StoreFS = {
  mkdtemp: fs.mkdtemp, lstat: fs.lstat,
  open: (path, flags, mode) => fs.open(path, flags, mode),
  unlink: fs.unlink, rmdir: fs.rmdir,
};
type Identity = { dev: number | bigint; ino: number | bigint };
const same = (a: Identity, b: Identity) => a.dev === b.dev && a.ino === b.ino;
const identity = (s: StoreStat): Identity => ({ dev: s.dev, ino: s.ino });
const missing = (e: unknown) => (e as NodeJS.ErrnoException)?.code === "ENOENT";

export interface StoreStatus { pending: number; active: number; orphans: string[]; errors: string[] }
export interface CaptureFile {
  readonly path: string;
  write(data: Uint8Array): Promise<number>;
  close(): Promise<void>;
  remove(): Promise<void>;
}

/** A store owns only paths it created. No crash recovery or foreign-directory sweep is attempted. */
export class CaptureStore {
  private readonly io: StoreFS;
  private readonly root: string;
  private readonly limit: number;
  private queue: Array<{ resolve: (file: CaptureFile) => void; reject: (error: unknown) => void }> = [];
  private running = 0;
  private stopped = false;
  private leases = new Set<Lease>();
  private orphanPaths = new Set<string>();
  private errors: string[] = [];

  constructor(options: { io?: StoreFS; tmpdir?: string; concurrency: number }) {
    if (!Number.isSafeInteger(options.concurrency) || options.concurrency < 1) throw new Error("invalid store concurrency");
    this.io = options.io ?? nodeStoreFS;
    this.root = options.tmpdir ?? tmpdir();
    this.limit = options.concurrency;
  }
  create(): Promise<CaptureFile> {
    if (this.stopped) return Promise.reject(new Error("capture store shut down"));
    return new Promise((resolve, reject) => { this.queue.push({ resolve, reject }); this.pump(); });
  }
  /** Nonblocking: active creates and open files remain visible until their operations settle. */
  shutdown(): StoreStatus {
    this.stopped = true;
    for (const item of this.queue.splice(0)) item.reject(new Error("capture store shut down"));
    for (const lease of this.leases) void lease.remove().catch(() => {});
    return this.status();
  }
  status(): StoreStatus {
    return { pending: this.running + this.queue.length, active: this.leases.size,
      orphans: [...this.orphanPaths], errors: [...this.errors] };
  }
  private pump(): void {
    while (!this.stopped && this.running < this.limit && this.queue.length) {
      const item = this.queue.shift()!;
      this.running++;
      void this.createOne().then(async (lease) => {
        this.leases.add(lease);
        if (this.stopped) {
          try { await lease.close(); await lease.remove(); } catch { /* recorded by lease */ }
          item.reject(new Error("capture store shut down"));
        } else item.resolve(lease);
      }, item.reject).finally(() => { this.running--; this.pump(); });
    }
  }
  private record(path: string, error: unknown): void {
    this.orphanPaths.add(path);
    this.errors.push(`${path}: ${String(error)}`);
  }
  private async createOne(): Promise<Lease> {
    let dir: string | undefined;
    let dirId: Identity | undefined;
    let file: string | undefined;
    let handle: StoreHandle | undefined;
    let fileId: Identity | undefined;
    try {
      dir = await this.io.mkdtemp(join(this.root, "harnez-capture-"));
      const ds = await this.io.lstat(dir);
      if (!ds.isDirectory() || ds.isSymbolicLink()) throw new Error("capture directory identity invalid");
      dirId = identity(ds);
      file = join(dir, "output");
      handle = await this.io.open(file, constants.O_CREAT | constants.O_EXCL | constants.O_WRONLY | (constants.O_NOFOLLOW ?? 0), 0o600);
      const stat = await handle.stat();
      if (!stat.isFile()) throw new Error("capture file identity invalid");
      fileId = identity(stat);
      return new Lease(this.io, dir, dirId, file, fileId, handle,
        (lease) => this.leases.delete(lease), (path, e) => this.record(path, e));
    } catch (error) {
      if (handle) try { await handle.close(); } catch (e) { if (file) this.record(file, e); }
      if (dir) {
        if (!dirId || (file && handle && !fileId)) this.record(dir, new Error("identity unavailable; retained"));
        else {
          try { await removeOwned(this.io, dir, dirId, file, fileId); }
          catch (e) { this.record(dir, e); }
        }
      }
      throw error;
    }
  }
}

async function removeOwned(io: StoreFS, dir: string, dirId: Identity, file?: string, fileId?: Identity): Promise<void> {
  let ds: StoreStat;
  try { ds = await io.lstat(dir); } catch (e) { if (missing(e)) return; throw e; }
  if (!ds.isDirectory() || ds.isSymbolicLink() || !same(ds, dirId)) throw new Error("capture directory replaced");
  if (file && fileId) {
    try {
      const stat = await io.lstat(file);
      if (!stat.isFile() || stat.isSymbolicLink() || !same(stat, fileId)) throw new Error("capture file replaced");
      await io.unlink(file);
    } catch (e) { if (!missing(e)) throw e; }
  }
  const final = await io.lstat(dir);
  if (!final.isDirectory() || final.isSymbolicLink() || !same(final, dirId)) throw new Error("capture directory replaced");
  await io.rmdir(dir);
}

class Lease implements CaptureFile {
  readonly path: string;
  private readonly io: StoreFS;
  private readonly dir: string;
  private readonly dirId: Identity;
  private readonly fileId: Identity;
  private readonly handle: StoreHandle;
  private readonly done: (lease: Lease) => void;
  private readonly report: (path: string, error: unknown) => void;
  private writes = new Set<Promise<unknown>>();
  private closePromise?: Promise<void>;
  private removePromise?: Promise<void>;
  constructor(io: StoreFS, dir: string, dirId: Identity, path: string, fileId: Identity, handle: StoreHandle,
    done: (lease: Lease) => void, report: (path: string, error: unknown) => void) {
    this.io = io; this.dir = dir; this.dirId = dirId; this.path = path; this.fileId = fileId;
    this.handle = handle; this.done = done; this.report = report;
  }
  write(data: Uint8Array): Promise<number> {
    if (this.closePromise || this.removePromise) return Promise.reject(new Error("capture file closing"));
    const work = Promise.resolve().then(() => this.handle.write(data)).then((r) => r.bytesWritten);
    this.writes.add(work);
    void work.then(() => this.writes.delete(work), () => this.writes.delete(work));
    return work;
  }
  close(): Promise<void> {
    if (!this.closePromise) this.closePromise = (async () => {
      await Promise.allSettled([...this.writes]);
      try { await this.handle.close(); } catch (e) { this.report(this.path, e); throw e; }
    })();
    return this.closePromise;
  }
  remove(): Promise<void> {
    if (!this.removePromise) this.removePromise = (async () => {
      try { await this.close(); await removeOwned(this.io, this.dir, this.dirId, this.path, this.fileId); }
      catch (e) { this.report(this.dir, e); throw e; }
      finally { this.done(this); }
    })();
    return this.removePromise;
  }
}

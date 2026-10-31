// Standalone capture state machine. The caller supplies all I/O, timing and scheduling.
export interface CaptureConfig { low: number; high: number; hard: number; stallMs: number }
export interface CaptureIO<H> {
  open(): Promise<H>;
  write(handle: H, data: Uint8Array): Promise<number>;
  close(handle: H): Promise<void>;
}
export interface CaptureTimers {
  set(fn: () => void, ms: number): unknown;
  clear(timer: unknown): void;
}
export interface CaptureResult {
  complete: boolean; reason?: string;
  accepted: number; committed: number; queued: number; inflight: number; dropped: number;
  confirmedDropped: number; uncertain: number;
}
export interface CaptureCleanup { pending: boolean; errors: string[] }
export interface CaptureCallbacks {
  pause(): void;
  resume(): void;
  settled(result: CaptureResult): void;
}

export class CaptureWriter<H> {
  private queue: Uint8Array[] = [];
  private queued = 0;
  private inflight = 0;
  private accepted = 0;
  private committed = 0;
  private dropped = 0;
  private uncertain = 0;
  private handle?: H;
  private opening = true;
  private writing = false;
  private closing = false;
  private closed = false;
  private ending = false;
  private settled = false;
  private detached = false;
  private paused = false;
  private scheduled = false;
  private reason?: string;
  private timer?: unknown;
  private readonly resultPromise: Promise<CaptureResult>;
  private resolve!: (result: CaptureResult) => void;
  private readonly cleanupPromise: Promise<CaptureCleanup>;
  private resolveCleanup!: (result: CaptureCleanup) => void;
  private cleanupDone = false;
  private cleanupErrors: string[] = [];
  private readonly io: CaptureIO<H>;
  private readonly timers: CaptureTimers;
  private readonly schedule: (fn: () => void) => void;
  private readonly config: CaptureConfig;
  private readonly callbacks: CaptureCallbacks;

  constructor(io: CaptureIO<H>, timers: CaptureTimers,
    schedule: (fn: () => void) => void, config: CaptureConfig,
    callbacks: CaptureCallbacks) {
    if (!Number.isSafeInteger(config.low) || !Number.isSafeInteger(config.high) || !Number.isSafeInteger(config.hard) ||
        !Number.isSafeInteger(config.stallMs) || config.low < 0 || config.low >= config.high ||
        config.high > config.hard || config.stallMs <= 0) throw new Error("invalid capture bounds");
    this.io = io;
    this.timers = timers;
    this.schedule = schedule;
    this.config = config;
    this.callbacks = callbacks;
    this.resultPromise = new Promise((resolve) => { this.resolve = resolve; });
    this.cleanupPromise = new Promise((resolve) => { this.resolveCleanup = resolve; });
    this.arm();
    try { Promise.resolve(io.open()).then((h) => this.opened(h), (e) => this.openFailed(e)); }
    catch (e) { this.openFailed(e); }
  }

  /** Input is copied because producers may reuse their buffers. A hard-limit overflow fails the capture. */
  append(data: Uint8Array): void {
    if (this.ending || this.settled) throw new Error("capture is ending");
    this.accepted += data.byteLength;
    if (this.reason !== undefined || this.queued + this.inflight + data.byteLength > this.config.hard) {
      this.dropped += data.byteLength;
      this.fail("capture queue limit exceeded");
      return;
    }
    if (data.byteLength) { this.queue.push(Uint8Array.from(data)); this.queued += data.byteLength; }
    this.flow();
    this.pumpSoon();
  }

  /** Marks unobserved stream loss, while flushing all bytes already accepted. */
  truncate(reason: string): void {
    if (this.settled) throw new Error("capture is settled");
    this.reason ??= reason || "capture truncated";
    this.ending = true;
    this.pumpSoon();
  }

  finish(): Promise<CaptureResult> {
    if (!this.settled) { this.ending = true; this.pumpSoon(); }
    return this.resultPromise;
  }

  /** Settles synchronously; an in-flight operation retains ownership until its callback arrives. */
  abort(reason: string): CaptureResult {
    if (!this.settled) {
      this.reason ??= reason || "capture aborted";
      this.dropQueued();
      this.dropped += this.inflight;
      this.uncertain += this.inflight;
      this.inflight = 0;
      this.resume();
      this.settle();
      if (!this.opening && !this.writing) this.close();
    }
    return this.snapshot();
  }

  detach(): void { this.resume(); this.detached = true; }

  /** Resolves only after outstanding operations and any resulting close have completed. */
  cleanup(): Promise<CaptureCleanup> { return this.cleanupPromise; }
  cleanupStatus(): CaptureCleanup { return { pending: !this.cleanupDone, errors: [...this.cleanupErrors] }; }

  private snapshot(): CaptureResult {
    return { complete: this.settled && this.reason === undefined && this.closed, reason: this.reason,
      accepted: this.accepted, committed: this.committed, queued: this.queued, inflight: this.inflight, dropped: this.dropped,
      confirmedDropped: this.dropped - this.uncertain, uncertain: this.uncertain };
  }
  private call(fn: () => void): void {
    if (this.detached) return;
    try { fn(); } catch { this.abort("capture callback failed"); }
  }
  private flow(): void {
    const buffered = this.queued + this.inflight;
    if (!this.paused && buffered >= this.config.high) { this.paused = true; this.call(() => this.callbacks.pause()); }
    else if (this.paused && buffered <= this.config.low) this.resume();
  }
  private resume(): void { if (this.paused) { this.paused = false; this.call(() => this.callbacks.resume()); } }
  private arm(): void {
    this.disarm();
    this.timer = this.timers.set(() => { this.timer = undefined; this.abort("capture I/O stalled"); }, this.config.stallMs);
  }
  private disarm(): void { if (this.timer !== undefined) { this.timers.clear(this.timer); this.timer = undefined; } }
  private opened(handle: H): void {
    this.opening = false;
    this.handle = handle;
    if (this.settled) { this.close(); return; }
    this.disarm();
    this.pumpSoon();
  }
  private openFailed(error: unknown): void {
    this.opening = false;
    if (!this.settled) { this.fail(`capture open failed: ${String(error)}`); this.settle(); }
    else this.cleanupErrors.push(`capture open failed: ${String(error)}`);
    this.finishCleanup();
  }
  private pumpSoon(): void {
    if (this.scheduled || this.settled) return;
    this.scheduled = true;
    try { this.schedule(() => { this.scheduled = false; this.pump(); }); }
    catch { this.scheduled = false; this.abort("capture scheduler failed"); }
  }
  private pump(): void {
    if (this.settled || this.opening || this.writing || this.closing) return;
    if (this.queue.length === 0) { if (this.ending) this.close(); return; }
    const data = this.queue.shift()!;
    this.queued -= data.byteLength;
    this.inflight = data.byteLength;
    this.writing = true;
    this.arm();
    try { Promise.resolve(this.io.write(this.handle!, data)).then((n) => this.wrote(data, n), (e) => this.writeFailed(e)); }
    catch (e) { this.writeFailed(e); }
  }
  private wrote(data: Uint8Array, n: number): void {
    this.writing = false;
    if (this.settled) {
      if (!Number.isSafeInteger(n) || n <= 0 || n > data.byteLength) this.cleanupErrors.push("capture late write returned invalid count");
      this.close(); return;
    }
    this.disarm();
    if (!Number.isSafeInteger(n) || n <= 0 || n > data.byteLength) { this.writeFailed("invalid or zero write"); return; }
    this.inflight = 0;
    this.committed += n;
    if (n < data.byteLength) { const rest = data.subarray(n); this.queue.unshift(rest); this.queued += rest.byteLength; }
    this.flow();
    this.pumpSoon();
  }
  private writeFailed(error: unknown): void {
    this.writing = false;
    if (this.settled) { this.cleanupErrors.push(`capture late write failed: ${String(error)}`); this.close(); return; }
    this.disarm();
    this.dropped += this.inflight;
    this.uncertain += this.inflight;
    this.inflight = 0;
    this.fail(`capture write failed: ${String(error)}`);
    this.close();
  }
  private dropQueued(): void { this.dropped += this.queued; this.queue = []; this.queued = 0; }
  private fail(reason: string): void {
    this.reason ??= reason;
    this.dropQueued();
    this.resume();
    this.ending = true;
    if (!this.opening && !this.writing) this.close();
  }
  private close(): void {
    if (this.closing || this.closed || this.opening || this.writing || this.handle === undefined) return;
    this.closing = true;
    if (!this.settled) this.arm();
    try { Promise.resolve(this.io.close(this.handle)).then(() => this.closeDone(false), (e) => this.closeDone(true, e)); }
    catch (e) { this.closeDone(true, e); }
  }
  private closeDone(failed: boolean, error?: unknown): void {
    this.closing = false;
    this.closed = true;
    if (failed) {
      const message = `capture close failed: ${String(error)}`;
      this.cleanupErrors.push(message);
      this.reason ??= message;
    }
    if (!this.settled) this.settle();
    this.finishCleanup();
  }
  private finishCleanup(): void {
    if (this.cleanupDone || !this.settled || this.opening || this.writing || this.closing || (this.handle !== undefined && !this.closed)) return;
    this.cleanupDone = true;
    this.resolveCleanup(this.cleanupStatus());
  }
  private settle(): void {
    if (this.settled) return;
    this.settled = true;
    this.disarm();
    const result = this.snapshot();
    this.resolve(result);
    this.call(() => this.callbacks.settled(result));
  }
}

// Core of the project-local Pi background-task extension. It has no Pi or typebox imports, so
// tests can drive it with fake children, an injected clock and an injected signal function.
import { spawn as nodeSpawn, type ChildProcess, type SpawnOptions } from "node:child_process";
import { EventEmitter } from "node:events";
import { readdirSync, readFileSync } from "node:fs";
import { StringDecoder } from "node:string_decoder";

export type TaskStatus = "running" | "completed" | "failed" | "cancelled";
// "not-confirmed": SIGTERM and SIGKILL were sent within bounded waits, but the shell leader and its process group
// were not confirmed gone. The task stays "running" and is watched until its group is empty; it then settles with
// the status its real exit gives (with a follow-up), or a later cancel can retry.
export type CancelOutcome = "cancelled" | "already-ended" | "not-confirmed";
/**
 * Decides how a running task ends. Exactly one phase applies at a time:
 * - "live": no cancel is pending. The shell leader's exit settles the task at once (a natural end).
 * - "cancelling": a cancel attempt is in flight. The task settles "cancelled" only when the leader is reaped and
 *   its group is empty, whatever the leader's own exit was.
 * - "unconfirmed": an attempt could not confirm the group. The task is an ordinary running task again: it settles
 *   with its real exit (a natural end, with a follow-up) once the leader is reaped and the group is empty.
 */
export type TaskPhase = "live" | "cancelling" | "unconfirmed";

export interface BackgroundTaskConfig {
  outputLimitChars: number;
  finishedRetentionLimit: number;
  stop: { graceMs: number; killWaitMs: number; pollMs: number };
  status: StatusConfig;
}

export interface StatusConfig {
  maxCells: number; reservedCells: number; labelMaxCells: number; fallbackLabel: string;
  manualEmoji: string; defaultEmoji: string; labelEmoji: Record<string, string>;
}

export interface BackgroundTask {
  readonly label: string;
  readonly emoji: string;
  readonly id: number;
  readonly command: string;
  readonly startedAt: number;
  status: TaskStatus;
  /** Frozen when the task leaves "running"; elapsed time stops growing from here. */
  endedAt?: number;
  pid?: number;
  exitCode?: number | null;
  signal?: NodeJS.Signals | null;
  /** Combined stdout/stderr, bounded to outputLimitChars (newest text kept). */
  output: string;
  // Internal bookkeeping below; read-only for callers.
  child: ChildProcess;
  /** The shell leader has exited or closed. Its PID may be reused after reaping, so no signal is sent. */
  leaderGone: boolean;
  outputDone: boolean;
  notified: boolean;
  finishSeq: number;
  stdoutDecoder: StringDecoder;
  stderrDecoder: StringDecoder;
  drainTimer?: NodeJS.Timeout;
  /** See TaskPhase. Only meaningful while status is "running". */
  phase: TaskPhase;
  /** The cancel attempt in progress. Concurrent cancel calls share it. Cleared when the attempt concludes. */
  attempt?: Promise<CancelOutcome>;
  /** Re-checks an "unconfirmed" task's group on a timer. Armed only in that phase; cleared when the task settles. */
  watch?: NodeJS.Timeout;
  /** Shutdown released this task: its child's events are ignored from then on. */
  released: boolean;
  lastSignalError?: string;
}

export interface RunnerOptions {
  config: BackgroundTaskConfig;
  visibleWidth: (text: string) => number;
  shell: string;
  platform: NodeJS.Platform;
  now: () => number;
  spawn: (command: string, args: string[], options: SpawnOptions) => ChildProcess;
  /** Same contract as process.kill; signal 0 probes existence and throws ESRCH when nothing is there. */
  kill: (pid: number, signal: NodeJS.Signals | 0) => void;
  /**
   * Live (not zombie, not dead) members of a process group, or undefined when this platform cannot tell.
   * Needed because a zombie still answers a signal-0 group probe but runs nothing and holds no output pipe.
   */
  liveGroupMembers: (pgid: number) => number | undefined;
}

/** Width without Pi: exact for ASCII, 2 per other code point. It may overcount, so labels can shorten but never overrun a bound. */
function conservativeWidth(text: string): number {
  let width = 0;
  for (const char of text) width += char.codePointAt(0)! < 0x7f ? 1 : 2;
  return width;
}

export function defaultRunnerOptions(config: BackgroundTaskConfig): RunnerOptions {
  return {
    config,
    visibleWidth: conservativeWidth,
    shell: process.env.SHELL || "/bin/sh",
    platform: process.platform,
    now: () => Date.now(),
    spawn: (command, args, options) => nodeSpawn(command, args, options),
    kill: (pid, signal) => process.kill(pid, signal),
    liveGroupMembers: liveProcessGroupMembers,
  };
}

/**
 * Parses one /proc/<pid>/stat line into its state letter and process group. The command name (field 2) is in
 * parentheses and may contain spaces or parentheses, so the fields are read after the last ")". Returns undefined
 * for any line that does not have the expected shape: a missing or non-numeric field is never read as a value.
 */
export function parseProcStat(line: string): { state: string; group: number } | undefined {
  const close = line.lastIndexOf(")");
  if (close === -1 || line[close + 1] !== " ") return undefined;
  const [state = "", parent = "", group = ""] = line.slice(close + 2).split(" ");
  if (!/^[A-Za-z]$/.test(state) || !/^[0-9]+$/.test(parent) || !/^[0-9]+$/.test(group)) return undefined;
  return { state, group: Number(group) };
}

/**
 * Counts processes in group pgid whose state is neither zombie (Z) nor dead (X), reading procDir (/proc by default).
 * Returns undefined when liveness cannot be established: procDir is unreadable, or any entry's stat is unreadable
 * (other than vanishing between readdir and read) or malformed. An undefined result must never be read as "empty".
 * Returns undefined too on platforms without /proc (macOS, Windows, other systems).
 * The count is a snapshot; a member can exit right after it is read.
 */
export function liveProcessGroupMembers(pgid: number, procDir = "/proc"): number | undefined {
  let entries: string[];
  try {
    entries = readdirSync(procDir);
  } catch {
    return undefined;
  }
  let live = 0;
  for (const entry of entries) {
    if (!/^[1-9][0-9]*$/.test(entry)) continue;
    let text: string;
    try {
      text = readFileSync(`${procDir}/${entry}/stat`, "utf8");
    } catch (error) {
      if (errorCode(error) === "ENOENT") continue; // exited between readdir and read
      return undefined; // unreadable (EACCES, EISDIR, ...): membership is unknown
    }
    const stat = parseProcStat(text);
    if (!stat) return undefined; // malformed: membership is unknown
    if (stat.group === pgid && stat.state !== "Z" && stat.state !== "X") live++;
  }
  return live;
}

const TASK_ID_PATTERN = /^[1-9][0-9]*$/;

/** Accepts a safe positive integer or its canonical decimal string (no sign, no leading zero). */
export function parseTaskId(value: unknown): number {
  if (typeof value === "number" && Number.isSafeInteger(value) && value > 0) return value;
  if (typeof value === "string" && TASK_ID_PATTERN.test(value)) {
    const id = Number(value);
    if (Number.isSafeInteger(id)) return id;
  }
  throw new Error(`Invalid background task ID ${JSON.stringify(String(value))}: use a positive integer such as 1`);
}

export function resolveTask(runner: BackgroundTaskRunner, value: unknown): BackgroundTask {
  const id = parseTaskId(value);
  const task = runner.tasks.get(id);
  if (!task) throw new Error(`No background task #${id}`);
  return task;
}

export function parseBackgroundTaskConfig(text: string, source: string, width: (text: string) => number): BackgroundTaskConfig {
  // Whole-line comments are dropped before JSON.parse; the file is otherwise plain JSON syntax.
  const body = text.split("\n").filter((line) => !line.trimStart().startsWith("#")).join("\n");
  let value: unknown;
  try {
    value = JSON.parse(body);
  } catch (error) {
    throw new Error(`${source}: not JSON-compatible YAML: ${(error as Error).message}`);
  }
  const root = exactKeys(value, source, ["output_limit_chars", "finished_retention_limit", "stop", "status"]);
  const stop = exactKeys(root.stop, `${source}: stop`, ["grace_ms", "kill_wait_ms", "poll_ms"]);
  const status = exactKeys(root.status, `${source}: status`, ["max_cells", "reserved_cells", "label_max_cells", "fallback_label", "manual_emoji", "default_emoji", "label_emoji"]);
  const labelEmoji = status.label_emoji;
  if (typeof labelEmoji !== "object" || labelEmoji === null || Array.isArray(labelEmoji)) throw new Error(`${source}: invalid label_emoji`);
  for (const [key, value] of Object.entries(labelEmoji)) {
    if (!/^[a-z][a-z0-9_-]*$/.test(key) || typeof value !== "string" || !value || /[\p{C}]/u.test(value)) throw new Error(`${source}: invalid label_emoji`);
  }
  const fallbackLabel = status.fallback_label;
  if (typeof fallbackLabel !== "string" || !validLabel(fallbackLabel) || fallbackLabel !== fallbackLabel.normalize("NFC")) throw new Error(`${source}: invalid fallback_label`);
  const emoji = (value: unknown) => {
    if (typeof value !== "string" || !value || /[\p{C}]/u.test(value)) throw new Error(`${source}: invalid emoji`);
    return value;
  };
  const labelMaxCells = positiveInt(status.label_max_cells, `${source}: label_max_cells`);
  if (width(fallbackLabel) > labelMaxCells) throw new Error(`${source}: fallback_label exceeds label_max_cells`);
  const reservedCells = status.reserved_cells;
  if (typeof reservedCells !== "number" || !Number.isSafeInteger(reservedCells) || reservedCells < 0) throw new Error(`${source}: invalid reserved_cells`);
  return {
    status: { maxCells: positiveInt(status.max_cells, `${source}: max_cells`), reservedCells,
      labelMaxCells, fallbackLabel,
      manualEmoji: emoji(status.manual_emoji), defaultEmoji: emoji(status.default_emoji), labelEmoji: labelEmoji as Record<string, string> },
    outputLimitChars: positiveInt(root.output_limit_chars, `${source}: output_limit_chars`),
    finishedRetentionLimit: positiveInt(root.finished_retention_limit, `${source}: finished_retention_limit`),
    stop: {
      graceMs: positiveInt(stop.grace_ms, `${source}: stop.grace_ms`),
      killWaitMs: positiveInt(stop.kill_wait_ms, `${source}: stop.kill_wait_ms`),
      pollMs: positiveInt(stop.poll_ms, `${source}: stop.poll_ms`),
    },
  };
}

export function loadBackgroundTaskConfig(path: string, width: (text: string) => number): BackgroundTaskConfig {
  return parseBackgroundTaskConfig(readFileSync(path, "utf8"), path, width);
}

function exactKeys(value: unknown, label: string, keys: string[]): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) throw new Error(`${label}: expected an object`);
  const record = value as Record<string, unknown>;
  for (const key of Object.keys(record)) {
    if (!keys.includes(key)) throw new Error(`${label}: unknown key "${key}"`);
  }
  for (const key of keys) {
    if (!(key in record)) throw new Error(`${label}: missing key "${key}"`);
  }
  return record;
}

function positiveInt(value: unknown, label: string): number {
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value < 1) {
    throw new Error(`${label}: expected a positive safe integer`);
  }
  return value;
}

function errorCode(error: unknown): string | undefined {
  return typeof error === "object" && error !== null && "code" in error ? String((error as { code: unknown }).code) : undefined;
}


function validLabel(label: string): boolean {
  return /^[\p{L}\p{N}][\p{L}\p{M}\p{N}_-]*$/u.test(label);
}

export function taskLabel(label: string | undefined, command: string, config: StatusConfig, width: (s: string) => number): string {
  // Runtime guard: callers outside TypeScript or a real schema check can pass any value.
  if (label !== undefined && typeof label !== "string") return config.fallbackLabel;
  let token = label;
  if (token === undefined) {
    token = "";
    for (const part of command.trim().split(/\s+/)) {
      // Do not interpret shell quoting, escaping, expansion or operators. Splitting those could expose an assignment value.
      if (!/^[A-Za-z0-9_./:@%+,=\-]+$/.test(part)) return config.fallbackLabel;
      if (/^[A-Za-z_][A-Za-z0-9_]*=/.test(part)) continue;
      token = part;
      break;
    }
  }
  const candidate = (label === undefined ? (token.split(/[\\/]/).pop() ?? "").replace(/\.[^.]*$/, "") : token).trim().normalize("NFC");
  if (!validLabel(candidate)) return config.fallbackLabel;
  let result = "";
  for (const { segment } of new Intl.Segmenter(undefined, { granularity: "grapheme" }).segment(candidate)) {
    if (width(result + segment) > config.labelMaxCells) break;
    result += segment;
  }
  return result || config.fallbackLabel;
}

export function formatTaskStatus(tasks: Iterable<BackgroundTask>, budget: number, width: (s: string) => number): string | undefined {
  const running = [...tasks].filter((task) => task.status === "running").sort((a, b) => a.id - b.id);
  if (!running.length) return undefined;
  const count = `Tasks (${running.length})`;
  const superscripts = "⁰¹²³⁴⁵⁶⁷⁸⁹";
  const entries = running.map((task) => [...String(task.id)].map((digit) => superscripts[Number(digit)]).join("") + task.emoji + task.label);
  for (let n = entries.length; n > 0; n--) {
    const omitted = entries.length - n;
    const text = `${count}: ${entries.slice(0, n).join(", ")}${omitted ? `, +${omitted}` : ""}`;
    if (width(text) <= budget) return text;
  }
  return count;
}

export class BackgroundTaskRunner extends EventEmitter {
  readonly tasks = new Map<number, BackgroundTask>();
  shuttingDown = false;
  private nextId = 1;
  private finishSeq = 0;
  private readonly options: RunnerOptions;

  // Events: "changed" (task) on any status change, "output" (task, text) per decoded chunk,
  // "finished" (task) once per completed or failed task whose output is final.
  constructor(options: RunnerOptions) {
    super();
    this.options = options;
  }

  start(command: string, cwd: string, metadata: { label?: string; manual?: boolean } = {}): BackgroundTask {
    const cleanCommand = command.trim();
    if (!cleanCommand) throw new Error("Provide a command: /task <command>");
    if (this.shuttingDown) throw new Error("Background tasks are shutting down");

    // Resolve display metadata first: a failure here must not leave a spawned process without a task.
    const label = taskLabel(metadata.label, cleanCommand, this.options.config.status, this.options.visibleWidth);
    const emoji = metadata.manual ? this.options.config.status.manualEmoji : (Object.hasOwn(this.options.config.status.labelEmoji, label.toLowerCase()) ? this.options.config.status.labelEmoji[label.toLowerCase()] : this.options.config.status.defaultEmoji);
    const { platform } = this.options;
    const child = this.options.spawn(this.options.shell, ["-lc", cleanCommand], {
      cwd,
      env: process.env,
      detached: platform !== "win32",
      stdio: ["ignore", "pipe", "pipe"],
    });
    const task: BackgroundTask = {
      label,
      emoji,
      id: this.nextId++,
      command: cleanCommand,
      startedAt: this.options.now(),
      status: "running",
      pid: child.pid,
      output: "",
      child,
      phase: "live",
      released: false,
      leaderGone: false,
      outputDone: false,
      notified: false,
      finishSeq: 0,
      stdoutDecoder: new StringDecoder("utf8"),
      stderrDecoder: new StringDecoder("utf8"),
    };
    this.tasks.set(task.id, task);
    this.emit("changed", task);

    // One decoder per stream: a multibyte character split across chunks of the same stream decodes intact.
    child.stdout?.on("data", (chunk: Buffer) => {
      if (!task.outputDone) this.appendOutput(task, task.stdoutDecoder.write(chunk));
    });
    child.stderr?.on("data", (chunk: Buffer) => {
      if (!task.outputDone) this.appendOutput(task, task.stderrDecoder.write(chunk));
    });
    child.once("error", (error: Error) => {
      if (task.released) return;
      this.appendOutput(task, `\n${error.message}`);
      this.leaderGone(task, null, null);
    });
    child.once("exit", (code: number | null, signal: NodeJS.Signals | null) => this.leaderGone(task, code, signal));
    child.once("close", (code: number | null, signal: NodeJS.Signals | null) => {
      this.leaderGone(task, code, signal);
      this.finishOutput(task, false);
    });
    return task;
  }

  elapsedMs(task: BackgroundTask): number {
    return (task.endedAt ?? this.options.now()) - task.startedAt;
  }

  /**
   * Cancels a running task. Concurrent calls during one attempt share its promise, so one SIGTERM and one SIGKILL
   * are sent per attempt. Resolves "cancelled" only after the shell leader is reaped and its process group holds no
   * live member. "not-confirmed" keeps the task running: it is watched until its group is empty and settles with its
   * real exit (a normal follow-up), and a later cancel retries. "already-ended" means the task had ended before the
   * call, including when its group drained since the last check: that natural end is then settled here, once.
   */
  cancel(task: BackgroundTask): Promise<CancelOutcome> {
    if (task.status !== "running") return Promise.resolve("already-ended");
    if (task.attempt) return task.attempt;
    // The watcher may not have run since the group emptied, so check the actual state before cancelling.
    this.checkTermination(task);
    if (task.status !== "running") return Promise.resolve("already-ended");
    // The watcher and the attempt never run together: the attempt is the only checker while it is in flight.
    clearInterval(task.watch);
    task.watch = undefined;
    task.phase = "cancelling";
    task.attempt = this.attempt(task);
    return task.attempt;
  }

  /**
   * Cancels every running task, then releases all tasks: output is finalized, streams are destroyed, timers are
   * cleared and later events from the released children are ignored. Shutdown sends no follow-ups. Resolves with the
   * IDs of tasks that are still running because their termination was not confirmed.
   */
  async shutdown(): Promise<number[]> {
    this.shuttingDown = true;
    const running = [...this.tasks.values()].filter((task) => task.status === "running");
    await Promise.all(running.map((task) => this.cancel(task)));
    const unconfirmed = running.filter((task) => task.status === "running").map((task) => task.id);
    for (const task of this.tasks.values()) this.release(task);
    this.tasks.clear();
    return unconfirmed;
  }

  /** One termination attempt: SIGTERM, a bounded wait, then SIGKILL and a second bounded wait. Waits end early on settlement. */
  private async attempt(task: BackgroundTask): Promise<CancelOutcome> {
    const { stop } = this.options.config;
    this.signalGroup(task, "SIGTERM");
    await this.untilSettled(task, stop.graceMs);
    if (task.status === "running") {
      this.signalGroup(task, "SIGKILL");
      await this.untilSettled(task, stop.killWaitMs);
    }
    return this.concludeAttempt(task);
  }

  /** Runs when the attempt's waits end. Re-checks the state, so a settlement during the waits is reported as it happened. */
  private concludeAttempt(task: BackgroundTask): CancelOutcome {
    task.attempt = undefined;
    this.checkTermination(task);
    // While the phase is "cancelling", only a cancelled settlement can happen, so a settled task was cancelled.
    if (task.status !== "running") return "cancelled";
    task.phase = "unconfirmed";
    this.watch(task);
    return "not-confirmed";
  }

  /** Resolves when the task leaves "running" or after ms, whichever comes first. */
  private untilSettled(task: BackgroundTask, ms: number): Promise<void> {
    return new Promise((resolve) => {
      const finish = () => {
        clearTimeout(deadline);
        clearInterval(poll);
        this.off("changed", onChange);
        resolve();
      };
      const onChange = (changed: BackgroundTask) => {
        if (changed === task && task.status !== "running") finish();
      };
      const deadline = setTimeout(finish, ms);
      const poll = setInterval(() => {
        this.checkTermination(task);
        if (task.status !== "running") finish();
      }, this.options.config.stop.pollMs);
      this.on("changed", onChange);
      this.checkTermination(task);
      if (task.status !== "running") finish();
    });
  }

  /**
   * Signals the task's process group. Before the leader is reaped the group ID is safe: the leader's PID, which
   * equals the group ID, is still held. After it is reaped, the ID can be reused once the group is empty, so a
   * signal is sent only when a live member is seen right now (which keeps the ID reserved). Otherwise no signal.
   */
  private signalGroup(task: BackgroundTask, signal: NodeJS.Signals): void {
    if (task.status !== "running" || task.pid === undefined) return;
    if (this.options.platform === "win32") {
      if (task.leaderGone) return;
      this.killQuietly(task, task.pid, signal);
      return;
    }
    if (task.leaderGone && !(this.options.liveGroupMembers(task.pid) ?? 0)) return;
    this.killQuietly(task, -task.pid, signal);
  }

  private killQuietly(task: BackgroundTask, target: number, signal: NodeJS.Signals): void {
    try {
      this.options.kill(target, signal);
    } catch (error) {
      if (errorCode(error) !== "ESRCH") task.lastSignalError = `${signal}: ${(error as Error).message}`;
    }
  }

  /**
   * True when the group has no live member. Only meaningful after the leader is reaped. A zombie member answers
   * the signal-0 probe but is not live, so the probe alone cannot decide; where liveness is unknown, the probe
   * decides and an existing group counts as not gone (ambiguous means alive).
   */
  private groupGone(task: BackgroundTask): boolean {
    if (task.pid === undefined || this.options.platform === "win32") return true;
    const live = this.options.liveGroupMembers(task.pid);
    if (live !== undefined) return live === 0;
    try {
      this.options.kill(-task.pid, 0);
      return false;
    } catch (error) {
      return errorCode(error) === "ESRCH";
    }
  }

  /** Settles a running task whose leader is reaped and whose group is empty. "live" tasks settle at the leader's exit instead. */
  private checkTermination(task: BackgroundTask): void {
    if (task.status !== "running" || !task.leaderGone || task.phase === "live") return;
    if (!this.groupGone(task)) return;
    this.settle(task, task.phase === "cancelling" ? "cancelled" : this.endedStatus(task));
  }

  private endedStatus(task: BackgroundTask): TaskStatus {
    return task.exitCode === 0 ? "completed" : "failed";
  }

  private leaderGone(task: BackgroundTask, code: number | null | undefined, signal: NodeJS.Signals | null | undefined): void {
    if (task.leaderGone || task.released) return;
    task.leaderGone = true;
    task.exitCode = code ?? null;
    task.signal = signal ?? null;
    if (task.phase === "live") this.settle(task, this.endedStatus(task));
    else this.checkTermination(task);
    this.armDrainTimer(task);
  }

  /** Re-checks an unconfirmed task's group on a timer until it settles. Unref'd: it never keeps the process alive. */
  private watch(task: BackgroundTask): void {
    if (task.watch || task.status !== "running") return;
    task.watch = setInterval(() => this.checkTermination(task), this.options.config.stop.pollMs);
    task.watch.unref();
  }

  private settle(task: BackgroundTask, status: TaskStatus): void {
    if (task.status !== "running") return;
    task.status = status;
    task.endedAt = this.options.now();
    task.finishSeq = ++this.finishSeq;
    clearInterval(task.watch);
    task.watch = undefined;
    this.evictFinished();
    this.emit("changed", task);
    this.notifyFinished(task);
  }

  /** Keeps the newest finishedRetentionLimit finished tasks. Running tasks are never evicted. */
  private evictFinished(): void {
    const finished = [...this.tasks.values()]
      .filter((task) => task.status !== "running")
      .sort((a, b) => a.finishSeq - b.finishSeq);
    const excess = finished.length - this.options.config.finishedRetentionLimit;
    for (const task of finished.slice(0, Math.max(0, excess))) this.tasks.delete(task.id);
  }

  private appendOutput(task: BackgroundTask, text: string): void {
    if (!text) return;
    task.output = clipOutput(task.output + text, this.options.config.outputLimitChars);
    this.emit("output", task, text);
  }

  /** Output streams can outlive the shell (an escaped child holding the pipe); bound the wait for them. */
  private armDrainTimer(task: BackgroundTask): void {
    if (task.outputDone || task.drainTimer) return;
    task.drainTimer = setTimeout(() => this.finishOutput(task, true), this.options.config.stop.killWaitMs);
  }

  private finishOutput(task: BackgroundTask, drained: boolean): void {
    if (task.outputDone) return;
    clearTimeout(task.drainTimer);
    task.drainTimer = undefined;
    this.appendOutput(task, task.stdoutDecoder.end() + task.stderrDecoder.end());
    task.outputDone = true;
    if (drained) {
      task.child.stdout?.destroy();
      task.child.stderr?.destroy();
      this.appendOutput(task, "\n[output streams stayed open after the command exited; later output was discarded]");
    }
    this.notifyFinished(task);
  }

  /** Ends the link between a task and its child at shutdown. Events from the child are ignored after this. */
  private release(task: BackgroundTask): void {
    clearInterval(task.watch);
    task.watch = undefined;
    this.finishOutput(task, false);
    task.child.stdout?.destroy();
    task.child.stderr?.destroy();
    clearTimeout(task.drainTimer);
    task.drainTimer = undefined;
    task.released = true;
  }

  /** Follow-ups are for natural completion or failure only: never for cancellation, and never during shutdown. */
  private notifyFinished(task: BackgroundTask): void {
    if (task.notified || !task.outputDone || this.shuttingDown) return;
    if (task.status !== "completed" && task.status !== "failed") return;
    task.notified = true;
    this.emit("finished", task);
  }
}

function clipOutput(text: string, limit: number): string {
  if (text.length <= limit) return text;
  let tail = text.slice(text.length - limit);
  // Do not start the retained text with the low half of a surrogate pair.
  if (tail.charCodeAt(0) >= 0xdc00 && tail.charCodeAt(0) <= 0xdfff) tail = tail.slice(1);
  return tail;
}

import { fileURLToPath } from "node:url";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { Type } from "typebox";
import { visibleWidth } from "@earendil-works/pi-tui";
import {
  BackgroundTaskRunner,
  defaultRunnerOptions,
  formatTaskStatus,
  loadBackgroundTaskConfig,
  resolveTask,
  type BackgroundTask,
  type CancelOutcome,
  type RunnerOptions,
} from "../lib/background-task-runner.ts";

// Bounds live in spec/ (see docs/Spec.md); the extension has no fallback values of its own.
const CONFIG_PATH = fileURLToPath(new URL("../../spec/pi-background-tasks.yaml", import.meta.url));
const TaskIdParams = Type.Object({
  id: Type.Integer({ minimum: 1, description: "Background task ID from background_task, e.g. 1" }),
});

type Notify = { ui: { notify: (message: string, level: "info" | "error") => void } };
type CommandHandler = Parameters<ExtensionAPI["registerCommand"]>[1]["handler"];

/** Wires the runner into Pi. Tests pass overrides (fake spawn, clock, signal function, small bounds). */
export function installBackgroundTasks(pi: ExtensionAPI, overrides: Partial<RunnerOptions> = {}) {
  const options: RunnerOptions = {
    ...defaultRunnerOptions(loadBackgroundTaskConfig(CONFIG_PATH, visibleWidth)),
    visibleWidth,
    ...overrides,
  };
  const runner = new BackgroundTaskRunner(options);
  let setStatus: ((text: string | undefined) => void) | undefined;

  let lastStatus: string | undefined;
  let statusSet = false;
  const updateStatus = () => {
    const columns = process.stdout.columns;
    const budget = typeof columns === "number" && Number.isFinite(columns)
      ? Math.min(options.config.status.maxCells, Math.max(0, columns - options.config.status.reservedCells))
      : options.config.status.maxCells;
    const text = formatTaskStatus(runner.tasks.values(), budget, options.visibleWidth);
    if (statusSet && text === lastStatus) return;
    if (setStatus) { setStatus(text); lastStatus = text; statusSet = true; }
  };
  runner.on("changed", updateStatus);
  runner.on("finished", (task: BackgroundTask) => {
    pi.sendUserMessage(
      `Background command #${task.id} finished (${task.status}, ${describeEnd(task)}).\nCommand: ${task.command}\nOutput:\n${task.output || "(no output)"}\n\nInspect this result and report what it means. If it indicates unfinished work, continue appropriately.`,
      { deliverAs: "followUp" },
    );
  });

  pi.on("session_start", (_event, ctx) => {
    process.stdout.removeListener("resize", updateStatus);
    setStatus = (text) => ctx.ui.setStatus("background-tasks", text);
    statusSet = false;
    process.stdout.on("resize", updateStatus);
    updateStatus();
  });

  pi.on("session_shutdown", async () => {
    process.stdout.removeListener("resize", updateStatus);
    const unconfirmed = await runner.shutdown();
    updateStatus();
    setStatus = undefined;
    statusSet = false;
    if (unconfirmed.length) {
      process.stderr.write(`Background tasks not confirmed stopped at shutdown: ${unconfirmed.map((id) => `#${id}`).join(", ")}\n`);
    }
  });

  pi.registerTool({
    name: "background_task",
    label: "Background Task",
    description: "Run any shell command as a background task. The command's output is sent back to you for inspection when it exits.",
    parameters: Type.Object({
      command: Type.String({ description: "Shell command to run in the background" }),
      label: Type.Optional(Type.String({ description: "Single-word task label, e.g. build or tests" })),
    }),
    async execute(_toolCallId, params, _signal, _onUpdate, ctx) {
      const task = runner.start(params.command, ctx.cwd, { label: params.label });
      return {
        content: [{ type: "text", text: `Started background command #${task.id}: ${task.command}` }],
        details: { id: task.id, command: task.command, status: task.status },
      };
    },
  });

  pi.registerTool({
    name: "background_task_list",
    label: "Background Tasks",
    description: "List background commands, including running/completed status, elapsed time, exit code or signal, and command.",
    parameters: Type.Object({}),
    async execute() {
      const listed = [...runner.tasks.values()].map((task) => ({
        id: task.id,
        command: task.command,
        status: task.status,
        elapsedMs: runner.elapsedMs(task),
        exitCode: task.exitCode,
        signal: task.signal,
      }));
      return {
        content: [{ type: "text", text: listed.length ? listed.map((task) => statusLine(task.id, task.status, task.elapsedMs, task.exitCode, task.signal, task.command)).join("\n") : "No background tasks" }],
        details: { tasks: listed },
      };
    },
  });

  pi.registerTool({
    name: "background_task_result",
    label: "Background Task Result",
    description: "Show the status, exit code or signal, elapsed time and captured output (bounded, newest kept) of one background command by ID.",
    parameters: TaskIdParams,
    async execute(_toolCallId, params) {
      const task = resolveTask(runner, params.id);
      const elapsedMs = runner.elapsedMs(task);
      const header = statusLine(task.id, task.status, elapsedMs, task.exitCode, task.signal, task.command);
      const body = task.output || (task.status === "running" ? "Task is still running; no output yet." : "No output.");
      return {
        content: [{ type: "text", text: `${header}\n${body}` }],
        details: { id: task.id, command: task.command, status: task.status, exitCode: task.exitCode, signal: task.signal, elapsedMs, output: task.output },
      };
    },
  });

  pi.registerTool({
    name: "background_task_cancel",
    label: "Cancel Background Task",
    description: "Stop a running background command by ID: SIGTERM to its process group, then SIGKILL after a bounded grace. Returns an error, and keeps the task running, when termination cannot be confirmed.",
    parameters: TaskIdParams,
    async execute(_toolCallId, params) {
      const task = resolveTask(runner, params.id);
      const outcome = await runner.cancel(task);
      if (outcome === "not-confirmed") throw new Error(notConfirmedMessage(task));
      return { content: [{ type: "text", text: cancelMessage(task, outcome) }], details: { id: task.id, outcome, status: task.status, signal: task.signal } };
    },
  });

  const startCommand: CommandHandler = async (command, ctx) => {
    const task = runner.start(command, ctx.cwd, { manual: true });
    ctx.ui.notify(`Started background command #${task.id}: ${task.command}`, "info");
  };
  pi.registerCommand("task", {
    description: "Run any shell command in the background: /task <command>",
    handler: startCommand,
  });

  const listTasks: CommandHandler = async (_args, ctx) => {
    const rows = [...runner.tasks.values()].map((task) => statusLine(task.id, task.status, runner.elapsedMs(task), task.exitCode, task.signal, task.command));
    ctx.ui.notify(rows.length ? rows.join("\n") : "No background tasks", "info");
  };
  pi.registerCommand("tasks", {
    description: "List background commands and their status",
    handler: listTasks,
  });

  const showResult: CommandHandler = async (args, ctx) => {
    await withTaskOrNotify(runner, args, ctx, (task) => {
      const result = task.output || (task.status === "running" ? "Task is still running." : "No output.");
      ctx.ui.notify(`${statusLine(task.id, task.status, runner.elapsedMs(task), task.exitCode, task.signal, "")}\n${result}`, task.status === "failed" ? "error" : "info");
    });
  };
  pi.registerCommand("task-result", {
    description: "Show captured output: /task-result <id>",
    handler: showResult,
  });

  const cancelCommand: CommandHandler = async (args, ctx) => {
    await withTaskOrNotify(runner, args, ctx, async (task) => {
      const outcome = await runner.cancel(task);
      if (outcome === "not-confirmed") ctx.ui.notify(notConfirmedMessage(task), "error");
      else ctx.ui.notify(cancelMessage(task, outcome), "info");
    });
  };
  pi.registerCommand("task-cancel", {
    description: "Stop a background command: /task-cancel <id>",
    handler: cancelCommand,
  });

  return { runner };
}

// Pi loads this file from the project and the global location into one runtime. Both copies share that runtime's
// event bus, so the first copy to install answers the probe and later copies yield. Pi delivers bus events
// synchronously and by reference, so the probe's mutation is visible to the emitter before emit returns.
const CLAIM_CHANNEL = "harnez:background-tasks:claim";
type ClaimProbe = { claimed: boolean };

export default function (pi: ExtensionAPI) {
  const bus: Partial<ExtensionAPI["events"]> | undefined = pi.events;
  if (typeof bus?.emit !== "function" || typeof bus?.on !== "function") {
    // Without the bus there is no way to see an earlier copy, so both copies register. Say so instead of claiming dedup.
    process.stderr.write("background-tasks: pi.events is missing; duplicate-load protection is off\n");
    installBackgroundTasks(pi);
    return;
  }
  const probe: ClaimProbe = { claimed: false };
  bus.emit(CLAIM_CHANNEL, probe);
  if (probe.claimed) return;
  // Install first: if it throws, no subscription is made, so a failed load leaves no claim behind.
  installBackgroundTasks(pi);
  const release = bus.on(CLAIM_CHANNEL, markClaimed);
  pi.on("session_shutdown", () => release());
}

/**
 * Marks a probe as claimed. Anything else that arrives on the channel (undefined, a primitive, a frozen or read-only
 * payload) is left alone and never throws, so the subscription survives it. A payload that refuses the write is not
 * marked, so the emitter still sees no claim and registers.
 */
function markClaimed(data: unknown): void {
  if (typeof data !== "object" || data === null) return;
  try {
    (data as ClaimProbe).claimed = true;
  } catch {
    // Read-only property or frozen object: no claim is recorded.
  }
}

async function withTaskOrNotify(runner: BackgroundTaskRunner, args: string, ctx: Notify, action: (task: BackgroundTask) => void | Promise<void>): Promise<void> {
  let task: BackgroundTask;
  try {
    task = resolveTask(runner, args.trim());
  } catch (error) {
    ctx.ui.notify((error as Error).message, "error");
    return;
  }
  await action(task);
}

function describeEnd(task: BackgroundTask): string {
  return task.signal ? `signal ${task.signal}` : `exit ${task.exitCode ?? "unknown"}`;
}

function statusLine(id: number, status: string, elapsedMs: number, exitCode: number | null | undefined, signal: NodeJS.Signals | null | undefined, command: string): string {
  // null exit with no signal means the process failed without an exit code (for example a spawn error), not "null".
  const end = signal ? ` signal ${signal}` : exitCode === undefined ? "" : ` exit ${exitCode ?? "unknown"}`;
  return `#${id} [${status}] ${Math.floor(elapsedMs / 1000)}s${end}${command ? ` — ${command}` : ""}`;
}

function cancelMessage(task: BackgroundTask, outcome: CancelOutcome): string {
  if (outcome === "cancelled") return `Cancelled background command #${task.id} (${task.signal ?? "no signal"}); its process group is confirmed gone.`;
  return `Background command #${task.id} already ${task.status}; nothing to cancel.`;
}

function notConfirmedMessage(task: BackgroundTask): string {
  const detail = task.lastSignalError ? ` Signal error: ${task.lastSignalError}.` : "";
  return `Could not confirm that background command #${task.id} stopped. It is still running: SIGTERM and then SIGKILL were sent to its process group within bounded waits.${detail} The task is watched and will report its exit when its process group is empty; retry background_task_cancel to signal again.`;
}

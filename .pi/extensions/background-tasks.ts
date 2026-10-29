import { spawn, type ChildProcess } from "node:child_process";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { Type } from "typebox";

interface BackgroundTask {
  id: number;
  command: string;
  process: ChildProcess;
  output: string;
  startedAt: number;
  status: "running" | "completed" | "failed" | "cancelled";
  exitCode?: number | null;
  completionNotified?: boolean;
}

const MAX_OUTPUT = 16_000;
const TaskParams = Type.Object({
  command: Type.String({ description: "Shell command to run in the background" }),
});

export default function (pi: ExtensionAPI) {
  const tasks = new Map<number, BackgroundTask>();
  let nextId = 1;
  let shuttingDown = false;
  let setStatus: ((text: string | undefined) => void) | undefined;

  const updateStatus = () => {
    const running = [...tasks.values()].filter((task) => task.status === "running").length;
    setStatus?.(running ? `Tasks: ${running}` : undefined);
  };

  const notifyCompletion = (task: BackgroundTask) => {
    if (shuttingDown || task.status === "cancelled" || task.completionNotified) return;
    task.completionNotified = true;
    const output = task.output || "(no output)";
    pi.sendUserMessage(
      `Background command #${task.id} finished (${task.status}, exit ${task.exitCode ?? "unknown"}).\nCommand: ${task.command}\nOutput:\n${output}\n\nInspect this result and report what it means. If it indicates unfinished work, continue appropriately.`,
      { deliverAs: "followUp" },
    );
  };

  const startTask = (command: string, cwd: string): BackgroundTask => {
    const cleanCommand = command.trim();
    if (!cleanCommand) throw new Error("Provide a command: /task <command>");

    const child = spawn(process.env.SHELL || "/bin/sh", ["-lc", cleanCommand], {
      cwd,
      env: process.env,
      detached: process.platform !== "win32",
      stdio: ["ignore", "pipe", "pipe"],
    });
    const task: BackgroundTask = {
      id: nextId++,
      command: cleanCommand,
      process: child,
      output: "",
      startedAt: Date.now(),
      status: "running",
    };
    tasks.set(task.id, task);
    updateStatus();

    const append = (chunk: Buffer) => {
      task.output = (task.output + chunk.toString("utf8")).slice(-MAX_OUTPUT);
    };
    child.stdout?.on("data", append);
    child.stderr?.on("data", append);
    child.once("error", (error) => {
      task.output = `${task.output}\n${error.message}`.slice(-MAX_OUTPUT);
      task.status = "failed";
      updateStatus();
      notifyCompletion(task);
    });
    child.once("close", (code) => {
      task.exitCode = code;
      if (task.status === "running") task.status = code === 0 ? "completed" : "failed";
      updateStatus();
      notifyCompletion(task);
    });
    return task;
  };

  const stopTask = (task: BackgroundTask) => {
    if (task.process.pid && process.platform !== "win32") {
      try {
        process.kill(-task.process.pid, "SIGTERM");
      } catch {
        task.process.kill("SIGTERM");
      }
    } else {
      task.process.kill("SIGTERM");
    }
  };

  pi.on("session_start", (_event, ctx) => {
    setStatus = (text) => ctx.ui.setStatus("background-tasks", text);
    updateStatus();
  });

  pi.on("session_shutdown", () => {
    shuttingDown = true;
    for (const task of tasks.values()) {
      if (task.status === "running") stopTask(task);
    }
    tasks.clear();
    updateStatus();
    setStatus = undefined;
  });

  pi.registerTool({
    name: "background_task",
    label: "Background Task",
    description: "Run any shell command as a background task. The command's output is sent back to you for inspection when it exits.",
    parameters: TaskParams,
    async execute(_toolCallId, params, _signal, _onUpdate, ctx) {
      const task = startTask(params.command, ctx.cwd);
      return {
        content: [{ type: "text", text: `Started background command #${task.id}: ${task.command}` }],
        details: { id: task.id, command: task.command, status: task.status },
      };
    },
  });

  pi.registerTool({
    name: "background_task_list",
    label: "Background Tasks",
    description: "List background commands, including running/completed status, elapsed time, exit code, and command.",
    parameters: Type.Object({}),
    async execute() {
      const listed = [...tasks.values()].map((task) => ({
        id: task.id,
        command: task.command,
        status: task.status,
        elapsedSeconds: Math.floor((Date.now() - task.startedAt) / 1000),
        exitCode: task.exitCode,
      }));
      return {
        content: [{ type: "text", text: listed.length ? listed.map((task) => `#${task.id} [${task.status}] ${task.elapsedSeconds}s${task.exitCode === undefined ? "" : ` exit ${task.exitCode}`} — ${task.command}`).join("\n") : "No background tasks" }],
        details: { tasks: listed },
      };
    },
  });

  const startCommand: Parameters<ExtensionAPI["registerCommand"]>[1]["handler"] = async (command, ctx) => {
    const task = startTask(command, ctx.cwd);
    ctx.ui.notify(`Started background command #${task.id}: ${task.command}`, "info");
  };
  pi.registerCommand("task", {
    description: "Run any shell command in the background: /task <command>",
    handler: startCommand,
  });

  const listTasks: Parameters<ExtensionAPI["registerCommand"]>[1]["handler"] = async (_args, ctx) => {
    const rows = [...tasks.values()].map((task) => {
      const elapsed = Math.floor((Date.now() - task.startedAt) / 1000);
      return `#${task.id} [${task.status}] ${elapsed}s — ${task.command}`;
    });
    ctx.ui.notify(rows.length ? rows.join("\n") : "No background tasks", "info");
  };
  pi.registerCommand("tasks", {
    description: "List background commands and their status",
    handler: listTasks,
  });

  const showResult: Parameters<ExtensionAPI["registerCommand"]>[1]["handler"] = async (args, ctx) => {
    const task = tasks.get(Number(args.trim()));
    if (!task) {
      ctx.ui.notify("Unknown background task ID", "error");
      return;
    }
    const result = task.output || (task.status === "running" ? "Task is still running." : "No output.");
    ctx.ui.notify(`#${task.id} [${task.status}]${task.exitCode === undefined ? "" : ` (exit ${task.exitCode})`}\n${result}`, task.status === "failed" ? "error" : "info");
  };
  pi.registerCommand("task-result", {
    description: "Show captured output: /task-result <id>",
    handler: showResult,
  });

  const cancelTask: Parameters<ExtensionAPI["registerCommand"]>[1]["handler"] = async (args, ctx) => {
    const task = tasks.get(Number(args.trim()));
    if (!task || task.status !== "running") {
      ctx.ui.notify("No running background task with that ID", "error");
      return;
    }
    task.status = "cancelled";
    stopTask(task);
    updateStatus();
    ctx.ui.notify(`Cancelled background command #${task.id}`, "info");
  };
  pi.registerCommand("task-cancel", {
    description: "Stop a background command: /task-cancel <id>",
    handler: cancelTask,
  });
}

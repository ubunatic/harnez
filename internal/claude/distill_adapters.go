package claude

import (
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/fsutil"
)

const piDistillAdapter = `// Managed by harnez. Keep rewrite policy in 'harnez distill hook'.
import { spawnSync } from "node:child_process";
import { appendFileSync } from "node:fs";
import { readFileSync, unlinkSync, rmdirSync } from "node:fs";
import { dirname } from "node:path";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

const AUTOPIPE_ENV = "HARNEZ_DISTILL_AUTOPIPE";
const CANARY_LOG_ENV = "HARNEZ_DISTILL_CANARY_LOG";

function autopipeEnabled(): boolean {
  const value = process.env[AUTOPIPE_ENV];
  return value === "1" || value?.toLowerCase() === "true";
}

function recordCanaryRewrite(agent: string, original: string, updated: string): void {
  const logPath = process.env[CANARY_LOG_ENV];
  if (!logPath || original === updated) return;

  appendFileSync(logPath, JSON.stringify({
    agent,
    original,
    updated,
    autopipe: process.env[AUTOPIPE_ENV],
  }) + "\n");
}

function rewriteWithHarnez(agent: string, command: string): string {
  if (!autopipeEnabled()) return command;

  const payload = JSON.stringify({
    tool_name: "Bash",
    tool_input: { command },
  });
  const result = spawnSync("harnez", ["distill", "hook"], {
    input: payload + "\n",
    encoding: "utf8",
    env: process.env,
  });

  if (result.error || result.status !== 0 || result.stdout.trim() === "") return command;

  try {
    const decoded = JSON.parse(result.stdout);
    const updated = decoded?.hookSpecificOutput?.updatedInput?.command;
    if (typeof updated === "string" && updated !== "") {
      recordCanaryRewrite(agent, command, updated);
      return updated;
    }
    return command;
  } catch {
    return command;
  }
}

export default function harnezDistill(pi: ExtensionAPI) {
  pi.on("tool_call", async (event) => {
    if (event.toolName !== "bash") return;
    if (typeof event.input?.command !== "string") return;

    event.input.command = rewriteWithHarnez("pi", event.input.command);
  });
  pi.on("tool_result", async (event) => {
    if (process.env.HARNEZ_READ_AUTOPIPE !== "1" || event.toolName !== "read" || event.isError) return;
    if (!event.content.every((item) => item.type === "text")) return;
    const text = event.content.map((item) => item.type === "text" ? item.text : "").join("\n");
    const result = spawnSync("harnez", ["distill", "read"], {
      input: JSON.stringify({ text, path: event.input?.path || "read.txt", start_line: event.input?.offset || 1,
        provider: process.env.HARNEZ_READ_PROVIDER || "unknown", vision: process.env.HARNEZ_READ_VISION === "1" }),
      encoding: "utf8", timeout: 10000, maxBuffer: 16 * 1024 * 1024,
    });
    if (result.error || result.status !== 0) return;
    let decoded;
    try { decoded = JSON.parse(result.stdout); } catch { return; }
    const content: any[] = [];
    try {
      if (decoded.text) content.push({ type: "text", text: decoded.text });
      for (const path of decoded.images || []) content.push({ type: "image", mimeType: "image/png", data: readFileSync(path).toString("base64") });
      return { content };
    } catch { return; }
    finally {
      for (const path of decoded.images || []) { try { unlinkSync(path); } catch {} }
      if (decoded.images?.length) { try { rmdirSync(dirname(decoded.images[0])); } catch {} }
    }
  });
}
`

const openCodeDistillAdapter = `// Managed by harnez. Keep rewrite policy in 'harnez distill hook'.
import { spawnSync } from "node:child_process";
import { appendFileSync } from "node:fs";
import type { Plugin } from "@opencode-ai/plugin";

const AUTOPIPE_ENV = "HARNEZ_DISTILL_AUTOPIPE";
const CANARY_LOG_ENV = "HARNEZ_DISTILL_CANARY_LOG";

function autopipeEnabled(): boolean {
  const value = process.env[AUTOPIPE_ENV];
  return value === "1" || value?.toLowerCase() === "true";
}

function recordCanaryRewrite(agent: string, original: string, updated: string): void {
  const logPath = process.env[CANARY_LOG_ENV];
  if (!logPath || original === updated) return;

  appendFileSync(logPath, JSON.stringify({
    agent,
    original,
    updated,
    autopipe: process.env[AUTOPIPE_ENV],
  }) + "\n");
}

function rewriteWithHarnez(agent: string, command: string): string {
  if (!autopipeEnabled()) return command;

  const payload = JSON.stringify({
    tool_name: "Bash",
    tool_input: { command },
  });
  const result = spawnSync("harnez", ["distill", "hook"], {
    input: payload + "\n",
    encoding: "utf8",
    env: process.env,
  });

  if (result.error || result.status !== 0 || result.stdout.trim() === "") return command;

  try {
    const decoded = JSON.parse(result.stdout);
    const updated = decoded?.hookSpecificOutput?.updatedInput?.command;
    if (typeof updated === "string" && updated !== "") {
      recordCanaryRewrite(agent, command, updated);
      return updated;
    }
    return command;
  } catch {
    return command;
  }
}

export const HarnezDistillPlugin: Plugin = async () => ({
  "tool.execute.before": async (input, output) => {
    if (input.tool !== "bash") return;
    if (typeof output.args?.command !== "string") return;

    output.args.command = rewriteWithHarnez("opencode", output.args.command);
  },
  "tool.execute.after": async (input, output) => {
    if (process.env.HARNEZ_READ_AUTOPIPE !== "1" || input.tool !== "read" || typeof output.output !== "string") return;
    const result = spawnSync("harnez", ["distill", "read"], {
      input: JSON.stringify({ text: output.output, path: input.args?.filePath || "read.txt", start_line: input.args?.offset || 1,
        provider: "unknown", vision: false }),
      encoding: "utf8", timeout: 10000, maxBuffer: 16 * 1024 * 1024,
    });
    if (result.error || result.status !== 0) return;
    try { const decoded = JSON.parse(result.stdout); if (typeof decoded.text === "string") output.output = decoded.text; } catch {}
  },
});
`

type generatedAdapter struct {
	label   string
	path    string
	content string
}

func distillAdapters(cfg *Config) []generatedAdapter {
	if cfg == nil {
		return nil
	}
	var adapters []generatedAdapter
	if target := fsutil.ExpandHome(cfg.DistillAutopipe.PiExtensionTarget); target != "" {
		adapters = append(adapters, generatedAdapter{
			label:   "pi distill adapter",
			path:    target,
			content: piDistillAdapter,
		})
	}
	if target := fsutil.ExpandHome(cfg.DistillAutopipe.OpenCodePluginTarget); target != "" {
		adapters = append(adapters, generatedAdapter{
			label:   "opencode distill adapter",
			path:    target,
			content: openCodeDistillAdapter,
		})
	}
	return adapters
}

func distillAdapterSummary(cfg *Config) string {
	var labels []string
	for _, adapter := range distillAdapters(cfg) {
		labels = append(labels, adapter.label+"="+fsutil.ContractHome(filepath.Clean(adapter.path)))
	}
	return strings.Join(labels, ", ")
}

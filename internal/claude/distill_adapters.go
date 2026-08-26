package claude

import (
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/fsutil"
)

const piDistillAdapter = `// Managed by harnez. Keep rewrite policy in 'harnez distill hook'.
import { spawnSync } from "node:child_process";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

const AUTOPIPE_ENV = "HARNEZ_DISTILL_AUTOPIPE";

function autopipeEnabled(): boolean {
  const value = process.env[AUTOPIPE_ENV];
  return value === "1" || value?.toLowerCase() === "true";
}

function rewriteWithHarnez(command: string): string {
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
    return typeof updated === "string" && updated !== "" ? updated : command;
  } catch {
    return command;
  }
}

export default function harnezDistill(pi: ExtensionAPI) {
  pi.on("tool_call", async (event) => {
    if (event.toolName !== "bash") return;
    if (typeof event.input?.command !== "string") return;

    event.input.command = rewriteWithHarnez(event.input.command);
  });
}
`

const openCodeDistillAdapter = `// Managed by harnez. Keep rewrite policy in 'harnez distill hook'.
import { spawnSync } from "node:child_process";
import type { Plugin } from "@opencode-ai/plugin";

const AUTOPIPE_ENV = "HARNEZ_DISTILL_AUTOPIPE";

function autopipeEnabled(): boolean {
  const value = process.env[AUTOPIPE_ENV];
  return value === "1" || value?.toLowerCase() === "true";
}

function rewriteWithHarnez(command: string): string {
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
    return typeof updated === "string" && updated !== "" ? updated : command;
  } catch {
    return command;
  }
}

export const HarnezDistillPlugin: Plugin = async () => ({
  "tool.execute.before": async (input, output) => {
    if (input.tool !== "bash") return;
    if (typeof output.args?.command !== "string") return;

    output.args.command = rewriteWithHarnez(output.args.command);
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

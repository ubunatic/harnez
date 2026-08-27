package claude

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDistillAdaptersTargets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := &Config{DistillAutopipe: DistillAutopipe{
		PiExtensionTarget:    "~/.pi/agent/extensions/harnez-distill.ts",
		OpenCodePluginTarget: "~/.config/opencode/plugins/harnez-distill.ts",
	}}

	adapters := distillAdapters(cfg)
	if len(adapters) != 2 {
		t.Fatalf("distillAdapters() returned %d adapters, want 2", len(adapters))
	}

	wantPi := filepath.Join(home, ".pi", "agent", "extensions", "harnez-distill.ts")
	wantOpenCode := filepath.Join(home, ".config", "opencode", "plugins", "harnez-distill.ts")
	if adapters[0].path != wantPi {
		t.Fatalf("Pi adapter path = %q, want %q", adapters[0].path, wantPi)
	}
	if adapters[1].path != wantOpenCode {
		t.Fatalf("OpenCode adapter path = %q, want %q", adapters[1].path, wantOpenCode)
	}
}

func TestDistillAdaptersCanBeDisabled(t *testing.T) {
	cfg := &Config{}
	if got := distillAdapters(cfg); len(got) != 0 {
		t.Fatalf("distillAdapters(empty config) returned %#v, want none", got)
	}
}

func TestPiDistillAdapterContent(t *testing.T) {
	required := []string{
		`import { spawnSync } from "node:child_process";`,
		`import { appendFileSync } from "node:fs";`,
		`import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";`,
		`HARNEZ_DISTILL_AUTOPIPE`,
		`HARNEZ_DISTILL_CANARY_LOG`,
		`spawnSync("harnez", ["distill", "hook"]`,
		`recordCanaryRewrite(agent, command, updated);`,
		`tool_name: "Bash"`,
		`pi.on("tool_call"`,
		`event.toolName !== "bash"`,
		`event.input.command = rewriteWithHarnez("pi", event.input.command);`,
	}
	for _, needle := range required {
		if !strings.Contains(piDistillAdapter, needle) {
			t.Errorf("Pi adapter missing %q\n%s", needle, piDistillAdapter)
		}
	}
	for _, forbidden := range []string{"RewriteBashCommand", "go test", "cargo test", "AGY", "Codex"} {
		if strings.Contains(piDistillAdapter, forbidden) {
			t.Errorf("Pi adapter should not duplicate policy or mention unsupported hooks; found %q", forbidden)
		}
	}
}

func TestOpenCodeDistillAdapterContent(t *testing.T) {
	required := []string{
		`import { spawnSync } from "node:child_process";`,
		`import { appendFileSync } from "node:fs";`,
		`import type { Plugin } from "@opencode-ai/plugin";`,
		`HARNEZ_DISTILL_AUTOPIPE`,
		`HARNEZ_DISTILL_CANARY_LOG`,
		`spawnSync("harnez", ["distill", "hook"]`,
		`recordCanaryRewrite(agent, command, updated);`,
		`tool_name: "Bash"`,
		`"tool.execute.before"`,
		`input.tool !== "bash"`,
		`output.args.command = rewriteWithHarnez("opencode", output.args.command);`,
	}
	for _, needle := range required {
		if !strings.Contains(openCodeDistillAdapter, needle) {
			t.Errorf("OpenCode adapter missing %q\n%s", needle, openCodeDistillAdapter)
		}
	}
	for _, forbidden := range []string{"RewriteBashCommand", "go test", "cargo test", "AGY", "Codex"} {
		if strings.Contains(openCodeDistillAdapter, forbidden) {
			t.Errorf("OpenCode adapter should not duplicate policy or mention unsupported hooks; found %q", forbidden)
		}
	}
}

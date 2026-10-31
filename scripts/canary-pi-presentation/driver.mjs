// Interactive Pi and local model stub in run.sh's isolated PID/network namespaces.
import { execFileSync, spawnSync } from "node:child_process";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";

const [home, piBin, extension] = process.argv.slice(2);
const agentDir = path.join(home, "agent");
const cwd = path.join(home, "work");
const socket = path.join(home, "tmux.sock");
const marker = "PRESENTATION_CANARY_SECRET_1046";
fs.mkdirSync(agentDir);
fs.mkdirSync(cwd);
let stage = 0;
let modelResult = false;
const server = http.createServer((req, res) => {
  let body = "";
  req.on("data", (chunk) => body += chunk);
  req.on("end", () => {
    const request = JSON.parse(body);
    const messages = request.messages ?? [];
    const text = JSON.stringify(messages);
    let tool;
    if (stage === 0) {
      stage = 1;
      const hex = Buffer.from(marker).toString("hex");
      tool = { name: "background_task", args: { command: `node -e 'process.stdout.write(Buffer.from("${hex}","hex"))'` } };
    } else if (stage === 1 && text.includes("Background command #1 finished")) {
      stage = 2;
      tool = { name: "background_task_result", args: { id: 1 } };
    } else if (stage === 2 && messages.some((message) => message.role === "tool" && JSON.stringify(message.content).includes(marker))) {
      modelResult = true;
      stage = 3;
    }
    res.writeHead(200, { "content-type": "text/event-stream" });
    const chunk = (delta, finish = null) => res.write(`data: ${JSON.stringify({ id: "c", object: "chat.completion.chunk", model: "m", choices: [{ index: 0, delta, finish_reason: finish }] })}\n\n`);
    if (tool) {
      chunk({ role: "assistant", tool_calls: [{ index: 0, id: `tool${stage}`, type: "function", function: { name: tool.name, arguments: JSON.stringify(tool.args) } }] });
      chunk({}, "tool_calls");
    } else {
      chunk({ role: "assistant", content: stage === 3 ? "MODEL_RESULT_RECEIVED" : "ok" });
      chunk({}, "stop");
    }
    res.end("data: [DONE]\n\n");
  });
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
fs.writeFileSync(path.join(agentDir, "models.json"), JSON.stringify({ providers: { stub: { baseUrl: `http://127.0.0.1:${server.address().port}/v1`, api: "openai-completions", apiKey: "local", models: [{ id: "m", contextWindow: 100000, maxTokens: 200, input: ["text"] }] } } }));
fs.writeFileSync(path.join(agentDir, "settings.json"), JSON.stringify({ compaction: { enabled: false }, retry: { enabled: false } }));
const tmux = (...args) => execFileSync("tmux", ["-S", socket, ...args], { encoding: "utf8" });
const quote = (value) => `'${value.replaceAll("'", "'\\''")}'`;
const env = { PATH: process.env.PATH, HOME: home, PI_CODING_AGENT_DIR: agentDir, PI_OFFLINE: "1", PI_SKIP_VERSION_CHECK: "1", PI_TELEMETRY: "0", TMPDIR: home, TERM: "xterm-256color" };
const command = ["env", "-i", ...Object.entries(env).map(([key, value]) => `${key}=${value}`), piBin, "--provider", "stub", "--model", "m", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-themes", "--no-context-files", "--no-mcp", "--session-dir", path.join(home, "sessions"), "-e", extension].map(quote).join(" ");
const pane = () => tmux("capture-pane", "-p", "-t", "canary:0.0", "-S", "0");
async function until(fn, label) {
  const deadline = Date.now() + 15000;
  while (Date.now() < deadline) {
    if (fn()) return;
    await new Promise((resolve) => setImmediate(resolve));
  }
  throw new Error(`timeout waiting for ${label}; pane:\n${pane()}`);
}
function assert(value, label) { if (!value) throw new Error(`${label}; pane:\n${pane()}`); console.log(`PASS ${label}`); }
try {
  tmux("new-session", "-d", "-s", "canary", "-x", "120", "-y", "45", "-c", cwd, command);
  await until(() => pane().includes("background-tasks.ts"), "interactive Pi startup");
  tmux("send-keys", "-t", "canary:0.0", "run presentation canary", "Enter");
  await until(() => modelResult && pane().includes("Output hidden (Ctrl+O to expand)") && pane().includes("MODEL_RESULT_RECEIVED"), "collapsed result and model response");
  assert(modelResult, "model received explicit result containing marker");
  assert(!pane().includes(marker), "collapsed pane hides marker");
  tmux("send-keys", "-t", "canary:0.0", "C-o");
  await until(() => pane().includes(marker), "expanded marker");
  assert(pane().includes(marker), "expanded pane shows marker");
  tmux("send-keys", "-t", "canary:0.0", "C-o");
  await until(() => pane().includes("Output hidden (Ctrl+O to expand)") && !pane().includes(marker), "collapsed marker");
  assert(!pane().includes(marker), "collapsed again hides marker");
} finally {
  spawnSync("tmux", ["-S", socket, "kill-server"], { stdio: "ignore" });
  server.closeAllConnections();
  server.close();
}

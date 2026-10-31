// Real Pi, local model stub, and repository extension; run through run.sh for network isolation.
import { spawn } from "node:child_process";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import readline from "node:readline";

const [home, piBin, extension] = process.argv.slice(2);
const agentDir = path.join(home, "agent");
const cwd = path.join(home, "work");
fs.mkdirSync(agentDir);
fs.mkdirSync(cwd);
let nextTool;
let readResult;
const completions = [];
const server = http.createServer((req, res) => {
  let body = "";
  req.on("data", (data) => body += data);
  req.on("end", () => {
    const request = JSON.parse(body);
    for (const message of request.messages ?? []) {
      const content = typeof message.content === "string" ? message.content : (message.content ?? []).map((part) => part.text ?? "").join("");
      const match = content.match(/Background command #(\d+) finished[\s\S]*/);
      if (match && !completions.some((entry) => entry.id === +match[1])) completions.push({ id: +match[1], text: match[0] });
      if (message.role === "tool" && content.includes("PAGE_MARKER")) readResult = content;
    }
    res.writeHead(200, { "content-type": "text/event-stream" });
    const chunk = (delta, finish = null) => res.write(`data: ${JSON.stringify({ id: "c", object: "chat.completion.chunk", model: "m", choices: [{ index: 0, delta, finish_reason: finish }] })}\n\n`);
    if (nextTool) {
      const tool = nextTool;
      nextTool = undefined;
      chunk({ role: "assistant", tool_calls: [{ index: 0, id: `t${Date.now()}`, type: "function", function: { name: tool.name, arguments: JSON.stringify(tool.args) } }] });
      chunk({}, "tool_calls");
    } else {
      chunk({ role: "assistant", content: "ok" });
      chunk({}, "stop");
    }
    res.end("data: [DONE]\n\n");
  });
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
fs.writeFileSync(path.join(agentDir, "models.json"), JSON.stringify({ providers: { stub: { baseUrl: `http://127.0.0.1:${server.address().port}/v1`, api: "openai-completions", apiKey: "local", models: [{ id: "m", contextWindow: 100000, maxTokens: 200, input: ["text"] }] } } }));
fs.writeFileSync(path.join(agentDir, "settings.json"), JSON.stringify({ compaction: { enabled: false }, retry: { enabled: false } }));
const pi = spawn(piBin, ["--mode", "rpc", "--provider", "stub", "--model", "m", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-themes", "--session-dir", path.join(home, "sessions"), "-e", extension], { cwd, stdio: ["pipe", "pipe", "pipe"], env: { PATH: process.env.PATH, HOME: home, PI_CODING_AGENT_DIR: agentDir, PI_OFFLINE: "1", PI_SKIP_VERSION_CHECK: "1", PI_TELEMETRY: "0", TMPDIR: home } });
let stderr = "";
let closed = false;
let ends = 0;
let id = 0;
const pending = new Map();
const errors = [];
pi.stderr.on("data", (data) => stderr += data);
pi.on("close", () => closed = true);
readline.createInterface({ input: pi.stdout }).on("line", (line) => {
  const msg = JSON.parse(line);
  if (msg.type === "response") { pending.get(msg.id)?.(msg); pending.delete(msg.id); }
  if (msg.type === "agent_end") ends++;
  if (msg.type === "extension_error") errors.push(msg.error);
  if (msg.type === "tool_execution_end" && msg.toolName === "read") readResult = JSON.stringify(msg.result);
});
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
async function until(fn, label, timeout = 15000) {
  const deadline = Date.now() + timeout;
  while (!fn()) { if (Date.now() > deadline) throw new Error(`timeout: ${label}`); await sleep(10); }
}
function send(command) { return new Promise((resolve) => { const key = String(++id); pending.set(key, resolve); pi.stdin.write(JSON.stringify({ ...command, id: key }) + "\n"); }); }
async function turn(text, tool) {
  while ((await send({ type: "get_state" })).data?.isStreaming) await sleep(10);
  const before = ends;
  nextTool = tool;
  const response = await send({ type: "prompt", message: text });
  if (!response.success) throw new Error(JSON.stringify(response));
  await until(() => ends > before, `turn ${text}`);
  while ((await send({ type: "get_state" })).data?.isStreaming) await sleep(10);
}
function assert(ok, label) { if (!ok) throw new Error(label); console.log(`PASS ${label}`); }
let capturePath;
try {
  // 20 MiB of lines plus a distinctive line for a real offset/limit read.
  const command = `node -e 'process.stdout.write("PAGE_MARKER\\n"+"0123456789abcdef\\n".repeat(1233619))'`;
  await turn("start capture", { name: "background_task", args: { command } });
  await until(() => completions.some((entry) => entry.id === 1), "capture completion", 30000);
  const text = completions.find((entry) => entry.id === 1).text;
  const match = text.match(/Capture complete: (\d+) committed bytes\. Path: ([^\n]+?)\. Deleted/);
  assert(!!match, `default capture completion: ${text.slice(0, 300)}`);
  capturePath = match[2];
  const size = fs.statSync(capturePath).size;
  assert(size === +match[1] && size >= 20 * 1024 * 1024, `reported ${match[1]} bytes, actual ${size}, path ${capturePath}`);
  await turn("read first line", { name: "read", args: { path: capturePath, offset: 1, limit: 1 } });
  await until(() => readResult !== undefined, "read tool result");
  assert(readResult?.includes("PAGE_MARKER"), `Pi read tool returned PAGE_MARKER with offset 1 and limit 1: ${readResult}`);
  await turn("tool opt out", { name: "background_task", args: { command: "printf TOOL_OPTOUT", capture: false } });
  await until(() => completions.some((entry) => entry.id === 2), "tool opt out");
  assert(/Output:\s*TOOL_OPTOUT/.test(completions.find((entry) => entry.id === 2).text), "tool capture false returns output");
  await send({ type: "prompt", message: "/task --no-capture printf SLASH_OPTOUT" });
  await until(() => completions.some((entry) => entry.id === 3), "slash opt out");
  assert(/Output:\s*SLASH_OPTOUT/.test(completions.find((entry) => entry.id === 3).text), "slash --no-capture returns output");
  await turn("drain timeout", { name: "background_task", args: { command: "setsid sh -c 'sleep 5' & printf DRAIN_PREFIX" } });
  await until(() => completions.some((entry) => entry.id === 4), "drain timeout completion");
  const incomplete = completions.find((entry) => entry.id === 4).text;
  assert(/Capture incomplete \(output drain timed out\)/.test(incomplete), `drain timeout is incomplete: ${incomplete}`);
  assert(errors.length === 0, `extension errors: ${JSON.stringify(errors)}`);
} finally {
  pi.stdin.end();
  await until(() => closed, "Pi shutdown", 10000).catch(() => { pi.kill("SIGKILL"); });
  server.closeAllConnections();
  server.close();
}
assert(closed, `Pi exited; stderr: ${stderr.slice(0, 300)}`);
assert(capturePath && !fs.existsSync(capturePath), "capture removed on session shutdown");

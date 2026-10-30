// Run: node --import ./test/pi/register.mjs --test test/pi/background-claim.test.ts
// Duplicate-load protection over Pi's event bus, the mirrored global package, and the real Pi loader. The original
// unit tests stay in background-tasks.test.ts. The real-loader tests skip with a stated reason when no Pi package is
// found (set HARNEZ_PI_PACKAGE_DIR to point at one). Every real-loader run sets HOME and PI_CODING_AGENT_DIR into a
// fresh temp home, so the real config is never read or written.
import { after, test } from "node:test";
import assert from "node:assert/strict";
import { visibleWidth as stubWidth } from "./width-stub.mjs";
import { formatTaskStatus, taskLabel } from "../../.pi/lib/background-task-runner.ts";
import { mkdirSync, mkdtempSync, copyFileSync, existsSync, readFileSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { homedir, tmpdir } from "node:os";
import { delimiter, dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import backgroundTasksExtension, { installBackgroundTasks } from "../../.pi/extensions/background-tasks.ts";

const CLAIM_CHANNEL = "harnez:background-tasks:claim";
const REPO = fileURLToPath(new URL("../../", import.meta.url));
// The tree a global package mirrors: the extension, its sibling lib, and the spec it reads relative to itself.
const SOURCE_FILES = [".pi/extensions/background-tasks.ts", ".pi/lib/background-task-runner.ts", "spec/pi-background-tasks.yaml"];
// The M2 installer must write this same manifest for the global copy (type module, extension entry point).
const MIRROR_PACKAGE_JSON = { name: "harnez-pi-background-tasks", private: true, type: "module", pi: { extensions: [".pi/extensions/background-tasks.ts"] } };
const EXPECTED_TOOLS = ["background_task", "background_task_cancel", "background_task_list", "background_task_result"];
const EXPECTED_COMMANDS = ["task", "task-cancel", "task-result", "tasks"];
const ALL_REGISTRATIONS = [...EXPECTED_TOOLS, ...EXPECTED_COMMANDS];
const tempDirs: string[] = [];

after(() => {
  for (const dir of tempDirs) rmSync(dir, { recursive: true, force: true });
});

function tempDir(prefix: string): string {
  const dir = mkdtempSync(join(tmpdir(), prefix));
  tempDirs.push(dir);
  return dir;
}

/** Copies the source tree under root, keeping the repository layout; optionally writes the package manifest. */
function mirrorTree(root: string, options: { packageJson?: boolean } = {}): string {
  for (const rel of SOURCE_FILES) {
    const target = join(root, rel);
    mkdirSync(join(target, ".."), { recursive: true });
    copyFileSync(join(REPO, rel), target);
  }
  if (options.packageJson) writeFileSync(join(root, "package.json"), JSON.stringify(MIRROR_PACKAGE_JSON, null, 2));
  return root;
}

/** Imports a mirrored extension directly. The query asks typebox-hook.mjs to stub pi-tui for this copy only. */
function importMirror(root: string) {
  return import(`${pathToFileURL(join(root, ".pi/extensions/background-tasks.ts")).href}?pi-tui-stub`);
}

type Listener = (data: unknown) => void;

/**
 * Models Pi's bus contract as the extension relies on it: emit calls listeners synchronously with the same object,
 * and on returns an unsubscribe. The real bus is checked against this model in the live canary test below. Unlike
 * the real bus, which logs a listener's error, this fake lets it propagate to the emitter.
 */
function fakeBus() {
  const listeners = new Map<string, Listener[]>();
  const emitted: string[] = [];
  return {
    emitted,
    emit(channel: string, data: unknown) {
      emitted.push(channel);
      for (const listener of [...(listeners.get(channel) ?? [])]) listener(data);
    },
    on(channel: string, handler: Listener) {
      listeners.set(channel, [...(listeners.get(channel) ?? []), handler]);
      return () => listeners.set(channel, (listeners.get(channel) ?? []).filter((listener) => listener !== handler));
    },
    /** Listeners currently held on the claim channel, including any foreign ones a test adds. */
    count: () => (listeners.get(CLAIM_CHANNEL) ?? []).length,
  };
}
type FakeBus = ReturnType<typeof fakeBus>;

/**
 * Records what a fake pi registers. failOn names a tool or command whose registration throws. The fake does not roll
 * back earlier registrations on failure, so the tests assert only the bus subscription after a failed load, never
 * that the fake pi is empty.
 */
function fakePi(options: { bus?: FakeBus; events?: unknown; failOn?: string } = {}) {
  const handlers = new Map<string, Array<(...args: unknown[]) => unknown>>();
  const tools = new Map<string, unknown>();
  const commands = new Map<string, unknown>();
  const pi = {
    events: "events" in options ? options.events : options.bus,
    on: (name: string, handler: (...args: unknown[]) => unknown) => handlers.set(name, [...(handlers.get(name) ?? []), handler]),
    registerTool: (tool: { name: string }) => {
      if (options.failOn === tool.name) throw new Error(`refused ${tool.name}`);
      tools.set(tool.name, tool);
    },
    registerCommand: (name: string, definition: unknown) => {
      if (options.failOn === name) throw new Error(`refused ${name}`);
      commands.set(name, definition);
    },
    sendUserMessage: () => {},
  };
  return { pi: pi as any, handlers, tools, commands };
}

/** Only events is reachable: proves a yielded load never registers or subscribes on its pi. */
function eventsOnlyPi(bus: FakeBus) {
  const refuse = (name: string) => () => {
    throw new Error(`pi touched: ${name}`);
  };
  return {
    events: bus,
    on: refuse("on"),
    registerTool: refuse("registerTool"),
    registerCommand: refuse("registerCommand"),
    sendUserMessage: refuse("sendUserMessage"),
  } as any;
}

async function fireShutdown(handlers: Map<string, Array<(...args: unknown[]) => unknown>>) {
  for (const handler of handlers.get("session_shutdown") ?? []) await handler({ type: "session_shutdown" }, {});
}

/** Captures process.stderr.write while body runs. */
function captureStderr(body: () => void): string {
  const original = process.stderr.write;
  let captured = "";
  process.stderr.write = ((chunk: string | Uint8Array) => {
    captured += String(chunk);
    return true;
  }) as typeof process.stderr.write;
  try {
    body();
  } finally {
    process.stderr.write = original;
  }
  return captured;
}

/** Captures console.error while body runs, for Pi's bus, which logs a listener's error instead of rethrowing it. */
function captureConsoleError(body: () => void): string[] {
  const original = console.error;
  const logged: string[] = [];
  console.error = (...args: unknown[]) => {
    logged.push(args.map(String).join(" "));
  };
  try {
    body();
  } finally {
    console.error = original;
  }
  return logged;
}

/** Reads the claim state a new copy would see: emits a fresh probe and reports whether any listener marked it. */
function isClaimed(bus: { emit(channel: string, data: unknown): void }): boolean {
  const probe = { claimed: false };
  bus.emit(CLAIM_CHANNEL, probe);
  return probe.claimed;
}

test("default export: the first copy registers 4 tools and 4 commands and subscribes once; a second copy on the same bus yields", () => {
  const bus = fakeBus();
  const first = fakePi({ bus });
  backgroundTasksExtension(first.pi);
  assert.deepEqual([...first.tools.keys()].sort(), EXPECTED_TOOLS);
  assert.deepEqual([...first.commands.keys()].sort(), EXPECTED_COMMANDS);
  assert.equal(bus.count(), 1, "the first copy holds one subscription");

  backgroundTasksExtension(eventsOnlyPi(bus));
  assert.equal(bus.count(), 1, "the yielded copy adds no subscription");
});

test("project and global copies on one bus: exactly one tool and command set", async () => {
  const bus = fakeBus();
  // Two mirrored trees imported separately: two module instances, as Pi loads two copies of the file.
  const projectRoot = mirrorTree(tempDir("bg-project-copy-"));
  const globalRoot = mirrorTree(tempDir("bg-global-copy-"), { packageJson: true });
  const projectModule = await importMirror(projectRoot);
  const globalModule = await importMirror(globalRoot);
  const projectCopy = fakePi({ bus });
  projectModule.default(projectCopy.pi);
  const globalCopy = fakePi({ bus });
  globalModule.default(globalCopy.pi);
  assert.deepEqual([...projectCopy.tools.keys()].sort(), EXPECTED_TOOLS);
  assert.deepEqual([...projectCopy.commands.keys()].sort(), EXPECTED_COMMANDS);
  assert.equal(globalCopy.tools.size, 0, "the global copy registers no tools");
  assert.equal(globalCopy.commands.size, 0, "the global copy registers no commands");
  assert.equal(bus.count(), 1);
});

test("owner shutdown removes the subscription; a reload on the same bus registers again", async () => {
  const bus = fakeBus();
  const first = fakePi({ bus });
  backgroundTasksExtension(first.pi);
  await fireShutdown(first.handlers);
  assert.equal(bus.count(), 0, "shutdown releases the subscription");

  const reloaded = fakePi({ bus });
  backgroundTasksExtension(reloaded.pi);
  assert.deepEqual([...reloaded.tools.keys()].sort(), EXPECTED_TOOLS);
  assert.deepEqual([...reloaded.commands.keys()].sort(), EXPECTED_COMMANDS);
  assert.equal(bus.count(), 1);
  await fireShutdown(reloaded.handlers);
});

for (const name of ALL_REGISTRATIONS) {
  test(`registration of ${name} refused: the load throws, leaves no subscription, and a retry on the same bus registers`, () => {
    const bus = fakeBus();
    assert.throws(() => backgroundTasksExtension(fakePi({ bus, failOn: name }).pi), { message: `refused ${name}` });
    assert.equal(bus.count(), 0, "a failed install must not subscribe");

    const retry = fakePi({ bus });
    backgroundTasksExtension(retry.pi);
    assert.deepEqual([...retry.tools.keys()].sort(), EXPECTED_TOOLS);
    assert.deepEqual([...retry.commands.keys()].sort(), EXPECTED_COMMANDS);
    assert.equal(bus.count(), 1);
  });
}

test("the direct installBackgroundTasks helper emits no probe and subscribes to nothing", () => {
  const bus = fakeBus();
  installBackgroundTasks(fakePi({ bus }).pi);
  assert.deepEqual(bus.emitted, []);
  assert.equal(bus.count(), 0);
});

test("missing event API: the load says duplicate protection is off and still registers; no subscription is claimed", () => {
  const fake = fakePi();
  const stderr = captureStderr(() => backgroundTasksExtension(fake.pi));
  assert.deepEqual([...fake.tools.keys()].sort(), EXPECTED_TOOLS);
  assert.deepEqual([...fake.commands.keys()].sort(), EXPECTED_COMMANDS);
  assert.match(stderr, /pi\.events is missing; duplicate-load protection is off/);
});

test("partial event API (emit without on): treated as missing, warns, and still registers", () => {
  const fake = fakePi({ events: { emit: () => {} } });
  const stderr = captureStderr(() => backgroundTasksExtension(fake.pi));
  assert.deepEqual([...fake.tools.keys()].sort(), EXPECTED_TOOLS);
  assert.match(stderr, /pi\.events is missing; duplicate-load protection is off/);
});

test("claim subscriber ignores undefined, primitives, frozen and read-only payloads without throwing, and stays subscribed", () => {
  const bus = fakeBus();
  const first = fakePi({ bus });
  backgroundTasksExtension(first.pi);
  const readOnly = Object.defineProperty({}, "claimed", { value: false, writable: false, enumerable: true });
  for (const payload of [undefined, null, 1, "claimed", true, Object.freeze({ claimed: false }), readOnly]) {
    assert.doesNotThrow(() => bus.emit(CLAIM_CHANNEL, payload), `payload ${String(payload)} must not throw`);
  }
  assert.equal(readOnly.claimed, false, "a read-only payload is not marked");
  assert.equal(bus.count(), 1, "the subscription survives the foreign payloads");
  assert.equal(isClaimed(bus), true, "a real probe is still claimed afterwards");
  backgroundTasksExtension(eventsOnlyPi(bus));
  assert.equal(bus.count(), 1);
});

test("a throwing foreign listener on the claim channel fails this load and adds no subscription of ours", () => {
  const bus = fakeBus();
  bus.on(CLAIM_CHANNEL, () => {
    throw new Error("foreign listener failed");
  });
  const fake = fakePi({ bus });
  assert.throws(() => backgroundTasksExtension(fake.pi), { message: "foreign listener failed" });
  assert.equal(fake.tools.size, 0, "the probe is emitted before any install, so nothing registers");
  assert.equal(bus.count(), 1, "only the foreign listener is on the channel");
});

test("boundary: the claim is not authenticated; a listener that marks every probe makes this copy yield with no tools", () => {
  const bus = fakeBus();
  bus.on(CLAIM_CHANNEL, (data) => {
    (data as { claimed: boolean }).claimed = true;
  });
  const spoofed = fakePi({ bus });
  backgroundTasksExtension(spoofed.pi);
  assert.equal(spoofed.tools.size, 0, "a spoofed claim suppresses registration; it cannot cause a duplicate");
  assert.equal(spoofed.commands.size, 0);
});

test("mirrored package: a copy of the tree registers from its sibling lib and spec and subscribes once", async () => {
  const bus = fakeBus();
  const root = mirrorTree(tempDir("bg-mirror-"), { packageJson: true });
  const mirrored = await importMirror(root);
  const fake = fakePi({ bus });
  mirrored.default(fake.pi);
  assert.deepEqual([...fake.tools.keys()].sort(), EXPECTED_TOOLS);
  assert.deepEqual([...fake.commands.keys()].sort(), EXPECTED_COMMANDS);
  assert.equal(bus.count(), 1);
  await fireShutdown(fake.handlers);
  assert.equal(bus.count(), 0);
});

test("mirrored package with an invalid sibling spec fails loudly and leaves no subscription", async () => {
  const bus = fakeBus();
  const root = mirrorTree(tempDir("bg-mirror-bad-"));
  writeFileSync(join(root, "spec/pi-background-tasks.yaml"), "{}\n");
  const mirrored = await importMirror(root);
  assert.throws(() => mirrored.default(fakePi({ bus }).pi), /missing key|unknown key|output_limit_chars/);
  assert.equal(bus.count(), 0);
});

// Real Pi loader: the installed Pi package discovers and loads extensions, so these tests prove the layouts, the bus
// probe and the trust and reload order under its own loader, resource loader and event bus.
const LOADER_REL = "dist/core/extensions/loader.js";

/** Finds the Pi 1.4.x-style package: HARNEZ_PI_PACKAGE_DIR first, else the pi launcher on PATH. Never touches user config. */
function locatePiPackage(): { dir: string } | { skip: string } {
  const override = process.env.HARNEZ_PI_PACKAGE_DIR;
  if (override) {
    return existsSync(join(override, LOADER_REL)) ? { dir: override } : { skip: `HARNEZ_PI_PACKAGE_DIR has no ${LOADER_REL}` };
  }
  const launcher = (process.env.PATH ?? "")
    .split(delimiter)
    .filter(Boolean)
    .map((dir) => join(dir, "pi"))
    .find((candidate) => existsSync(candidate));
  if (!launcher) return { skip: "no pi on PATH and HARNEZ_PI_PACKAGE_DIR is unset" };
  // The launcher resolves to <agent>/bin/pi and reads <agent>/install/current-version; this only reads the install.
  const agentDir = dirname(dirname(realpathSync(launcher)));
  const versionFile = join(agentDir, "install", "current-version");
  if (!existsSync(versionFile)) return { skip: `no ${versionFile}; set HARNEZ_PI_PACKAGE_DIR` };
  const version = readFileSync(versionFile, "utf8").trim();
  const dir = join(agentDir, "install", "releases", version, "node_modules", "@earendil-works", "pi-coding-agent");
  return existsSync(join(dir, LOADER_REL)) ? { dir } : { skip: `Pi ${version} package not found at ${dir}` };
}

const PI_LOCATION = locatePiPackage();
const SKIP = "dir" in PI_LOCATION ? false : `real-loader tests skipped: ${PI_LOCATION.skip}`;

test("real Pi cell widths agree for representative graphemes and status boundaries", { skip: SKIP }, async () => {
  const { dir } = PI_LOCATION as { dir: string };
  const { visibleWidth } = await import(pathToFileURL(join(dirname(dir), "pi-tui", "dist", "utils.js")).href);
  for (const sample of ["🇩🇪", "का", "漢字", "é", "e\u0301", "¹🔨build", "👤backup"]) {
    assert.equal(stubWidth(sample), visibleWidth(sample), `width for ${sample}`);
  }
  const status = { maxCells: 72, reservedCells: 16, labelMaxCells: 4, fallbackLabel: "task", manualEmoji: "👤", defaultEmoji: "🤖", labelEmoji: {} };
  assert.equal(taskLabel("漢字a", "", status, visibleWidth), "漢字");
  assert.equal(taskLabel("漢字a", "", { ...status, labelMaxCells: 3 }, visibleWidth), "漢");
  const tasks = [{ id: 1, label: "漢字", emoji: "🤖", status: "running" }] as any;
  const expected = "Tasks (1): ¹🤖漢字";
  const width = visibleWidth(expected);
  assert.equal(formatTaskStatus(tasks, width, visibleWidth), expected);
  assert.equal(formatTaskStatus(tasks, width - 1, visibleWidth), "Tasks (1)");
});

type PiModules = {
  home: string;
  loader: any;
  createEventBus: () => any;
  resourceLoader: any;
  settings: any;
  config: any;
};

// process.env is process-wide, so isolated runs queue behind each other even if a future test runs concurrently.
let isolatedRuns: Promise<unknown> = Promise.resolve();

/** Runs body with HOME and PI_CODING_AGENT_DIR pointing into a fresh temp home, restored afterwards. */
function withIsolatedPi<T>(body: (pi: PiModules) => Promise<T>): Promise<T> {
  const run = isolatedRuns.then(() => runIsolated(body));
  isolatedRuns = run.catch(() => undefined);
  return run;
}

async function runIsolated<T>(body: (pi: PiModules) => Promise<T>): Promise<T> {
  const { dir } = PI_LOCATION as { dir: string };
  const home = tempDir("bg-home-");
  const agentDir = join(home, ".pi", "agent");
  mkdirSync(agentDir, { recursive: true });
  const saved = { HOME: process.env.HOME, PI_CODING_AGENT_DIR: process.env.PI_CODING_AGENT_DIR };
  process.env.HOME = home;
  process.env.PI_CODING_AGENT_DIR = agentDir;
  try {
    // Pi resolves its agent dir and the OS home at call time; check both point into the temp home.
    assert.equal(homedir(), home, "homedir() must resolve to the temp home");
    const url = (rel: string) => pathToFileURL(join(dir, rel)).href;
    const loader = await import(url(LOADER_REL));
    const config = await import(url("dist/config.js"));
    assert.equal(config.getAgentDir(), agentDir, "Pi's agent dir must resolve to the temp home");
    const { createEventBus } = await import(url("dist/core/event-bus.js"));
    const resourceLoader = await import(url("dist/core/resource-loader.js"));
    const settings = await import(url("dist/core/settings-manager.js"));
    return await body({ home, loader, createEventBus, resourceLoader, settings, config });
  } finally {
    for (const [key, value] of Object.entries(saved)) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  }
}

/** A fresh cwd and agent dir for one runtime, both under the isolated home. Project copy under cwd/.pi, global under agent/extensions. */
function scenario(pi: PiModules, scope: { project: boolean; global: boolean }) {
  const root = mkdtempSync(join(pi.home, "runtime-"));
  const cwd = join(root, "project");
  const agentDir = join(root, "agent");
  mkdirSync(cwd, { recursive: true });
  mkdirSync(agentDir, { recursive: true });
  if (scope.project) mirrorTree(cwd);
  if (scope.global) mirrorTree(join(agentDir, "extensions", "harnez-background-tasks"), { packageJson: true });
  return { cwd, agentDir };
}

/** Counts every tool and command name across all loaded extensions, so a duplicate shows up as a repeated name. */
function registeredNames(result: { extensions: Array<{ tools: Map<string, unknown>; commands: Map<string, unknown> }> }) {
  const tools: string[] = [];
  const commands: string[] = [];
  for (const extension of result.extensions) {
    tools.push(...extension.tools.keys());
    commands.push(...extension.commands.keys());
  }
  return { tools: tools.sort(), commands: commands.sort() };
}

function assertOneSet(result: { errors: unknown[]; extensions: any[] }, label: string) {
  assert.deepEqual(result.errors, [], `${label}: loader errors`);
  assert.deepEqual(registeredNames(result), { tools: EXPECTED_TOOLS, commands: EXPECTED_COMMANDS }, `${label}: exactly one tool and command set`);
}

async function load(pi: PiModules, scope: { cwd: string; agentDir: string }, bus?: unknown) {
  return pi.loader.discoverAndLoadExtensions([], scope.cwd, scope.agentDir, bus);
}

async function loadAndCheckOnce(pi: PiModules, scope: { cwd: string; agentDir: string }, label: string, bus?: unknown) {
  const result = await load(pi, scope, bus);
  assertOneSet(result, label);
  return result;
}

async function shutdownAll(result: { extensions: Array<{ handlers: Map<string, Array<(...args: unknown[]) => unknown>> }> }) {
  for (const extension of result.extensions) await fireShutdown(extension.handlers);
}

/** A DefaultResourceLoader on the given bus, with the settings Pi's session builds. */
function resourceLoaderFor(pi: PiModules, scope: { cwd: string; agentDir: string }, bus: unknown) {
  const settingsManager = pi.settings.SettingsManager.create(scope.cwd, scope.agentDir);
  return new pi.resourceLoader.DefaultResourceLoader({ cwd: scope.cwd, agentDir: scope.agentDir, settingsManager, eventBus: bus });
}

/** Two-phase trust: reload runs a pre-trust pass (project untrusted), asks the callback, then the final pass. */
async function reloadWithTrust(loader: any, trusted: boolean) {
  await loader.reload({ resolveProjectTrust: async () => trusted });
  return loader.getExtensions();
}

/** Mirrors AgentSession.reload: the old runner emits session_shutdown, then its runtime is invalidated, before the loader reloads. */
async function retireExtensions(loader: any) {
  const old = loader.getExtensions();
  await shutdownAll(old);
  old.runtime.invalidate();
}

function assertNoSessionsWritten(agentDir: string) {
  assert.equal(existsSync(join(agentDir, "sessions")), false, "the isolated agent dir gets no session files");
}

test("live Pi event bus: emit is synchronous, passes the same object, and an unsubscribed handler is not called", { skip: SKIP }, async () => {
  await withIsolatedPi(async (pi) => {
    const bus = pi.createEventBus();
    const payload = { claimed: false };
    let received: unknown;
    let handlerRanBeforeEmitReturned = false;
    let emitReturned = false;
    const unsubscribe = bus.on("canary", (data: unknown) => {
      received = data;
      handlerRanBeforeEmitReturned = !emitReturned;
      (data as { claimed: boolean }).claimed = true;
    });
    bus.emit("canary", payload);
    emitReturned = true;
    assert.equal(received, payload, "the handler gets the same object, not a clone");
    assert.equal(handlerRanBeforeEmitReturned, true, "the handler runs synchronously inside emit");
    assert.equal(payload.claimed, true, "a mutation by the handler is visible to the emitter");

    unsubscribe();
    const later = { claimed: false };
    bus.emit("canary", later);
    assert.equal(later.claimed, false, "an unsubscribed handler is not called");
  });
});

test("live Pi event bus: the real claim subscriber ignores foreign payloads with no throw and no logged error", { skip: SKIP }, async () => {
  await withIsolatedPi(async (pi) => {
    const bus = pi.createEventBus();
    const loaded = await loadAndCheckOnce(pi, scenario(pi, { project: true, global: false }), "guard", bus);
    const readOnly = Object.defineProperty({}, "claimed", { value: false, writable: false, enumerable: true });
    const frozen = Object.freeze({ claimed: false });
    const logged = captureConsoleError(() => {
      for (const payload of [undefined, null, 1, "claimed", frozen, readOnly]) bus.emit(CLAIM_CHANNEL, payload);
    });
    assert.deepEqual(logged, [], "Pi's bus logs a listener error; the claim subscriber must not raise one");
    assert.equal(frozen.claimed, false);
    assert.equal(readOnly.claimed, false);
    assert.equal(isClaimed(bus), true, "the subscriber survives the foreign payloads");
    await shutdownAll(loaded);
  });
});

for (const [label, scope] of [
  ["project only", { project: true, global: false }],
  ["global only", { project: false, global: true }],
  ["project and global", { project: true, global: true }],
] as const) {
  test(`real Pi loader, ${label}: one tool set and one command set; after shutdown the same bus registers exactly once again`, { skip: SKIP }, async () => {
    await withIsolatedPi(async (pi) => {
      const runtime = scenario(pi, scope);
      const bus = pi.createEventBus();
      const first = await loadAndCheckOnce(pi, runtime, label, bus);
      await shutdownAll(first);
      const second = await loadAndCheckOnce(pi, runtime, `${label} after reload`, bus);
      await shutdownAll(second);
    });
  });
}

test("real Pi loader: invalidating a runtime without shutdown also releases its bus subscription", { skip: SKIP }, async () => {
  await withIsolatedPi(async (pi) => {
    const runtime = scenario(pi, { project: true, global: true });
    const bus = pi.createEventBus();
    const first = await loadAndCheckOnce(pi, runtime, "before invalidate", bus);
    first.runtime.invalidate();
    const second = await loadAndCheckOnce(pi, runtime, "after invalidate", bus);
    await shutdownAll(second);
  });
});

test("real Pi loader, two concurrent independent runtimes, each with project and global copies: each gets one complete set", { skip: SKIP }, async () => {
  await withIsolatedPi(async (pi) => {
    const runtimeA = scenario(pi, { project: true, global: true });
    const runtimeB = scenario(pi, { project: true, global: true });
    const [a, b] = await Promise.all([load(pi, runtimeA), load(pi, runtimeB)]);
    assertOneSet(a, "runtime A");
    assertOneSet(b, "runtime B");
    await shutdownAll(a);
    await shutdownAll(b);
  });
});

test("real Pi loader, one bus shared by two runtimes: the second yields while the first is subscribed, and registers after the first shuts down", { skip: SKIP }, async () => {
  await withIsolatedPi(async (pi) => {
    const bus = pi.createEventBus();
    const first = await loadAndCheckOnce(pi, scenario(pi, { project: true, global: false }), "first runtime", bus);
    const second = await load(pi, scenario(pi, { project: true, global: false }), bus);
    assert.deepEqual(registeredNames(second), { tools: [], commands: [] }, "the second runtime yields on the shared bus");

    await shutdownAll(first);
    const third = await loadAndCheckOnce(pi, scenario(pi, { project: true, global: false }), "third runtime", bus);
    await shutdownAll(third);
  });
});

test("real Pi loader, install refused partway: the load reports it, registers nothing, and leaves no claim on the bus", { skip: SKIP }, async () => {
  await withIsolatedPi(async (pi) => {
    const bus = pi.createEventBus();
    const root = mirrorTree(tempDir("bg-partial-"));
    // Tools register, then the first command is refused: the install throws after its tool registrations.
    writeFileSync(
      join(root, "partial-refused.ts"),
      'import background from "./.pi/extensions/background-tasks.ts";\n' +
        "export default function (pi: any) {\n" +
        '  background({ ...pi, registerCommand: () => { throw new Error("refused task"); } });\n' +
        "}\n",
    );
    const scope = scenario(pi, { project: false, global: false });
    const failed = await pi.loader.loadExtensions([join(root, "partial-refused.ts")], scope.cwd, bus);
    assert.equal(failed.extensions.length, 0, "the failed extension is not kept");
    assert.equal(failed.errors.length, 1);
    assert.match(failed.errors[0].error, /refused task/);
    assert.equal(isClaimed(bus), false, "a failed load leaves no claim on the bus");

    const clean = await pi.loader.loadExtensions([join(root, ".pi/extensions/background-tasks.ts")], scope.cwd, bus);
    assertOneSet(clean, "clean load after the failed one");
    await shutdownAll(clean);
  });
});

test("real Pi loader: a subscription made before a load throws is removed by the loader's discard", { skip: SKIP }, async () => {
  await withIsolatedPi(async (pi) => {
    const bus = pi.createEventBus();
    const dir = tempDir("bg-discard-");
    const fixture = join(dir, "subscribe-then-throw.ts");
    writeFileSync(
      fixture,
      `export default function (pi: any) {\n  pi.events.on("${CLAIM_CHANNEL}", (data: any) => { data.claimed = true; });\n  throw new Error("failed after subscribing");\n}\n`,
    );
    const failed = await pi.loader.loadExtensions([fixture], dir, bus);
    assert.equal(failed.errors.length, 1);
    assert.match(failed.errors[0].error, /failed after subscribing/);
    assert.equal(isClaimed(bus), false, "discard must unsubscribe what the failed load added");
  });
});

for (const [label, scope, expectation] of [
  ["trusted startup, project and global", { project: true, global: true }, "one"],
  ["untrusted startup, project and global", { project: true, global: true }, "one"],
  ["untrusted startup, project only", { project: true, global: false }, "none"],
] as const) {
  test(`DefaultResourceLoader two-phase trust, ${label}: ${expectation === "one" ? "one tool set and one command set, no conflict" : "the project copy is not loaded, so no tools"}`, { skip: SKIP }, async () => {
    await withIsolatedPi(async (pi) => {
      const runtime = scenario(pi, scope);
      const bus = pi.createEventBus();
      const loader = resourceLoaderFor(pi, runtime, bus);
      const trusted = label.startsWith("trusted");
      const result = await reloadWithTrust(loader, trusted);
      if (expectation === "one") {
        assertOneSet(result, label);
      } else {
        assert.deepEqual(result.errors, [], `${label}: loader errors`);
        assert.deepEqual(registeredNames(result), { tools: [], commands: [] }, "no copy loads from an untrusted project");
      }
      await retireExtensions(loader);
      assertNoSessionsWritten(runtime.agentDir);
    });
  });
}

test("DefaultResourceLoader: trust order reverses across reloads on one loader; each step has exactly one set and the bus is clean at the end", { skip: SKIP }, async () => {
  await withIsolatedPi(async (pi) => {
    const runtime = scenario(pi, { project: true, global: true });
    const bus = pi.createEventBus();
    const loader = resourceLoaderFor(pi, runtime, bus);
    assertOneSet(await reloadWithTrust(loader, true), "startup, trusted");
    await retireExtensions(loader);
    assertOneSet(await reloadWithTrust(loader, false), "reload, untrusted (global copy only)");
    await retireExtensions(loader);
    assertOneSet(await reloadWithTrust(loader, true), "reload, trusted again");
    await retireExtensions(loader);
    assert.equal(isClaimed(bus), false, "after the last retire no subscription remains on the loader's bus");
    assertNoSessionsWritten(runtime.agentDir);
  });
});

test("DefaultResourceLoader: a reload that does not retire the previous runtime yields to it, so the host must shut down first", { skip: SKIP }, async () => {
  await withIsolatedPi(async (pi) => {
    const runtime = scenario(pi, { project: true, global: true });
    const bus = pi.createEventBus();
    const loader = resourceLoaderFor(pi, runtime, bus);
    const first = await reloadWithTrust(loader, true);
    assertOneSet(first, "startup");
    const second = await reloadWithTrust(loader, true);
    assert.deepEqual(registeredNames(second), { tools: [], commands: [] }, "the first runtime's subscription still holds the claim");
    // Retire both runtimes explicitly: the second has no subscription of its own, and the first still holds one.
    await shutdownAll(first);
    first.runtime.invalidate();
    await shutdownAll(second);
    second.runtime.invalidate();
    assert.equal(isClaimed(bus), false, "retiring the first runtime releases the claim");
  });
});

test("DefaultResourceLoader: plain reload() after a trusted startup flips ownership to the project copy in the new runtime, one set", { skip: SKIP }, async () => {
  await withIsolatedPi(async (pi) => {
    const runtime = scenario(pi, { project: true, global: true });
    const bus = pi.createEventBus();
    const loader = resourceLoaderFor(pi, runtime, bus);
    const startup = await reloadWithTrust(loader, true);
    assertOneSet(startup, "startup, trusted");
    const startupOwner = startup.extensions.find((extension: any) => extension.tools.size > 0);
    assert.ok(startupOwner.resolvedPath.startsWith(join(runtime.agentDir, "extensions")), `startup owner is the global copy, got ${startupOwner.resolvedPath}`);

    // No resolveProjectTrust here: reload() keeps the persisted trust from the startup pass, so the project copy loads.
    await retireExtensions(loader);
    await loader.reload();
    const reloaded = loader.getExtensions();
    assert.notEqual(reloaded, startup, "the reload produced a new runtime");
    assertOneSet(reloaded, "plain reload after trusted startup");
    const owners = reloaded.extensions.filter((extension: any) => extension.tools.size > 0);
    assert.equal(owners.length, 1, "exactly one extension owns the tools after the reload");
    assert.notEqual(owners[0], startupOwner, "ownership flips to the extension of the new runtime");
    assert.ok(owners[0].resolvedPath.startsWith(runtime.cwd), `the project copy owns after a plain reload, got ${owners[0].resolvedPath}`);
    assert.equal(isClaimed(bus), true, "the new runtime holds the bus claim while it is live");

    await retireExtensions(loader);
    assert.equal(isClaimed(bus), false, "retiring the new runtime releases the claim");
    assertNoSessionsWritten(runtime.agentDir);
  });
});

// Canary fixture for issue 1047: holds compaction hooks and fires controlled messages.
// Logs go through ctx.ui.notify, so they share one ordered stream (RPC stdout) with Pi's own events.
export default function (pi: any) {
  let ctx: any;
  let cfg: any = {};
  const holds: Record<string, () => void> = {};
  const log = (e: string, data: any = {}) => ctx?.ui.notify("CANARY " + JSON.stringify({ e, ...data }), "info");
  const hold = (name: string, signal?: AbortSignal) =>
    new Promise<void>((resolve) => {
      holds[name] = resolve;
      signal?.addEventListener("abort", () => resolve(), { once: true });
      log("held", { name });
    });

  pi.on("session_start", (_event: any, c: any) => {
    ctx = c;
    log("session_start");
  });
  pi.on("session_before_compact", async (event: any) => {
    if (cfg.holdBefore) await hold("before", event.signal);
    if (cfg.cancelBefore) return { cancel: true };
  });
  pi.on("session_compact", async () => {
    log("session_compact");
    if (cfg.holdCompact) await hold("compact");
  });
  pi.on("session_compact_failed", (event: any) => log("session_compact_failed", { aborted: event.aborted }));
  pi.registerCommand("cfg", {
    handler: async (args: string) => {
      cfg = JSON.parse(args);
      log("cfg");
    },
  });
  pi.registerCommand("fire", {
    handler: async (args: string) => {
      pi.sendUserMessage(`FIXTURE ${args.trim()} done`, { deliverAs: "followUp" });
      log("fired");
    },
  });
  pi.registerCommand("release", {
    handler: async (args: string) => {
      holds[args.trim()]?.();
      log("released");
    },
  });
}

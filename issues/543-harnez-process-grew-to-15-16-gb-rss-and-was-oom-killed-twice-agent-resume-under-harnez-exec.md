# 543 — harnez process grew to 15-16 GB RSS and was OOM-killed twice (agent resume under harnez exec)

**Status**: Open
**Priority**: P0
**Severity**: Critical
**Category**: Bug / Memory

---

## Observed (2026-09-24, reported by the lucky-fox session, kernel log checked by the host)

```
16:24:41 Out of memory: Killed process 1221989 (harnez) total-vm:28973388kB, anon-rss:14874096kB
16:25:16 Out of memory: Killed process 1222463 (harnez) total-vm:26873428kB, anon-rss:16145788kB
         task_memcg=/user.slice/.../app.slice/vte-spawn-51c3e7d9-….scope
```

- The scope `vte-spawn-51c3e7d9` holds `zsh → harnez agent chat --model claude:opus → claude --name lucky-fox`
  (the voxi session). The host's `cpu541` developer ran in a different scope, so it is not the source.
- At that time the lucky-fox tree ran `⚙ bash -c "harnez agent resume --name sel-dev …" | tail -12`,
  i.e. `harnez exec` wrapping a streaming `harnez agent resume`, piped into `tail`.
- The killed command lines were not captured. A fresh run (pid 1222684) stayed at ~15 MB.

## /goal

Find which harnez process and code path grows without bound, fix it, and add a guard so that a
long-running `harnez exec` / `harnez agent resume` keeps its memory bounded no matter how long it runs.

## Update: caught live (lucky-fox, 2026-09-24)

`harnez read -I -L 1:400 internal/tts/manager.go internal/tts/socket.go internal/tts/command.go internal/sh…`
(several files, image mode) reached **2.1 GB RSS after 3s** and was killed by hand (pid 1229245). Developer
agents call `harnez read -I` on multi-file sets because the lean-sprint skill tells them to. The 16 GB
OOM kills fit this pattern. **Primary suspect: `harnez read -I` with multiple files and a line range.**
Repro: run it on a few ~400-line Go files and watch the RSS.

## Earlier suspects (now secondary)

- `harnez exec` output capture: the quota-1 log, distill buffering, or an in-memory copy of the child
  output that grows with a verbose or long-streaming child.
- `harnez agent resume` streaming: accumulation of JSON events or messages for the whole turn.
- The ef0408b stopped-group monitor (full /proc scans every 25ms); replaced in 541 (1s, child stat only).
  Rule it in or out.

## Notes

- Reproduce: run a long, chatty child under `harnez exec` (e.g. `yes | head -c 5G`) and a long agent
  resume, and watch RSS (`ps -o rss`). Check `pprof` heap if RSS grows.
- Acceptance: RSS stays flat (e.g. <100 MB) for a multi-GB output stream and for a 10-minute agent turn.

## Update: host preflight on HEAD (2026-09-24)

- `harnez read -I -L 1:400` on 5–8 voxi files (`internal/tts`, `internal/shortcut`) peaked at 30–45 MB, whether
  it wrote to a pipe or a file and whether it ran bare or under `harnez exec`. Not reproduced.
- User: the 2.1 GB `read -I` sighting may have been the voice engine (voxi TTS), not harnez. Other memory spikes
  near **20 GB** were seen. So `read -I` is **not confirmed** as the cause. Treat the earlier suspects (`harnez exec`
  output capture, `harnez agent resume` streaming) as equal candidates again. Before blaming a process, check which
  process actually holds the RSS (`ps`/`journalctl -k` pid → cmdline).

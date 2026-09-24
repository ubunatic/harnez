# Process Hygiene — exec wrappers, stuck runs and `harnez clean`

How harnez keeps agent-launched processes from hanging turns, holding the quota-1 test budget, or
leaking CPU and memory. It is the runtime side of AgenticLoop Invariant 3 (Zero Zombie Guarantee).
Sources: issues 532, 533, 537, 538, 541, 543.

## Layers

| Layer | Mechanism | Where |
|---|---|---|
| Per command | `harnez exec`: own process group, 60s default timeout (`HTO=`), group kill on timeout | `cmd/harnez/exec.go` |
| Per command | Stopped-child recovery: a stopped (`T`) process anywhere in the child's tree → SIGCONT once, then kill, exit **125** | `cmd/harnez/stopped.go` |
| Test budget | quota-1 state is a run record `{started, pgid, pid_starttime, finished, exit, retry}`; only a normal exit consumes the quota, a killed/stopped run gets one retry | `internal/quota1` |
| After the fact | `harnez clean [procs] [q1] [--kill]` (dry-run by default) | `cmd/harnez/clean.go`, `internal/procs` |
| Agent routing | every agent's shell commands go through `harnez exec` (hooks, agy shim) | [HookRewritePattern.md](HookRewritePattern.md) |

## `harnez clean`

- `procs`: process groups recorded by `exec` (`$XDG_RUNTIME_DIR/harnez/procs/<pgid>.json`) that are
  stopped, or whose owner (pid + start time) is dead. SIGTERM → grace period → SIGKILL. Only the current
  uid, and the start time is checked against PID reuse.
- `q1`: releases the quota-1 state **only when harnez can verify** that the last run has no recorded exit
  and its process group is gone. An agent's belief that it is stuck is not enough; otherwise this would be the
  forbidden bypass. The quota-1 "blocked" message points agents to `harnez clean procs q1 --kill`.
- The old `clean` (remove managed blocks) is now `harnez revert --managed` (`make revert-managed`).
  `make clean` must never wipe global config.

## Decisions

- **No turn time limit.** Turn lengths vary (5–17 min in one sprint) and no history exists to base a limit on.
  Per-command bounds (above) free a hung turn without cutting off long legitimate ones.
- **Report leftovers, don't kill them at turn end** (538, planned). A turn may leave a dev server running on
  purpose. Only stopped or ownerless groups are reaped. systemd units and `setsid`/`systemd-run --user`
  processes never join the agent's group, so they are unaffected. Agents that mean to leave something
  running should detach it explicitly.

## Pitfalls (hard-won)

- **Stopped processes don't run their timers.** `go test -timeout` never fires on a stopped tree, which is why
  532 lasted 10h. Only an outside observer (exec's monitor, `clean`) can recover it.
- **Never scan all of /proc in a wait loop.** The first stopped-child monitor read `stat`+`status` of every
  process every 25ms: ~94% of a core per wrapper, ~3 cores per agent turn (541). Now it walks only the child's
  tree via `/proc/<pid>/task/<tid>/children` once per second (~0.3% CPU). Measure CPU for any new polling code.
- **Direct-child-only checks miss the real case.** A stop usually hits a grandchild (`go test` under `make`). Test
  with `sh -c 'sh -c "kill -STOP \$\$"'`, not only `sh -c 'kill -STOP $$'`.
- **`exec` itself can be killed.** Then no result is written. quota-1 treats "no `finished`" as possibly still
  running and blocks. Recovery is `clean procs q1`, not an automatic retry.
- **Memory:** `harnez read -I` on multiple files reached 2.1 GB in 3s and caused 15–16 GB OOM kills (543).
  Its recommendation is paused until that is fixed.

## Verification recipes

```bash
harnez exec --quota-1 -- sh -c 'kill -STOP $$'          # → exit 125 in ~2s, quota not consumed
harnez exec -- sh -c 'sh -c "kill -STOP \$\$"; echo x'   # descendant stop → 125
harnez exec -- sleep 30 & sleep 3; ps -o %cpu -C harnez  # waiting CPU < 1%
harnez clean                                             # dry-run report
harnez stats --agents --days 1                           # AGY shell coverage: 0 UNROUTED
```

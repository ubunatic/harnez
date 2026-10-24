# Optional Neus integration

`harnez install neus` runs `go install ubunatic.com/neus/cmd/neus@latest`,
then `neus install` to bootstrap the tool. Go and the installed binary must be
on PATH. Preview both commands with `harnez install neus --dry-run`.
Supported packages and bootstrap commands live in `spec/dependencies.yaml`.
The existing `harnez find` neus finder continues to invoke the binary directly.

Server indexing is disabled by default. Register the known repositories explicitly:

```sh
harnez server --neus-index --neus-repo /path/to/repo --neus-repo /path/to/another
```

These flags apply to the foreground server invocation; `server install` does not
persist them into its systemd unit. The server does not discover or scan arbitrary
directories. Paths are canonicalized and deduplicated, and must exist at startup.
`spec/server.yaml` owns `neus_indexing.enabled`, `repos`, `interval` (5m), `timeout`
(10m), `command` (`neus index`) and `service_name` (`neus-indexer.service`).
The first scan happens on the first interval tick. Each invocation receives all
registered roots as separate arguments. Failed scans are logged and retried on a
later tick. Scans run serially, with cancellation and a bounded command timeout;
server shutdown waits for services before releasing its socket.

Before every cycle, Harnez runs
`systemctl --user is-active --quiet neus-indexer.service`. An active service skips
the cycle; inactive/unknown-unit status permits indexing. Missing `systemctl` also
permits indexing. Other status failures skip the cycle and log the error.

Coordination assumption: sibling Neus issue 057 proposes this unit name, while
065/067 still track daemon and service implementation. The inspected Neus CLI has
`neus index <dir>...` and `neus install`, but its source has no indexer service or
shared writer lock yet. This status probe cannot prevent a service starting midway through
a Harnez scan, or coordinate manual `neus index` commands. Use one indexing owner;
shared-lock coordination awaits Neus's service implementation.

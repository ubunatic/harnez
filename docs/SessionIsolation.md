# Session Local Staging

`bash scripts/stage-install.sh "/absolute/private/path"` builds `harnez` and
its man pages under a new destination. Pass the destination as one quoted
positional argument; there is intentionally no Make staging entry point. The
script currently requires Linux,
Bash, GNU or uutils coreutils `realpath`/`stat`, GNU Make, and Go 1.20 or newer
(for `go build -C`). The destination's parent must already exist,
be a real directory with mode `0700`, and have no symlink components. The
destination itself must not exist. A destination with spaces is supported.

The stage contains `bin/harnez` and `share/man/man1/`. The command does not run
`go install`, install the `⚙` alias, or write man pages to a shared location.
On build or man generation failure it removes the newly created stage. The
caller must provide a trusted private parent, such as a session workspace
created by its host. This workflow does not install into shared destinations
safely: it does not provide a trusted installer or defeat same-UID path races.

The session registry remains at its existing shared default unless the caller
supplies `--store-dir`. Changing that default would currently hide runner
records from host discovery and resume because hosts do not have a contract for
discovering runner-local stores. Workers inherit `TAIA_SESSION_DIR` and nested
workers receive an explicit `--store-dir`; this is not a complete registry
discovery contract or a same-UID security boundary. Invalid runner state must
not silently fall back to shared state.

The stage test uses fake Go and binary commands to check path validation,
failure cleanup and exit status, and that hostile `GOBIN`, `GOPATH`, `HOME`,
and `XDG_DATA_HOME` values do not redirect writes. It invokes the script
directly with quotes, backticks and literal command-substitution text in the
quoted destination, verifies no marker was created, and checks shared binary,
alias and man directories retain only their sentinels. This test is part of
`make test`. The caller's shell and environment must be trusted; this workflow
does not protect against arbitrary shell code or hostile `MAKEFLAGS` processed
before invocation. A real
staged binary should also be checked for generated man pages. Taia's private directory creation and
Landlock policy are external assumptions; Harnez cannot prove cross-runner
denial from these tests.

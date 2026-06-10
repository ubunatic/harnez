<!-- claudeconfig:bundled -->
# C/C++ conventions

## Build acceleration

### ccache with CGO (Go + C/C++ via cgo)

Set `CC` and `CXX` to the ccache wrappers when invoking `go build` for CGO targets:

```makefile
CC  = ccache gcc
CXX = ccache g++
```

Export both in the build environment variable block, e.g.:

```makefile
QT_BUILD_ENV := CC="ccache gcc" CXX="ccache g++" GOGC=50 GOMEMLIMIT=512MiB
build-qt:
	$(QT_BUILD_ENV) go build -p 1 -v -o instamoji-qt ./cmd/instamoji-qt
```

Install ccache: `sudo apt install ccache`

Check hit rate: `ccache -s`

### Notes

- `-p 1` (single parallel job) is required for heavy Qt/CGO builds to avoid OOM on low-RAM hosts; ccache compensates for the lost parallelism on rebuilds.
- `GOGC=50 GOMEMLIMIT=512MiB` caps Go GC pressure during the CGO compile phase.

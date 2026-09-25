# Zero-Allocation Command Output Distillation Pipeline

## Overview

The `internal/distill` package filters verbose command output (such as `go test -v`, `git status`, and repeated warning logs) to minimize token consumption in agent contexts. Previously, `FilterGoTest`, `FilterGit`, and `FilterDeduplicate` allocated a string object and `[]string` slice entry for every single line in the stream, along with allocating a fresh 64KB `bufio.Scanner` buffer on every call. In addition, line truncation (`FilterHeadTail`) split entire output buffers using `strings.Split(s, "\n")`.

For a typical 25KB / 1000-line test run, this generated 1,022 heap allocations and consumed 152KB of memory per invocation.

The optimization replaces string-per-line conversions and line slices with:
1. **Pooled Scanner Buffers**: Reusing 64KB `bufio.Scanner` buffers via `sync.Pool`.
2. **Byte Buffer Streaming**: Writing raw line bytes (`scanner.Bytes()`) directly into `bytes.Buffer` instances without intermediate string conversions.
3. **Zero-Copy Line Truncation**: Performing head/tail line bounding via `strings.IndexByte` and `strings.LastIndexByte` byte offsets instead of splitting the entire string into a slice of lines.

## Hardware Target

- Multi-core x86_64 systems (4 to 10 active CPU cores tested).
- Memory footprint: Reduced per-call heap allocation from ~150KB to <1KB, eliminating hot-loop GC pressure during high-throughput CLI tool executions.

## Topology / Data Flow

```
[Raw Tool Output / Pipe Stream]
              |
              v
 [Pooled Scanner Buffer (64KB sync.Pool)]
              |
              v
 [Byte Buffer Stream (FilterGoTest / FilterGit)]
              |
              v
 [Zero-Copy Line Truncation (strings.IndexByte)]
              |
              v
   [Distilled Output String]
```

## Benchmark Evidence

Tested on Linux x86_64 using `go test -bench=Benchmark -benchmem -cpu=1,2,4,8,10 -count=5 ./internal/distill`.

### `BenchmarkDistill_GoTest` (End-to-End Pipeline)
- **ns/op**: 133,798 ns/op -> **38,597 ns/op** (**3.47x faster**)
- **B/op**: 152,137 B/op -> **846 B/op** (**180x reduction**)
- **allocs/op**: 1,022 allocs/op -> **11 allocs/op** (**98.9% allocation drop**)

### `BenchmarkFilterGoTest`
- **ns/op**: 136,007 ns/op -> **36,465 ns/op** (**3.73x faster**)
- **B/op**: 86,092 B/op -> **418 B/op** (**206x reduction**)
- **allocs/op**: 1,012 allocs/op -> **5 allocs/op** (**99.5% allocation drop**)

### `BenchmarkFilterGit`
- **ns/op**: 80,100 ns/op -> **15,370 ns/op** (**5.21x faster**)
- **B/op**: 92,901 B/op -> **644 B/op** (**144x reduction**)
- **allocs/op**: 521 allocs/op -> **6 allocs/op** (**98.8% allocation drop**)

### `BenchmarkFilterDeduplicate`
- **ns/op**: 117,979 ns/op -> **35,247 ns/op** (**3.35x faster**)
- **B/op**: 101,924 B/op -> **11,265 B/op** (**9.05x reduction**)
- **allocs/op**: 1,211 allocs/op -> **10 allocs/op** (**99.2% allocation drop**)

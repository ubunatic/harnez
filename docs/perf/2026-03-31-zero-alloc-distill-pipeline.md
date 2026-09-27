# Performance Optimization: Zero-Allocation Stream Distillation

## Overview
The `internal/distill` package filters low-signal noise (passing test output, ANSI formatting, repeated warnings, oversized outputs) from command streams before feeding them into agent context windows.

Previously, `FilterGoTest`, `FilterGit`, `FilterDeduplicate`, and `FilterHeadTail` allocated individual `string` objects for every scanned line and created intermediate `[]string` slices, followed by `strings.Join`. For a standard 500-test transcript (~1000 lines), this triggered over 1000 heap allocations per distillation call.

By switching to `strings.Builder` output sinks, reusable `[]byte` pending buffers (`buf = buf[:0]`), zero-alloc integer formatting (`strconv.Itoa`), and fast newline offset scanning (`FilterHeadTailString`), allocations dropped from **1022 allocs/op** to **13 allocs/op** (98.7% allocation reduction), and execution latency improved by **>2.1x** (160 µs -> 69 µs).

## Hardware Target
- Target OS: Modern Linux
- Hardware Topology: Multi-core x86_64 systems (4 to 10 cores evaluation cap)
- Memory Footprint: Reduced heap GC pressure in stream processing hot paths

## Topology / Data Flow
```
[Raw Command Output]
        │
        ▼
[bufio.Scanner / scanner.Bytes()]
        │
        ├─► [FilterGoTest / FilterGit / FilterDeduplicate] (Reused []byte pending buffer)
        │                                 │
        │                                 ▼
        ├─► [strings.Builder Output Sink]
        │                                 │
        │                                 ▼
        └─► [FilterHeadTailString] (Byte scan via strings.IndexByte / LastIndexByte)
                                          │
                                          ▼
                               [Distilled Output String]
```

## Benchmark Evidence

Command: `go test -bench=BenchmarkDistill_GoTest -benchmem -cpu=1,2,4,8,10 -count=5 ./internal/distill`

| Metric | Before Optimization | After Optimization | Delta |
|--------|---------------------|--------------------|-------|
| **Latency (ns/op)** | 160,078 ns/op | 69,293 ns/op | **-56.7% (-2.3x faster)** |
| **Allocations (allocs/op)** | 1,022 allocs/op | 13 allocs/op | **-98.7% (-1009 allocs)** |
| **Allocated Bytes (B/op)** | 152,139 B/op | 131,672 B/op | **-13.5% (-20,467 B)** |

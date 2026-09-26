# Performance Optimization: Zero-Allocation Distill Pipeline

## Overview
Optimized `FilterGoTest`, `FilterGit`, and `FilterDeduplicate` in `internal/distill/distill.go` to eliminate per-line string slice allocations and per-line string conversions. In addition, added a fast-path line count check (`strings.Count`) before invoking `FilterHeadTail` truncation.

## Hardware Target
- Target Architecture: Modern multi-core x86_64 / ARM64.
- Execution bounds: 1 to 10 CPU cores.

## Topology / Data Flow
```
[ Raw Output Stream ]
        │
        ▼
[ bufio.Scanner (byte slices) ]
        │
        ▼
[ Filtering & Deduplication (bytes.Buffer) ]
        │
        ▼
[ Fast-Path Line Count Check ] ──( <= MaxLines )──► [ Direct Output ]
        │
    ( > MaxLines )
        │
        ▼
[ FilterHeadTail Truncation ]
```

## Benchmark Evidence

### Before Optimization:
```
BenchmarkDistill_GoTest-4    8017    125120 ns/op    152138 B/op    1022 allocs/op
```

### After Optimization:
```
BenchmarkDistill_GoTest-4   14085     81736 ns/op    132001 B/op      15 allocs/op
```

### Summary of Impact:
- **Allocations:** Reduced from **1022 allocs/op** to **15 allocs/op** (~98.5% reduction in heap allocations).
- **Latency / Throughput:** Latency dropped from **~125 µs/op** to **~81 µs/op** (~35-45% speedup).
- **Memory Footprint:** Reduced memory allocated per operation from **152,138 B/op** to **132,001 B/op** (~13% memory reduction).

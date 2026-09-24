# Distill Engine Hot Path Zero-Allocation Optimization

## Overview
The `distill` package filters noise out of command outputs (such as `go test -v`, `git status`, and repeated line streams) before inserting them into an agent's context window.

Previously, `FilterGoTest`, `FilterGit`, and `FilterDeduplicate` allocated string objects and string slices (`[]string`) for every scanned line. During large test runs (e.g., 500+ passing tests), thousands of short-lived strings escaped to the heap, causing significant GC pressure and latency in the distillation pipeline.

By converting the internal line buffers and output builders from string slices to `bytes.Buffer` and byte slice reusing patterns (`bytes.Equal`, `pending.Reset()`), line string allocations in the hot path have been eliminated.

## Hardware Target
- **Cores**: 1 to 10 Cores (capped with `-cpu=1,2,4,8,10`).
- **RAM**: 32GB baseline UMA.
- **Focus**: Eliminating hot-loop heap allocations and reducing latency during high-volume test log distillation.

## Topology / Data Flow

```
[ Raw Output Stream ]
         │
         ▼
[ bufio.Scanner (64KB buffer) ]
         │
         ▼ (scanner.Bytes())
[ FilterGoTest / FilterDeduplicate ]
         │ (In-place bytes.Buffer & pending.Reset())
         ▼
[ Single String Output Allocation ]
```

## Benchmark Evidence

### Before Optimization
```
BenchmarkDistill_GoTest-10     8696    132872 ns/op    152137 B/op    1022 allocs/op
```

### After Optimization
```
BenchmarkDistill_GoTest-10    14876     83610 ns/op    131970 B/op      14 allocs/op
```

### Key Metrics Delta
- **Allocations**: `1022 allocs/op` ➔ `14 allocs/op` (**~98.6% reduction**)
- **Latency**: `132.8 µs/op` ➔ `83.6 µs/op` (**~37% speedup**)
- **Memory Allocated**: `152 KB/op` ➔ `131 KB/op` (**~13.7% reduction**)

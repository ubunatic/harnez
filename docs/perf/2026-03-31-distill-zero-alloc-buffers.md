# Performance Optimization: Zero-Allocation Distill Filters

## Overview
Optimized `FilterGoTest`, `FilterGit`, `FilterDeduplicate`, and `Distill` in `internal/distill/distill.go` by converting scanner iteration and output line buffering to direct `bytes.Buffer` accumulation and zero-allocation byte slice slice comparisons.

Key changes:
- Replaced line string allocations (`scanner.Text()`, `string(lineBytes)`) and slice-based line accumulation (`[]string`) in `FilterGoTest`, `FilterGit`, and `FilterDeduplicate` with reusable `bytes.Buffer` buffers (`outBuf`, `pendingBuf`, `prevBuf`).
- Replaced `fmt.Sprintf` and untracked file string slice allocations in `FilterGit` with a simple integer counter and `strconv.Itoa`.
- Added a line count check (`strings.Count(s, "\n") >= opts.MaxLines`) before executing `strings.Split` in `Distill` line head/tail truncation.

## Hardware Target
- Target Architecture: Multi-core x86_64 / ARM64, 4-10 Physical Cores, 32GB UMA Memory Baseline.
- Runtime Constraint: Enforces <=10 worker concurrency ceiling across test suites and benchmark runners.

## Topology / Data Flow

```
[Raw Tool Output Stream]
          │
          ▼
 [bufio.Scanner (64KB)]
          │
          ▼
   [Byte Line Stream] ──(Zero-alloc bytes.HasPrefix / bytes.Equal)
          │
          ▼
   [bytes.Buffer Arena] ──(WriteByte / Write)
          │
          ▼
[Distilled Context String]
```

## Benchmark Evidence

### Before Optimization (Base)
```
pkg: ubunatic.com/harnez/internal/distill
cpu: Intel(R) Xeon(R) Processor @ 2.30GHz
BenchmarkDistill_GoTest-4     7942     126626 ns/op     152138 B/op     1022 allocs/op
```

### After Optimization
```
pkg: ubunatic.com/harnez/internal/distill
cpu: Intel(R) Xeon(R) Processor @ 2.30GHz
BenchmarkDistill_GoTest-4    14672      81588 ns/op     131921 B/op       13 allocs/op
```

### Metric Comparison
| Metric | Before | After | Change |
|---|---|---|---|
| Latency (`ns/op`) | 126.6 µs | 81.5 µs | **-35.6%** |
| Memory (`B/op`) | 152.1 KB | 131.9 KB | **-13.3%** |
| Allocations (`allocs/op`) | 1022 allocs/op | 13 allocs/op | **-98.7%** |

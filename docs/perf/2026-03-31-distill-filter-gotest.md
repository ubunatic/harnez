# Performance Optimization: Zero-Allocation `FilterGoTest` and Output Distillation

## Overview

The `internal/distill` package filters low-signal test outputs (passing test noise, ANSI codes, duplicate warning lines) to minimize token consumption in agent contexts. Previously, `FilterGoTest`, `FilterGit`, and `FilterDeduplicate` allocated string objects for every scanned line—including passing `=== RUN` lines that were immediately discarded upon seeing `--- PASS`. In a 500-test run, this resulted in over 1,000 heap allocations (~152 KB) per distillation pass.

The optimization replaces string-slice buffering with:
1. Reusable byte buffer (`pendingBuf = pendingBuf[:0]`) for transient `=== RUN` headers, avoiding heap allocations for passing tests.
2. Direct output streaming to `strings.Builder` with fast integer formatting via `strconv.AppendInt`.
3. Non-allocating `FilterHeadTailString` operating directly on string byte indices instead of allocating `[]string` via `strings.Split`.

## Hardware Target

- Multi-core Linux workstations and cloud runners (capped at <=10 cores).
- 32GB RAM baseline; zero-allocation hot paths reduce GC pauses and cache thrashing during high-frequency agent tool execution.

## Topology / Data Flow

```
[Raw Output Stream (io.Reader)]
        |
        v
[bufio.Scanner (4KB initial buf)]
        |
        +---> [=== RUN / Transient] ---> [Reusable pendingBuf []byte] ---> (Discard on PASS / Flush on FAIL)
        |
        +---> [FAIL / Summary / Output] ---> [strings.Builder] ---> [Distilled String Output]
```

## Benchmark Evidence

Tested on Intel Xeon @ 2.30GHz (4 worker ceiling):

| Metric | Before Optimization | After Optimization | Improvement |
| :--- | :--- | :--- | :--- |
| **Execution Latency (ns/op)** | 125,600 ns/op | 42,591 ns/op | **~2.95x faster (66% reduction)** |
| **Allocated Bytes (B/op)** | 152,138 B/op | 8,789 B/op | **>94% reduction** |
| **Heap Allocations (allocs/op)** | 1,022 allocs/op | 13 allocs/op | **>98.7% reduction** |

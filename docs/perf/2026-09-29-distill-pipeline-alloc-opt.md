# Optimization: Zero-Allocation Buffer Streaming for Distill Pipeline

## Overview
The `distill` package filters verbose test logs and repeated terminal output to minimize agent context window overhead. In execution loops, `FilterGoTest` and `FilterDeduplicate` allocated string objects and string slice elements for every scanned line, creating significant garbage collection pressure and allocation overhead.

This optimization refactors the pipeline to:
1. Stream line output directly into a growing `bytes.Buffer` rather than building `[]string` slice accumulators and joining with `strings.Join`.
2. Keep pending test execution blocks in a reusable `bytes.Buffer`, clearing it with `.Reset()` when passing/skipping tests are encountered without allocating intermediate strings or slices.
3. Perform consecutive line comparison on raw byte slices (`bytes.Equal`) in `FilterDeduplicate` and format repetition counts using `strconv.AppendInt` into stack-allocated byte arrays.

## Hardware Target
- Target OS: Modern Linux x86_64
- Topology / Core Ceiling: Bounded 1-10 Cores (`-cpu=1,2,4,8,10`)

## Topology / Data Flow

```
[ Raw Output Stream ]
          │
          ▼
 [ bufio.Scanner ] ──► scanner.Bytes()
          │
          ├──► [ FilterGoTest ]
          │          ├── (=== RUN / PASS / SKIP) ──► pending.Reset() / bytes.Buffer buffer write
          │          └── (--- FAIL / ok / FAIL)  ──► flush to out bytes.Buffer
          │
          ▼
 [ FilterDeduplicate ]
          ├── bytes.Equal(line, prev) ──► count++
          └── flush ──► strconv.AppendInt directly to out bytes.Buffer
```

## Benchmark Evidence

### Environment
- Go version: `go1.26.5 linux/amd64`
- CPU: `Intel(R) Xeon(R) Processor @ 2.30GHz`
- Command: `go test -bench=BenchmarkDistill_GoTest -benchmem -cpu=1,2,4,8,10 -count=5 ./internal/distill/`

### Baseline vs. Optimized

| Metric | Baseline | Optimized | Delta |
| :--- | :--- | :--- | :--- |
| **Execution Latency (ns/op)** | ~133,473 ns/op | ~76,629 ns/op | **-42.6%** |
| **Allocated Memory (B/op)** | 152,135 B/op | 131,920 B/op | **-13.3%** |
| **Allocations (allocs/op)** | 1,022 allocs/op | 13 allocs/op | **-98.7%** |

### Summary
By eliminating 1,009 heap allocations per test output distillation run, allocation count dropped from 1,022 allocs/op down to 13 allocs/op (-98.7%), and output distillation throughput latency was reduced by ~42.6%.

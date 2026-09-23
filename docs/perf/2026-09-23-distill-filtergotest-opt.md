# Optimization: Fast-Path ANSI Stripping & Zero-Alloc Scanner Bytes in Distill

## Overview
The `distill` package filters verbose test logs and terminal output to minimize context window token usage for agentic loops. In hot execution loops, `Distill` called `StripANSI` (which invoked `regexp.ReplaceAllString` on every line) and `FilterGoTest` (which converted every line buffer to `string` using `scanner.Text()` and `strings.TrimSpace()`).

This optimization introduces:
1. Fast-path check in `StripANSI` (`!strings.Contains(s, "\x1b[")`) to skip expensive regex evaluation when no escape sequences exist.
2. In-place byte slice matching in `FilterGoTest` using `scanner.Bytes()`, `bytes.TrimSpace()`, and package-level `[]byte` prefix constants (`prefixRun`, `prefixPass`, `prefixFail`, etc.), postponing `string` creation only for retained output lines.

## Hardware Target
- Target OS: Modern Linux x86_64
- Topology / Core Ceiling: Bounded 1-10 Cores (`-cpu=1,2,4,8,10`)

## Topology / Data Flow

```
[ Raw Test / Terminal Output Stream ]
                  │
                  ▼
         [ StripANSI Fast-Path ] ─── (Contains \x1b[ ?) ───► Yes ──► [ regexp.ReplaceAllString ]
                  │                                                           │
                  │ No                                                        │
                  └───────────────────────┬───────────────────────────────────┘
                                          │
                                          ▼
                             [ bufio.Scanner (64KB buf) ]
                                          │
                                          ▼
                                  [ scanner.Bytes() ]
                                          │
                  ┌───────────────────────┴───────────────────────┐
                  ▼                                               ▼
     [ bytes.TrimSpace + bytes.HasPrefix ]           [ Convert to string ONLY ]
       (Zero-allocation line inspection)              (When appended to pending/out)
```

## Benchmark Evidence

### Environment
- Go version: `go1.26.5 linux/amd64`
- CPU: `Intel(R) Xeon(R) Processor @ 2.30GHz`
- Command: `go test -bench=BenchmarkDistill_GoTest -benchmem -cpu=1,2,4,8,10 -count=5 ./internal/distill/`

### Baseline vs. Optimized

| Metric | Baseline | Optimized | Delta |
| :--- | :--- | :--- | :--- |
| **Execution Latency (ns/op)** | 138,206 ns/op | 117,044 ns/op | **-15.3%** |
| **Allocated Memory (B/op)** | 222,708 B/op | 152,135 B/op | **-31.7%** |
| **Allocations (allocs/op)** | 1,526 allocs/op | 1,022 allocs/op | **-33.0%** |

### Summary
By eliminating 504 heap allocations per test distillation pass and bypassing regex evaluation when escape codes are absent, hot-path output distillation latency is reduced by ~15% while cutting GC heap churn by one third.

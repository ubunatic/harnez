# Optimization: Fast-Path ANSI Checking in ReadCard

## Overview
When rendering visual card previews for agentic context delivery (`readcard`), lines of text pass through `stripANSIEscapes` and `parseANSILine`. Previously, every line triggered `strings.Builder` byte copying and loop parsing even when completely devoid of ANSI escape sequences (`\x1b`).

This optimization introduces:
1. Fast-path check in `stripANSIEscapes` (`!strings.Contains(s, "\x1b")`) to return the input string directly with **zero allocations** and **~98% latency reduction** on plain text lines.
2. Fast-path check in `parseANSILine` (`!strings.Contains(line, "\x1b")`) to return a single `TokenText` slice directly with **~55% latency reduction** on plain text lines.

## Hardware Target
- Target OS: Modern Linux x86_64
- Topology / Core Ceiling: Bounded 1-10 Cores (`-cpu=1,2,4,8,10`)

## Topology / Data Flow

```
[ Input Text Line ]
         │
         ▼
[ Contains \x1b ? ] ───► No ──► [ Return input string / single Token slice directly ] (0 allocs)
         │
         │ Yes
         ▼
[ Full SGR / ANSI Parser ] ──► [ Loop & Parse Control Sequences ]
```

## Benchmark Evidence

### Environment
- Go version: `go1.26.5 linux/amd64`
- CPU: `Intel(R) Xeon(R) Processor @ 2.30GHz`
- Command: `go test -bench="BenchmarkStripANSI|BenchmarkParseANSI" -benchmem -cpu=1,2,4,8,10 ./internal/readcard/`

### Baseline vs. Optimized (Plain Text Lines)

| Metric | Baseline | Optimized | Delta |
| :--- | :--- | :--- | :--- |
| **stripANSIEscapes Latency (ns/op)** | 543.3 ns/op | 11.8 ns/op | **-97.8%** |
| **stripANSIEscapes Memory (B/op)** | 248 B/op | 0 B/op | **-100.0%** |
| **stripANSIEscapes Allocs (allocs/op)** | 5 allocs/op | 0 allocs/op | **-100.0%** |
| **parseANSILine Latency (ns/op)** | 180.8 ns/op | 83.2 ns/op | **-54.0%** |
| **parseANSILine Memory (B/op)** | 48 B/op | 48 B/op | **0.0%** |
| **parseANSILine Allocs (allocs/op)** | 1 allocs/op | 1 allocs/op | **0.0%** |

### Summary
By checking for the existence of `\x1b` before entering builder and parsing loops, `stripANSIEscapes` drops to 0 allocations and 11.8 ns/op on plain text lines (~46x speedup), while `parseANSILine` latency drops by over 50%.

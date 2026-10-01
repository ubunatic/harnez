# Distill Package Zero-Allocation Buffer Streaming & Fast-Paths

## Overview
The `distill` package filters low-signal noise (such as verbose passing test logs, duplicate status lines, and oversized output) before it enters an agent's context window.

During profiling, three major allocation hot-paths were identified in `internal/distill/distill.go`:
1. `FilterGoTest` allocated a `[]string` slice and converted every scanned line from `[]byte` to `string` (e.g., 1022 allocs/op for a 500-test run). Passing tests (`=== RUN` -> `--- PASS`) allocated slices and strings only to be discarded when `pending` was cleared.
2. `FilterDeduplicate` invoked `scanner.Text()` on every line, allocating strings for comparison and appending to `[]string` before joining with `\n`.
3. `FilterHeadTail` received `strings.Split(s, "\n")` directly from `Distill`, allocating line slices and strings even when the string length had fewer lines than `maxLines`.

The fixes:
- Refactored `FilterGoTest` to stream lines directly into `bytes.Buffer`s (`out` and `pending`). On passing tests, `pending.Reset()` clears buffered bytes in $O(1)$ without heap allocations.
- Refactored `FilterDeduplicate` to use `scanner.Bytes()` and `bytes.Equal()`, streaming unique output directly to a `bytes.Buffer`.
- Introduced `FilterHeadTailString` fast-path using `strings.Count(s, "\n")` to avoid line-splitting allocation when input line count is within `maxLines`.

## Hardware Target
- Topology: Multi-Core x86_64, Zen 4+ / AVX-512 capable.
- Bound: 10-core worker ceiling (`go test -cpu=1,2,4,8,10`).

## Topology / Data Flow

```
[Raw Tool Output Stream]
          │
          ▼
 [bufio.Scanner (64KB buffer)]
          │
          ├──> FilterGoTest: bytes.Buffer streaming & pending.Reset()
          │
          ├──> FilterDeduplicate: bytes.Equal() & bytes.Buffer streaming
          │
          └──> FilterHeadTailString: strings.Count fast-path (0 allocs if <= maxLines)
          │
          ▼
   [Distill Output]
```

## Benchmark Evidence

Comparison via `benchstat` baseline vs. optimized:

```
                              │ /tmp/baseline.txt │         /tmp/optimized.txt          │
                              │      sec/op       │    sec/op     vs base               │
Distill_GoTest                      127.95µ ± ∞ ¹   70.03µ ± ∞ ¹  -45.26% (p=0.008 n=5)
Distill_GoTest-10                   125.82µ ± ∞ ¹   83.14µ ± ∞ ¹  -33.93% (p=0.008 n=5)
FilterDeduplicate                    326.9µ ± ∞ ¹   245.1µ ± ∞ ¹  -25.00% (p=0.008 n=5)
FilterDeduplicate-10                 287.9µ ± ∞ ¹   225.7µ ± ∞ ¹  -21.61% (p=0.008 n=5)
Distill_HeadTailUnderLimit          4598.0n ± ∞ ¹   153.9n ± ∞ ¹  -96.65% (p=0.008 n=5)
Distill_HeadTailUnderLimit-10       4706.0n ± ∞ ¹   160.1n ± ∞ ¹  -96.60% (p=0.008 n=5)

                              │ /tmp/baseline.txt │            /tmp/optimized.txt             │
                              │       B/op        │     B/op       vs base                    │
Distill_GoTest                      148.6Ki ± ∞ ¹   128.9Ki ± ∞ ¹   -13.26% (p=0.008 n=5)
FilterDeduplicate                   220.2Ki ± ∞ ¹   171.8Ki ± ∞ ¹   -22.00% (p=0.008 n=5)
Distill_HeadTailUnderLimit          4.375Ki ± ∞ ¹   0.000Ki ± ∞ ¹  -100.00% (p=0.008 n=5)

                              │ /tmp/baseline.txt │           /tmp/optimized.txt            │
                              │     allocs/op     │  allocs/op   vs base                    │
Distill_GoTest                      1022.00 ± ∞ ¹   14.00 ± ∞ ¹   -98.63% (p=0.008 n=5)
FilterDeduplicate                    2514.0 ± ∞ ¹   515.0 ± ∞ ¹   -79.51% (p=0.008 n=5)
Distill_HeadTailUnderLimit            2.000 ± ∞ ¹   0.000 ± ∞ ¹  -100.00% (p=0.008 n=5)
```

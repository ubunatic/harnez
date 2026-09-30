## 2026-03-31 - Fast-Path ANSI Check in ReadCard Line Processing
**Learning:** `stripANSIEscapes` and `parseANSILine` in `readcard` were allocating `strings.Builder` and token slices for every single input line even when no escape sequences (`\x1b`) were present.
**Action:** Always check `!strings.Contains(s, "\x1b")` upfront before allocating builders or slice parsers for terminal text lines.

## 2026-03-31 - Zero-Allocation Pending Pools for Discarded Line Streams in Distill
**Learning:** Storing lines as `string` in temporary line-buffering accumulators (like `pending []string` for `go test` output) creates high GC heap allocation churn when most buffered lines are discarded upon test pass (`--- PASS`).
**Action:** Use a reusable byte slice pool (`pending [][]byte`) that recycles element slice capacities (`pending[i] = append(pending[i][:0], b...)`), avoiding string allocations until output lines are explicitly selected for emission.

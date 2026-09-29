## 2026-03-31 - Fast-Path ANSI Check in ReadCard Line Processing
**Learning:** `stripANSIEscapes` and `parseANSILine` in `readcard` were allocating `strings.Builder` and token slices for every single input line even when no escape sequences (`\x1b`) were present.
**Action:** Always check `!strings.Contains(s, "\x1b")` upfront before allocating builders or slice parsers for terminal text lines.

## 2026-09-29 - Zero-Allocation Buffer Streaming in Distill Pipelines
**Learning:** `bufio.Scanner` loops that build `[]string` slice accumulators for multi-line filtering or deduplication incur significant GC churn (allocating strings and slice re-allocations per line).
**Action:** Stream line byte slices (`scanner.Bytes()`) directly into `bytes.Buffer` instances and use stack-based integer formatting (`strconv.AppendInt`) for counters to achieve near-zero allocation line processing.

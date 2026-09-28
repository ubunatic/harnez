## 2026-03-31 - Fast-Path ANSI Check in ReadCard Line Processing
**Learning:** `stripANSIEscapes` and `parseANSILine` in `readcard` were allocating `strings.Builder` and token slices for every single input line even when no escape sequences (`\x1b`) were present.
**Action:** Always check `!strings.Contains(s, "\x1b")` upfront before allocating builders or slice parsers for terminal text lines.

## 2026-03-31 - Zero-Allocation Byte Buffer Streaming in Distill Filters
**Learning:** In line-by-line output distillation (e.g. `FilterGoTest`, `FilterGit`, `FilterDeduplicate`), collecting line strings into `[]string` and converting `scanner.Bytes()` to `string` per line generated thousands of transient heap allocations per command execution.
**Action:** Accumulate bytes directly in pre-sized `bytes.Buffer` buffers with `WriteByte('\n')` and use `bytes.Equal`/`bytes.HasPrefix` on raw scanner byte slices to keep heap allocations near zero.

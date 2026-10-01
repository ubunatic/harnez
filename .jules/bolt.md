## 2026-03-31 - Fast-Path ANSI Check in ReadCard Line Processing
**Learning:** `stripANSIEscapes` and `parseANSILine` in `readcard` were allocating `strings.Builder` and token slices for every single input line even when no escape sequences (`\x1b`) were present.
**Action:** Always check `!strings.Contains(s, "\x1b")` upfront before allocating builders or slice parsers for terminal text lines.

## 2026-03-31 - Zero-Allocation Buffer Streaming in Log Distillation Filters
**Learning:** Log filters (`FilterGoTest`, `FilterDeduplicate`) were allocating `[]string` slices and converting scanner bytes to heap strings on every line, causing 1000+ allocs/op during `go test -v` distillation.
**Action:** Use `bytes.Buffer` for accumulated output and byte slices (`scanner.Bytes()`, `bytes.Equal`) for line comparisons, resetting pending buffers with `pending.Reset()` to achieve $O(1)$ reuse and zero allocs for dropped output. Fast-path line counting with `strings.Count(s, "\n")` before `strings.Split`.

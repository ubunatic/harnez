## 2026-03-31 - Fast-Path ANSI Check in ReadCard Line Processing
**Learning:** `stripANSIEscapes` and `parseANSILine` in `readcard` were allocating `strings.Builder` and token slices for every single input line even when no escape sequences (`\x1b`) were present.
**Action:** Always check `!strings.Contains(s, "\x1b")` upfront before allocating builders or slice parsers for terminal text lines.

## 2026-03-31 - Zero-Allocation Line Buffering in Stream Distillation
**Learning:** `bufio.Scanner` with `scanner.Text()` and `[]string` slice appending in line filters (`FilterGoTest`, `FilterGit`, `FilterDeduplicate`) causes massive heap allocation churn (1,000+ allocs/op for 500 lines) due to per-line string escapes.
**Action:** Use `scanner.Bytes()`, `bytes.Buffer`, and `bytes.Equal` in line-processing filters to operate directly on byte slices and reset pending buffers with `buf.Reset()`.

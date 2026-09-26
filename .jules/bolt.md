## 2026-03-31 - Fast-Path ANSI Check in ReadCard Line Processing
**Learning:** `stripANSIEscapes` and `parseANSILine` in `readcard` were allocating `strings.Builder` and token slices for every single input line even when no escape sequences (`\x1b`) were present.
**Action:** Always check `!strings.Contains(s, "\x1b")` upfront before allocating builders or slice parsers for terminal text lines.

## 2026-09-26 - Zero-Allocation Byte Streaming in Distill Filters
**Learning:** `FilterGoTest`, `FilterGit`, and `FilterDeduplicate` allocated a `[]string` slice and converted every line byte slice from `bufio.Scanner` to a heap `string`, causing >1000 allocations per `go test` distillation pass. Using `bytes.Buffer` and byte slice comparisons (`bytes.Equal`, `bytes.HasPrefix`) reduced allocations from 1022 to 15 allocs/op (~98.5% drop) and improved throughput by ~35-45%.
**Action:** Always stream line-filtering transformations using `bufio.Scanner.Bytes()` into a single `bytes.Buffer` rather than building `[]string` slices and converting per line.

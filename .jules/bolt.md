## 2026-03-31 - Fast-Path ANSI Check in ReadCard Line Processing
**Learning:** `stripANSIEscapes` and `parseANSILine` in `readcard` were allocating `strings.Builder` and token slices for every single input line even when no escape sequences (`\x1b`) were present.
**Action:** Always check `!strings.Contains(s, "\x1b")` upfront before allocating builders or slice parsers for terminal text lines.

## 2026-03-31 - Zero-Allocation Buffer Streams in Command Output Distillation
**Learning:** `FilterGoTest`, `FilterGit`, and `FilterDeduplicate` in `internal/distill` were allocating new `string` objects and `[]string` slice elements for every scanned line, causing over 1,000 heap allocations per 25KB input batch.
**Action:** Stream line processing directly using pooled `bufio.Scanner` buffers (`sync.Pool`), write raw line bytes into `bytes.Buffer`, and use zero-copy index calculations for line truncation rather than `strings.Split`.

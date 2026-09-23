## 2026-03-31 - Fast-Path ANSI Check in ReadCard Line Processing
**Learning:** `stripANSIEscapes` and `parseANSILine` in `readcard` were allocating `strings.Builder` and token slices for every single input line even when no escape sequences (`\x1b`) were present.
**Action:** Always check `!strings.Contains(s, "\x1b")` upfront before allocating builders or slice parsers for terminal text lines.

## 2026-03-31 - Zero-Allocation Pending Buffer in Go Test Distillation Filter
**Learning:** `FilterGoTest` in `internal/distill` was allocating `string` objects for every `=== RUN` line and `[]string` slices for pending test blocks, allocating 1,022 objects and 152 KB per 500 tests even when 99% of test lines were discarded passing tests.
**Action:** Use a single reusable byte slice (`pendingBuf`) reset to length zero (`[:0]`) to store transient test headers and stream matching output lines directly to `strings.Builder`.

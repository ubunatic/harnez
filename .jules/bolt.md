## 2026-03-31 - Fast-Path ANSI Check in ReadCard Line Processing
**Learning:** `stripANSIEscapes` and `parseANSILine` in `readcard` were allocating `strings.Builder` and token slices for every single input line even when no escape sequences (`\x1b`) were present.
**Action:** Always check `!strings.Contains(s, "\x1b")` upfront before allocating builders or slice parsers for terminal text lines.

## 2026-03-31 - strings.Builder.Reset() Buffer Allocation in Hot Loops
**Learning:** Calling `strings.Builder.Reset()` clears its internal byte slice (`b.buf = nil`). In hot scanning loops where a builder is reset per line/block (e.g., `FilterGoTest`), every subsequent `Write` forces a new heap allocation.
**Action:** Use a reusable byte slice (`buf = buf[:0]`) for frequently reset inner-loop line buffers to preserve slice capacity and avoid GC allocation churn.

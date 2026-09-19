# Bundled bitmap fonts

The upstream BDF files are embedded in `internal/readcard/upstream/` and parsed lazily by
`internal/readcard`. The default 5x8 font is hand-tuned and is not derived from them.

| Size | Font | Source file | Licence |
|---|---|---|---|
| 3x5 (4x6 cell) | Tom Thumb by Robey Pointer | `internal/readcard/upstream/tom-thumb.bdf` | MIT |
| 6x12 (7x12 cell) | Spleen 6x12 2.2.0 | `internal/readcard/upstream/spleen-6x12.bdf` | BSD-2-Clause |
| 7x13 | X11 misc-fixed 7x13 | `internal/readcard/upstream/7x13.bdf` | Public domain |
| 8x16 | Spleen 8x16 2.2.0 | `internal/readcard/upstream/spleen-8x16.bdf` | BSD-2-Clause |

## Spleen (BSD-2-Clause)

Copyright (c) 2018-2026, Frederic Cambus. https://www.cambus.net/

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

## Coverage gaps

Glyphs the upstream fonts lack (for example `✓ ✗ ⚠ ✦`, `→ ←` in 6x12, box drawing in 3x5) are
listed in `internal/readcard/spec/glyphs-<size>.yaml` as an editable copy of that size's `?` matrix.
`scripts/fill-missing-glyphs.go` appends them; replace a matrix to hand-draw the glyph. Braille
cells are drawn procedurally and never need an entry.

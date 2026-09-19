# Bundled bitmap fonts

Source BDF files used by `scripts/import-bdf-font.go` to fill `internal/readcard/spec/glyphs.yaml`.
Only glyphs listed in the spec charset are imported. The default 5x8 font is hand-tuned and is not derived from these files.

| Size | Font | Source file | Licence |
|---|---|---|---|
| 3x5 (4x6 cell) | Tom Thumb by Robey Pointer | `tom-thumb/tom-thumb.bdf` | MIT (BDF `COPYRIGHT` field; upstream README in `tom-thumb/`) |
| 6x12 (7x12 cell) | Spleen 6x12 2.2.0 | `spleen/spleen-6x12.bdf` | BSD-2-Clause |
| 7x13 | X11 misc-fixed 7x13 | `x11-misc-fixed/7x13.bdf` | Public domain (BDF `COPYRIGHT` field) |
| 8x16 | Spleen 8x16 2.2.0 | `spleen/spleen-8x16.bdf` | BSD-2-Clause |

## Spleen (BSD-2-Clause)

Copyright (c) 2018-2026, Frederic Cambus. https://www.cambus.net/

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

## Coverage gaps

Glyphs the upstream fonts lack render as `?`: 6x12 lacks `→ ← ✓ ✗`; 7x13 lacks `✓ ✗`.

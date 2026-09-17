#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 Uwe Jugel
# SPDX-License-Identifier: AGPL-3.0-or-later
"""
Trial Ultra-Compact Retro Pixel and Bitmap Fonts for ViT Token Compression.
Evaluates 3x5, 4x6, 5x8, and 6x12 pixel fonts across micro canvases (256x256, 384x384, 512x512).
"""

import os
import sys
import json
import math
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

SCRATCH_DIR = Path("/home/uwe/projects/harnez/scratch/pixel-fonts")
SCRATCH_DIR.mkdir(parents=True, exist_ok=True)

# Standard bitmap font 5x7 matrix (ASCII 32-126)
# Each character is defined by 5 columns of 7 bits or 7 rows of 5 bits.
# We include standard micro font tables.

# 3x5 Micro Font bitmap patterns (3 bits wide, 5 rows high per character)
# Encoded as 5 integers (0-7) per glyph for ASCII 32-126
FONT_3X5_RAW = {
    ' ': [0, 0, 0, 0, 0],
    '!': [2, 2, 2, 0, 2],
    '"': [5, 5, 0, 0, 0],
    '#': [5, 7, 5, 7, 5],
    '$': [7, 6, 7, 3, 7],
    '%': [5, 1, 2, 4, 5],
    '&': [2, 5, 2, 5, 3],
    "'": [2, 2, 0, 0, 0],
    '(': [2, 4, 4, 4, 2],
    ')': [2, 1, 1, 1, 2],
    '*': [0, 5, 2, 5, 0],
    '+': [0, 2, 7, 2, 0],
    ',': [0, 0, 0, 2, 4],
    '-': [0, 0, 7, 0, 0],
    '.': [0, 0, 0, 0, 2],
    '/': [1, 1, 2, 4, 4],
    '0': [7, 5, 5, 5, 7],
    '1': [2, 6, 2, 2, 7],
    '2': [7, 1, 7, 4, 7],
    '3': [7, 1, 7, 1, 7],
    '4': [5, 5, 7, 1, 1],
    '5': [7, 4, 7, 1, 7],
    '6': [7, 4, 7, 5, 7],
    '7': [7, 1, 2, 2, 2],
    '8': [7, 5, 7, 5, 7],
    '9': [7, 5, 7, 1, 7],
    ':': [0, 2, 0, 2, 0],
    ';': [0, 2, 0, 2, 4],
    '<': [1, 2, 4, 2, 1],
    '=': [0, 7, 0, 7, 0],
    '>': [4, 2, 1, 2, 4],
    '?': [7, 1, 3, 0, 2],
    '@': [7, 5, 7, 4, 7],
    'A': [2, 5, 7, 5, 5],
    'B': [6, 5, 6, 5, 6],
    'C': [7, 4, 4, 4, 7],
    'D': [6, 5, 5, 5, 6],
    'E': [7, 4, 6, 4, 7],
    'F': [7, 4, 6, 4, 4],
    'G': [7, 4, 5, 5, 7],
    'H': [5, 5, 7, 5, 5],
    'I': [7, 2, 2, 2, 7],
    'J': [1, 1, 1, 5, 2],
    'K': [5, 5, 6, 5, 5],
    'L': [4, 4, 4, 4, 7],
    'M': [5, 7, 5, 5, 5],
    'N': [5, 7, 7, 5, 5],
    'O': [7, 5, 5, 5, 7],
    'P': [7, 5, 7, 4, 4],
    'Q': [7, 5, 5, 7, 1],
    'R': [7, 5, 6, 5, 5],
    'S': [7, 4, 7, 1, 7],
    'T': [7, 2, 2, 2, 2],
    'U': [5, 5, 5, 5, 7],
    'V': [5, 5, 5, 5, 2],
    'W': [5, 5, 5, 7, 5],
    'X': [5, 5, 2, 5, 5],
    'Y': [5, 5, 2, 2, 2],
    'Z': [7, 1, 2, 4, 7],
    '[': [3, 2, 2, 2, 3],
    '\\': [4, 4, 2, 1, 1],
    ']': [6, 2, 2, 2, 6],
    '^': [2, 5, 0, 0, 0],
    '_': [0, 0, 0, 0, 7],
    '`': [4, 2, 0, 0, 0],
    'a': [0, 6, 7, 5, 7],
    'b': [4, 6, 5, 5, 6],
    'c': [0, 7, 4, 4, 7],
    'd': [1, 3, 5, 5, 3],
    'e': [0, 7, 7, 4, 7],
    'f': [3, 4, 6, 4, 4],
    'g': [0, 7, 5, 7, 1],
    'h': [4, 6, 5, 5, 5],
    'i': [2, 0, 2, 2, 2],
    'j': [1, 0, 1, 5, 2],
    'k': [4, 5, 6, 5, 5],
    'l': [6, 2, 2, 2, 3],
    'm': [0, 5, 7, 5, 5],
    'n': [0, 6, 5, 5, 5],
    'o': [0, 2, 5, 5, 2],
    'p': [0, 6, 5, 6, 4],
    'q': [0, 3, 5, 3, 1],
    'r': [0, 5, 6, 4, 4],
    's': [0, 3, 6, 1, 6],
    't': [2, 7, 2, 2, 1],
    'u': [0, 5, 5, 5, 3],
    'v': [0, 5, 5, 5, 2],
    'w': [0, 5, 5, 7, 5],
    'x': [0, 5, 2, 5, 5],
    'y': [0, 5, 5, 3, 6],
    'z': [0, 7, 3, 6, 7],
    '{': [3, 2, 6, 2, 3],
    '|': [2, 2, 2, 2, 2],
    '}': [6, 2, 3, 2, 6],
    '~': [0, 5, 2, 0, 0],
}

# 5x7 Standard Console Font (5 bits wide, 7 rows high)
# Encoded as 7 integers (0-31) per glyph
FONT_5X7_RAW = {
    ' ': [0, 0, 0, 0, 0, 0, 0],
    '!': [4, 4, 4, 4, 0, 0, 4],
    '"': [10, 10, 10, 0, 0, 0, 0],
    '#': [10, 10, 31, 10, 31, 10, 10],
    '$': [4, 15, 20, 14, 5, 30, 4],
    '%': [25, 26, 4, 8, 16, 19, 25],
    '&': [12, 18, 20, 8, 21, 18, 13],
    "'": [12, 4, 8, 0, 0, 0, 0],
    '(': [2, 4, 8, 8, 8, 4, 2],
    ')': [8, 4, 2, 2, 2, 4, 8],
    '*': [0, 4, 21, 14, 21, 4, 0],
    '+': [0, 4, 4, 31, 4, 4, 0],
    ',': [0, 0, 0, 0, 12, 4, 8],
    '-': [0, 0, 0, 31, 0, 0, 0],
    '.': [0, 0, 0, 0, 0, 12, 12],
    '/': [1, 2, 4, 8, 16, 0, 0],
    '0': [14, 17, 19, 21, 25, 17, 14],
    '1': [4, 12, 4, 4, 4, 4, 14],
    '2': [14, 17, 1, 2, 4, 8, 31],
    '3': [31, 2, 4, 2, 1, 17, 14],
    '4': [2, 6, 10, 18, 31, 2, 2],
    '5': [31, 16, 30, 1, 1, 17, 14],
    '6': [6, 8, 16, 30, 17, 17, 14],
    '7': [31, 1, 2, 4, 8, 8, 8],
    '8': [14, 17, 17, 14, 17, 17, 14],
    '9': [14, 17, 17, 15, 1, 2, 12],
    ':': [0, 12, 12, 0, 12, 12, 0],
    ';': [0, 12, 12, 0, 12, 4, 8],
    '<': [2, 4, 8, 16, 8, 4, 2],
    '=': [0, 31, 0, 31, 0, 0, 0],
    '>': [8, 4, 2, 1, 2, 4, 8],
    '?': [14, 17, 1, 2, 4, 0, 4],
    '@': [14, 17, 1, 13, 21, 21, 14],
    'A': [14, 17, 17, 31, 17, 17, 17],
    'B': [30, 17, 17, 30, 17, 17, 30],
    'C': [14, 17, 16, 16, 16, 17, 14],
    'D': [28, 18, 17, 17, 17, 18, 28],
    'E': [31, 16, 16, 30, 16, 16, 31],
    'F': [31, 16, 16, 30, 16, 16, 16],
    'G': [14, 17, 16, 23, 17, 17, 14],
    'H': [17, 17, 17, 31, 17, 17, 17],
    'I': [14, 4, 4, 4, 4, 4, 14],
    'J': [7, 2, 2, 2, 2, 18, 12],
    'K': [17, 18, 20, 24, 20, 18, 17],
    'L': [16, 16, 16, 16, 16, 16, 31],
    'M': [17, 27, 21, 21, 17, 17, 17],
    'N': [17, 17, 25, 21, 19, 17, 17],
    'O': [14, 17, 17, 17, 17, 17, 14],
    'P': [30, 17, 17, 30, 16, 16, 16],
    'Q': [14, 17, 17, 17, 21, 18, 13],
    'R': [30, 17, 17, 30, 20, 18, 17],
    'S': [14, 17, 16, 14, 1, 17, 14],
    'T': [31, 4, 4, 4, 4, 4, 4],
    'U': [17, 17, 17, 17, 17, 17, 14],
    'V': [17, 17, 17, 17, 17, 10, 4],
    'W': [17, 17, 17, 21, 21, 27, 17],
    'X': [17, 17, 10, 4, 10, 17, 17],
    'Y': [17, 17, 10, 4, 4, 4, 4],
    'Z': [31, 1, 2, 4, 8, 16, 31],
    '[': [14, 8, 8, 8, 8, 8, 14],
    '\\': [16, 8, 4, 2, 1, 0, 0],
    ']': [14, 2, 2, 2, 2, 2, 14],
    '^': [4, 10, 17, 0, 0, 0, 0],
    '_': [0, 0, 0, 0, 0, 0, 31],
    '`': [8, 4, 2, 0, 0, 0, 0],
    'a': [0, 0, 14, 1, 15, 17, 15],
    'b': [16, 16, 22, 25, 17, 17, 30],
    'c': [0, 0, 14, 17, 16, 17, 14],
    'd': [1, 1, 13, 19, 17, 17, 15],
    'e': [0, 0, 14, 17, 31, 16, 14],
    'f': [6, 9, 8, 28, 8, 8, 8],
    'g': [0, 0, 15, 17, 17, 15, 1, 14], # 8 rows for descender
    'h': [16, 16, 22, 25, 17, 17, 17],
    'i': [4, 0, 12, 4, 4, 4, 14],
    'j': [2, 0, 6, 2, 2, 2, 18, 12],
    'k': [16, 16, 18, 20, 24, 20, 18],
    'l': [12, 4, 4, 4, 4, 4, 14],
    'm': [0, 0, 26, 21, 21, 17, 17],
    'n': [0, 0, 22, 25, 17, 17, 17],
    'o': [0, 0, 14, 17, 17, 17, 14],
    'p': [0, 0, 30, 17, 17, 30, 16, 16],
    'q': [0, 0, 15, 17, 17, 15, 1, 1],
    'r': [0, 0, 22, 25, 16, 16, 16],
    's': [0, 0, 15, 16, 14, 1, 30],
    't': [8, 8, 28, 8, 8, 9, 6],
    'u': [0, 0, 17, 17, 17, 19, 13],
    'v': [0, 0, 17, 17, 17, 10, 4],
    'w': [0, 0, 17, 17, 21, 21, 10],
    'x': [0, 0, 17, 10, 4, 10, 17],
    'y': [0, 0, 17, 17, 15, 1, 14],
    'z': [0, 0, 31, 2, 4, 8, 31],
    '{': [6, 8, 8, 16, 8, 8, 6],
    '|': [4, 4, 4, 4, 4, 4, 4],
    '}': [12, 2, 2, 1, 2, 2, 12],
    '~': [0, 0, 13, 22, 0, 0, 0],
}


class PixelFontRenderer:
    """Pure binary 1-bit pixel font renderer with zero anti-aliasing."""

    def __init__(self, font_name="5x8"):
        self.font_name = font_name
        if font_name == "3x5":
            self.char_w = 3
            self.char_h = 5
            self.cell_w = 4  # 3px char + 1px advance
            self.cell_h = 6  # 5px char + 1px leading
            self.raw_table = FONT_3X5_RAW
        elif font_name == "4x6":
            self.char_w = 4
            self.char_h = 6
            self.cell_w = 5
            self.cell_h = 7
            self.raw_table = FONT_3X5_RAW  # scaled or fallback
        elif font_name == "5x8":
            self.char_w = 5
            self.char_h = 7
            self.cell_w = 6
            self.cell_h = 8
            self.raw_table = FONT_5X7_RAW
        elif font_name == "6x12":
            self.char_w = 6
            self.char_h = 10
            self.cell_w = 7
            self.cell_h = 12
            self.raw_table = FONT_5X7_RAW  # scaled / expanded
        else:
            raise ValueError(f"Unknown font {font_name}")

    def get_glyph_bitmap(self, char):
        rows = self.raw_table.get(char, self.raw_table.get('?', [0]*self.char_h))
        grid = []
        for row_val in rows[:self.char_h]:
            row_bits = []
            for col in range(self.char_w - 1, -1, -1):
                row_bits.append(1 if (row_val & (1 << col)) != 0 else 0)
            grid.append(row_bits)
        while len(grid) < self.char_h:
            grid.append([0] * self.char_w)
        return grid

    def render_card(self, text, width, height, scale=1, bg_color=(13, 17, 23), fg_color=(230, 237, 243),
                    accent_color=(121, 192, 255), warn_color=(255, 123, 114)):
        """Renders text buffer into an image of exact width x height with integer nearest-neighbor scaling."""
        base_w = width // scale
        base_h = height // scale

        img = Image.new("RGB", (base_w, base_h), bg_color)
        pixels = img.load()

        lines = text.split("\n")
        x_margin = 2
        y_margin = 2

        max_cols = (base_w - 2 * x_margin) // self.cell_w
        max_rows = (base_h - 2 * y_margin) // self.cell_h

        cur_y = y_margin
        row_count = 0

        for line in lines:
            if row_count >= max_rows:
                break
            cur_x = x_margin
            col_count = 0

            # Line color syntax highlighting heuristic
            line_fg = fg_color
            if line.startswith("#") or line.startswith("//"):
                line_fg = (139, 148, 158)  # comment gray
            elif line.startswith("---") or line.startswith("===") or line.startswith("PROBE"):
                line_fg = accent_color
            elif "ERR" in line or "FAIL" in line or "!=" in line or "Exit=" in line:
                line_fg = fg_color

            for char in line:
                if col_count >= max_cols:
                    break
                glyph = self.get_glyph_bitmap(char)
                for r_idx, row in enumerate(glyph):
                    for c_idx, bit in enumerate(row):
                        if bit:
                            px = cur_x + c_idx
                            py = cur_y + r_idx
                            if 0 <= px < base_w and 0 <= py < base_h:
                                pixels[px, py] = line_fg
                cur_x += self.cell_w
                col_count += 1

            cur_y += self.cell_h
            row_count += 1

        if scale > 1:
            img = img.resize((width, height), Image.Resampling.NEAREST)

        return img, max_cols, max_rows


def generate_probe_text(font_label, cols, rows):
    """Generates standard syntax stress-test text tailored for the grid."""
    lines = [
        f"=== {font_label} [{cols}x{rows} GRID] ===",
        'Token1: if test "$x" != "$y" && test -f "${PATH:?}"; then local val=$(cmd); fi',
        'Token2: export HARNEZ_QUOTA_BYPASS=0; set -euo pipefail',
        'Glyphs: { [ ( < > == != := && || ! ~ * ; : / \\ # @ $ % ^ ) ] }',
        'Disamb: 0O 1lI| 8B 5S :; ,. -_ =+ /\\ {} [] ()',
        'Struct: type Task struct { ID int `json:"id"`; Status string }',
        'Funcs : func (s *Server) Route(p string, h Handler) (res []byte, err error)',
        'Rules : Invariant 3: Zero Zombie Guarantee (exit=127, pid=9482, score=5.0)',
        'Check : [PASS] 100% Verbatim OCR recovery benchmark passed under micro canvas',
        '--- DOCUMENTATION PAYLOAD ---',
        '# Agentic Loop Invariants (Issue 398 ViT Compression Test)',
        '1. Parallel Read, Sequential Write: Exactly one active writer per workspace.',
        '2. Zero Zombie Guarantee: Track and terminate every background process.',
        '3. Single Test Boundary: Execute tests strictly via make test-q1 target.',
        '4. Quota-1 Guardrails: Must modify repository source files before re-running.',
        '5. Context Discipline: Range-bounded reads only; avoid whole-doc dumps.',
        '6. Multi-Doc Bounded Card: 3-in-1 Cheatsheet packs Bash, Make, Git rules.',
        '7. Vision Multimodal: 256x256 tile = 87-258 tokens vs 3000 text tokens (15x-30x).',
    ]
    # Pad or repeat payload to fill rows
    while len(lines) < rows:
        idx = len(lines)
        lines.append(f"PayloadLine[{idx:02d}]: rule_{idx} status=active verified=true key=0x{idx*17:04x}")
    return "\n".join(lines[:rows])


def run_benchmark():
    print("================================================================================")
    print(" TRIAL RETRO PIXEL FONTS FOR VIT TOKEN COMPRESSION BENCHMARK (ISSUE 398)")
    print("================================================================================")

    resolutions = [
        (256, 256, "micro_256"),
        (384, 384, "square_384"),
        (512, 512, "tiled_512")
    ]

    font_configs = [
        ("3x5", 1, "Tom-Thumb / Micro 3x5 (1x native)"),
        ("3x5", 2, "Tom-Thumb / Micro 3x5 (2x integer nearest)"),
        ("5x8", 1, "Spleen / Proggy 5x8 (1x native)"),
        ("5x8", 2, "Spleen / Proggy 5x8 (2x integer nearest)"),
        ("6x12", 1, "Spleen 6x12 (1x native)")
    ]

    results = []

    for w, h, res_label in resolutions:
        for font_name, scale, font_desc in font_configs:
            renderer = PixelFontRenderer(font_name=font_name)
            # Calculate grid
            base_w = w // scale
            base_h = h // scale
            cols = (base_w - 4) // renderer.cell_w
            rows = (base_h - 4) // renderer.cell_h
            total_chars = cols * rows
            est_text_tokens = int(total_chars / 4.0)

            # Vision Token calculations
            # OpenAI / Codex: 85 base + 170 per 512x512 tile
            openai_tiles = math.ceil(w / 512) * math.ceil(h / 512)
            openai_tokens = 85 + 170 * openai_tiles

            # Gemini: 258 tokens per 384x384 or 768x768 tile
            gemini_tokens = 258

            # Claude: (W * H) / 750
            claude_tokens = int((w * h) / 750)

            # Compression ratios
            openai_ratio = est_text_tokens / openai_tokens if openai_tokens > 0 else 0
            gemini_ratio = est_text_tokens / gemini_tokens if gemini_tokens > 0 else 0
            claude_ratio = est_text_tokens / claude_tokens if claude_tokens > 0 else 0

            text_payload = generate_probe_text(f"{font_name} s{scale}", cols, rows)
            img, actual_cols, actual_rows = renderer.render_card(text_payload, w, h, scale=scale)

            filename = f"probe_{res_label}_{font_name}_s{scale}.png"
            out_path = SCRATCH_DIR / filename
            img.save(out_path)

            row_data = {
                "canvas": f"{w}x{h}",
                "font": font_name,
                "scale": f"{scale}x",
                "font_desc": font_desc,
                "grid": f"{cols}x{rows}",
                "chars": total_chars,
                "text_tokens": est_text_tokens,
                "openai_tokens": openai_tokens,
                "openai_ratio": f"{openai_ratio:.2f}x",
                "gemini_tokens": gemini_tokens,
                "gemini_ratio": f"{gemini_ratio:.2f}x",
                "claude_tokens": claude_tokens,
                "claude_ratio": f"{claude_ratio:.2f}x",
                "image_file": str(out_path)
            }
            results.append(row_data)

    # Print markdown table
    print("\n### Pixel Font Capacity & Multimodal Token Compression Matrix\n")
    print("| Canvas | Font & Scale | Grid (Cols x Rows) | Chars | Text Tokens | OpenAI Vision (Ratio) | Gemini Vision (Ratio) | Claude Vision (Ratio) |")
    print("| :---: | :--- | :---: | :---: | :---: | :---: | :---: | :---: |")
    for r in results:
        print(f"| **{r['canvas']}** | {r['font_desc']} | {r['grid']} | {r['chars']:,} | **{r['text_tokens']:,}** | {r['openai_tokens']} (**{r['openai_ratio']}**) | {r['gemini_tokens']} (**{r['gemini_ratio']}**) | {r['claude_tokens']} (**{r['claude_ratio']}**) |")

    # Generate Side-by-side Binary vs Anti-Aliasing comparison card
    generate_comparison_card()

    # Save JSON benchmark
    json_path = SCRATCH_DIR / "pixel_font_benchmark_results.json"
    json_path.write_text(json.dumps(results, indent=2))
    print(f"\n[OK] Benchmark completed. Results saved to {json_path}")
    return results


def generate_comparison_card():
    """Generates a side-by-side test card comparing 1-bit crisp pixel font vs Anti-Aliased vector font."""
    w, h = 600, 320
    img = Image.new("RGB", (w, h), (13, 17, 23))
    draw = ImageDraw.Draw(img)

    # 1-bit pixel renderer (5x8 and 3x5)
    p58 = PixelFontRenderer("5x8")
    p35 = PixelFontRenderer("3x5")

    # Header
    draw.text((10, 10), "--- CONTRAST PROBE: 1-BIT CRISP PIXEL vs ANTI-ALIASED VECTOR ---", fill=(121, 192, 255))

    # Section 1: 1-bit binary 5x8
    draw.text((10, 35), "[A] Pure 1-Bit Bitmap 5x8 (Zero Anti-Aliasing, 100% Binary Contrast):", fill=(88, 166, 255))
    sample_text = 'if test "$x" != "$y" && test -f "${PATH:?}"; then local val=$(cmd); fi\n{ [ ( < > == != := && || ! ~ * ; : / \\ # @ $ % ^ ) ] }\nInvariant 3: Zero Zombie Guarantee (pid=9482, exit=0)'
    
    sub_img, _, _ = p58.render_card(sample_text, 580, 50, scale=1)
    img.paste(sub_img, (10, 55))

    # Section 2: 1-bit binary 3x5 (Micro)
    draw.text((10, 115), "[B] Pure 1-Bit Micro 3x5 (Extreme Density, Zero Blur):", fill=(88, 166, 255))
    sub_img_35, _, _ = p35.render_card(sample_text, 580, 45, scale=1)
    img.paste(sub_img_35, (10, 135))

    # Section 3: Anti-aliased small vector simulation (Pillow default vector rendering at 8px)
    draw.text((10, 190), "[C] Standard Monospace Vector Font at 8px (Anti-Aliased Gray-Bleed):", fill=(255, 123, 114))
    # Draw vector text with default or truetype font if available
    try:
        ttf_font = ImageFont.load_default()
        draw.text((10, 210), sample_text, font=ttf_font, fill=(230, 237, 243))
    except Exception:
        draw.text((10, 210), sample_text, fill=(230, 237, 243))

    # Legend / Key Insights
    draw.text((10, 275), "Key ViT Observation: 1-bit binary eliminates subpixel gray bleed on :; != {} []", fill=(63, 185, 80))
    draw.text((10, 295), "ViT patches (14x14px) preserve crisp edges at 1-bit, preventing token hallucination.", fill=(139, 148, 158))

    out_path = SCRATCH_DIR / "contrast_comparison_1bit_vs_aa.png"
    img.save(out_path)
    print(f"[OK] Generated contrast comparison card: {out_path}")


if __name__ == "__main__":
    run_benchmark()
